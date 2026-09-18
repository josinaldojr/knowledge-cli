// Package embed computes local sentence embeddings for kv's hybrid
// retrieval, using a small BERT-family model (e.g.
// sentence-transformers/all-MiniLM-L6-v2) exported to ONNX and run through
// onnxruntime, rather than an external embeddings API. This keeps vault and
// wiki content on the machine, matching the local-first, no-egress design
// already documented for kv's other storage (see
// docs/automatic-mcp-memory.md and the "Storage dependency decision" in
// docs/provider-capability-matrix.md). The trade-off is a heavier,
// platform-specific runtime dependency; see model.go for how that is
// isolated and how the package degrades when it is unavailable.
package embed

import (
	"bufio"
	"os"
	"strings"
	"unicode"
)

// Special WordPiece tokens used by BERT-family vocabularies.
const (
	tokenCLS = "[CLS]"
	tokenSEP = "[SEP]"
	tokenUNK = "[UNK]"
	tokenPAD = "[PAD]"
)

// Tokenizer implements BERT-style WordPiece tokenization: lowercase, split
// on whitespace and punctuation, then greedily match the longest known
// subword at each position (continuation pieces are prefixed with "##").
// This mirrors the tokenization every BERT-family sentence-transformers
// model (including all-MiniLM-L6-v2) was trained with; using anything else
// would silently produce wrong embeddings.
type Tokenizer struct {
	vocab     map[string]int64
	maxTokens int
}

// LoadVocab reads a WordPiece vocabulary file (one token per line; the line
// number is the token ID), the format used by every HF BERT-family
// tokenizer.json/vocab.txt export. maxTokens caps the sequence length
// (including [CLS]/[SEP]); pass 0 for the common BERT default of 512.
func LoadVocab(path string, maxTokens int) (*Tokenizer, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	if maxTokens <= 0 {
		maxTokens = 512
	}
	vocab := map[string]int64{}
	scanner := bufio.NewScanner(f)
	var id int64
	for scanner.Scan() {
		token := strings.TrimRight(scanner.Text(), "\r\n")
		if token != "" {
			vocab[token] = id
		}
		id++
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return &Tokenizer{vocab: vocab, maxTokens: maxTokens}, nil
}

// Encode tokenizes text into the input_ids/attention_mask/token_type_ids
// triple a BERT-family ONNX export expects, wrapped in [CLS]/[SEP] and
// truncated to the tokenizer's maxTokens.
func (t *Tokenizer) Encode(text string) (inputIDs, attentionMask, tokenTypeIDs []int64) {
	pieces := []string{tokenCLS}
	for _, word := range basicTokenize(text) {
		pieces = append(pieces, t.wordpiece(word)...)
	}
	pieces = append(pieces, tokenSEP)
	if len(pieces) > t.maxTokens {
		pieces = append(pieces[:t.maxTokens-1], tokenSEP)
	}

	inputIDs = make([]int64, len(pieces))
	attentionMask = make([]int64, len(pieces))
	tokenTypeIDs = make([]int64, len(pieces))
	unk := t.vocab[tokenUNK]
	for i, piece := range pieces {
		id, ok := t.vocab[piece]
		if !ok {
			id = unk
		}
		inputIDs[i] = id
		attentionMask[i] = 1
	}
	return inputIDs, attentionMask, tokenTypeIDs
}

// wordpiece greedily splits a single whitespace/punctuation-delimited word
// into known subwords, longest match first, prefixing every piece after the
// first with "##". A word with no valid split becomes a single [UNK].
func (t *Tokenizer) wordpiece(word string) []string {
	runes := []rune(word)
	var pieces []string
	start := 0
	for start < len(runes) {
		end := len(runes)
		var matched string
		for end > start {
			candidate := string(runes[start:end])
			if start > 0 {
				candidate = "##" + candidate
			}
			if _, ok := t.vocab[candidate]; ok {
				matched = candidate
				break
			}
			end--
		}
		if matched == "" {
			return []string{tokenUNK}
		}
		pieces = append(pieces, matched)
		start = end
	}
	return pieces
}

// basicTokenize lowercases text and splits it into words, treating
// punctuation as its own token, matching BERT's BasicTokenizer.
func basicTokenize(text string) []string {
	var words []string
	var current strings.Builder
	flush := func() {
		if current.Len() > 0 {
			words = append(words, current.String())
			current.Reset()
		}
	}
	for _, r := range strings.ToLower(text) {
		switch {
		case unicode.IsSpace(r):
			flush()
		case unicode.IsPunct(r) || unicode.IsSymbol(r):
			flush()
			words = append(words, string(r))
		default:
			current.WriteRune(r)
		}
	}
	flush()
	return words
}
