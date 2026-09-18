# Hybrid retrieval for the Wiki

`kv wiki ask` and the `kv wiki serve` chat endpoint are kv's RAG surfaces:
they search the vault, then paste the full content of the top matches into
the prompt sent to the configured LLM (see `docs/*` under
`internal/wiki/web/docs` for the Wiki itself, and `internal/wiki/client.go`
for the LLM call). Retrieval quality directly controls how many tokens that
prompt costs: weak retrieval means padding the top-N with borderline lexical
matches to compensate for poor recall, or lowering N and risking a missed
relevant page.

## What changed

Before this change, `SearchIndex` (`internal/wiki/indexer.go`) was pure
lexical scoring: lowercase substring/token counts, no notion of semantic
similarity. `HybridSearchIndex` (`internal/wiki/hybrid.go`) replaces it on
the RAG path (`AskWiki` and the `wiki serve` chat handler; `SearchIndex`
itself is unchanged and still used by `kv wiki link`'s compiler and the
plain search endpoint) and combines two independently-ranked lists with
[Reciprocal Rank Fusion](internal/retrieval/hybrid.go):

- **Lexical**: BM25 (`internal/retrieval/bm25.go`), replacing the ad hoc
  substring counting `SearchIndex` still uses, with the standard,
  corpus-aware ranking function.
- **Semantic**: cosine similarity over local sentence embeddings
  (`internal/embed`), computed by running a small BERT-family
  sentence-transformers model (default:
  [`sentence-transformers/all-MiniLM-L6-v2`](https://huggingface.co/sentence-transformers/all-MiniLM-L6-v2))
  through [onnxruntime](https://onnxruntime.ai/) via
  [`onnxruntime_go`](https://github.com/yalue/onnxruntime_go).

## Why local ONNX instead of the Hugging Face Inference API

kv's other storage is explicitly local-first: engineering memory, vault
content, and snapshots never leave the machine (see
`docs/automatic-mcp-memory.md`). Computing embeddings locally keeps that
property; calling a hosted embeddings API instead would mean every vault
document gets sent to a third party on every reindex. The trade-off is a
heavier, platform-specific runtime dependency (CGo, a native onnxruntime
shared library, a ~90MB model download on first use) instead of a simple
HTTPS call — a deliberate choice given kv's design, not a default anyone
should reuse without weighing the same trade-off in their own project.

## Graceful degradation

Semantic ranking is best-effort. `embed.NewONNXEmbedder` returns a wrapped
`embed.ErrUnavailable` — never a partially-initialized embedder — whenever:

- the onnxruntime shared library isn't installed or discoverable,
- the model/vocabulary aren't cached locally and can't be downloaded, or
- the runtime fails to load the model for any other reason.

`wiki.DefaultEmbedder()` builds the shared embedder once per process,
prints a one-line notice to stderr on failure, and memoizes the failure so
kv never blocks or retries a slow network failure on every search.
`HybridSearchIndex` accepts a possibly-nil embedder and falls back to
BM25-only ranking whenever it is nil or its `Embed` call fails — the same
"degrade, don't block" posture kv's MCP provider adapters already use.

## Setup

1. Install an onnxruntime shared library for your platform (from the
   [official releases](https://github.com/microsoft/onnxruntime/releases))
   and either place it on your library search path as `onnxruntime.so` /
   `.dylib` / `.dll`, or set `embed.ModelConfig.SharedLibraryPath`.
2. On first use, `kv` downloads `onnx/model.onnx` and `vocab.txt` from the
   configured Hugging Face repo into
   `<kv cache dir>/models/all-MiniLM-L6-v2/` (see the storage table in
   `docs/automatic-mcp-memory.md` for where `<kv cache dir>` resolves to,
   including the `KV_CACHE_HOME` override) and reuses that copy afterward.
3. If either step fails, hybrid search silently degrades to BM25-only
   ranking; nothing else changes.

## Validation status

`internal/embed`'s WordPiece tokenizer, mean-pooling/L2-normalization, BM25
index, and Reciprocal Rank Fusion are covered by unit tests that don't need
network access or a real model (`internal/embed/*_test.go`,
`internal/retrieval/*_test.go`). The `onnxruntime_go` CGo binding itself
compiles and links in this environment.

What is **not** yet verified end to end: actually downloading
`all-MiniLM-L6-v2`'s ONNX export and running real inference against it,
because this repository's egress policy blocks `huggingface.co` (confirmed
via this session's proxy status: `connect_rejected`, "policy denial").
`TestNewONNXEmbedderDegradesGracefullyWhenModelUnreachable` exercises the
real failure path against that exact block and confirms the fallback
works, but the success path (real download, real tokenization against the
model's actual vocabulary, real ONNX inference, sane cosine-similarity
rankings) needs to be run once in an environment with Hugging Face access
before relying on semantic ranking in production. Also unverified: whether
`model.onnx`'s actual input/output tensor names match the
`input_ids`/`attention_mask`/`token_type_ids` → `last_hidden_state`
defaults in `embed.DefaultModelConfig()` — exporters occasionally name
these differently, in which case `ModelConfig.InputNames`/`OutputName`
need adjusting to match.

## Known simplifications / follow-ups

- Each document is embedded as a single vector (title + tags + full
  content, truncated to `ModelConfig.MaxTokens`, default 256 tokens), not
  chunked. A long page's tail is invisible to semantic ranking even though
  BM25 still sees all of it. Chunking is a natural follow-up once the
  end-to-end path above is validated.
- Embeddings are recomputed on every search; nothing is cached to disk yet.
  For small vaults this is fine, but a persistent embedding cache keyed by
  content hash (mirroring the snapshot store's SHA-256 deduplication) would
  remove the main scaling cost.
- `internal/vault` search (`kv find`) and MCP memory retrieval
  (`kv_memory_search`) still use their existing lexical scoring; hybrid
  retrieval is wired into the Wiki RAG path specifically, since that is
  where full document content is pasted into an LLM prompt today.
