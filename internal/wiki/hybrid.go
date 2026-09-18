package wiki

import (
	"context"
	"fmt"
	"math"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"kv/internal/embed"
	"kv/internal/retrieval"
)

// HybridSearchIndex ranks index entries against query using both lexical
// (BM25) and, when embedder is available, semantic (cosine similarity over
// local sentence embeddings) signals, combined with Reciprocal Rank Fusion.
// embedder may be nil, or its Embed call may fail (the local model isn't
// cached and couldn't be downloaded, onnxruntime isn't installed, ...);
// either way HybridSearchIndex quietly falls back to BM25-only ranking
// rather than failing the search, the same "degrade, don't block" posture
// kv's provider adapters already use for MCP delivery.
//
// This is the retrieval kv's RAG surfaces (AskWiki, the wiki serve chat
// endpoint) should use instead of the plain lexical SearchIndex: better
// ranking means the top few results can be trusted, instead of padding the
// prompt with borderline lexical matches to compensate for weak recall —
// the concrete token cost hybrid search is meant to cut.
func HybridSearchIndex(index []IndexEntry, query string, topN int, embedder embed.Embedder) []SearchResult {
	if len(index) == 0 || strings.TrimSpace(query) == "" {
		return nil
	}

	byPath := make(map[string]IndexEntry, len(index))
	docs := make(map[string]string, len(index))
	for _, entry := range index {
		byPath[entry.Path] = entry
		docs[entry.Path] = entry.Title + "\n" + strings.Join(entry.Tags, " ") + "\n" + entry.Content
	}

	rankings := [][]retrieval.Scored{retrieval.NewBM25Index(docs).Search(query)}
	if embedder != nil {
		if semantic, ok := semanticRanking(embedder, docs, query); ok {
			rankings = append(rankings, semantic)
		}
	}

	fused := retrieval.FuseRanked(rankings...)
	if topN > 0 && len(fused) > topN {
		fused = fused[:topN]
	}

	results := make([]SearchResult, 0, len(fused))
	for _, item := range fused {
		entry, ok := byPath[item.ID]
		if !ok {
			continue
		}
		results = append(results, SearchResult{Entry: entry, Score: int(math.Round(item.Score * 1000))})
	}
	return results
}

// semanticRanking embeds every document plus the query in a single batch
// call and ranks documents by cosine similarity to the query. It reports ok
// = false whenever the embedder can't produce a usable result, so callers
// can fall back to lexical-only ranking instead of failing outright.
func semanticRanking(embedder embed.Embedder, docs map[string]string, query string) (ranking []retrieval.Scored, ok bool) {
	ids := make([]string, 0, len(docs))
	texts := make([]string, 0, len(docs)+1)
	for id, text := range docs {
		ids = append(ids, id)
		texts = append(texts, text)
	}
	texts = append(texts, query)

	vectors, err := embedder.Embed(texts)
	if err != nil || len(vectors) != len(texts) {
		return nil, false
	}
	queryVector := vectors[len(vectors)-1]
	docVectors := vectors[:len(vectors)-1]

	scored := make([]retrieval.Scored, 0, len(ids))
	for i, id := range ids {
		similarity := embed.CosineSimilarity(docVectors[i], queryVector)
		if similarity > 0 {
			scored = append(scored, retrieval.Scored{ID: id, Score: similarity})
		}
	}
	sort.SliceStable(scored, func(i, j int) bool {
		if scored[i].Score != scored[j].Score {
			return scored[i].Score > scored[j].Score
		}
		return scored[i].ID < scored[j].ID
	})
	return scored, true
}

var (
	embedderOnce    sync.Once
	defaultEmbedder embed.Embedder // nil when semantic ranking is unavailable
)

// DefaultEmbedder lazily builds (downloading and caching its model on first
// use, if needed) the local embedder shared by kv's Wiki hybrid search,
// memoizing success or unavailability for the life of the process. A nil
// result means semantic ranking is unavailable and callers should pass it
// straight into HybridSearchIndex, which degrades to BM25-only ranking.
func DefaultEmbedder() embed.Embedder {
	embedderOnce.Do(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		e, err := embed.NewONNXEmbedder(ctx, embed.DefaultModelConfig())
		if err != nil {
			fmt.Fprintf(os.Stderr, "kv: semantic search unavailable, falling back to lexical-only ranking (%v)\n", err)
			return
		}
		defaultEmbedder = e
	})
	return defaultEmbedder
}
