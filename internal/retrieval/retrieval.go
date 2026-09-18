// Package retrieval holds the lexical matching primitives shared by kv's
// local, offline search surfaces: vault markdown search (internal/vault),
// the Wiki index (internal/wiki), and MCP engineering memory retrieval
// (internal/memory). Before this package existed, each of the three
// reimplemented the same lowercase substring/token-count scoring
// independently. This package extracts only the shared primitive; each
// caller keeps composing it with its own domain-specific scoring (memory's
// current/superseded bonuses, wiki's per-field weights, and so on), so
// behavior is unchanged, not merged into one policy.
package retrieval

import (
	"strings"
	"unicode"
)

// Words splits text on whitespace into lowercase tokens, leaving punctuation
// attached to a word (e.g. "backend-concurrency" stays intact). This is the
// tokenizer used by vault and Wiki free-text search.
func Words(text string) []string {
	return strings.Fields(strings.ToLower(text))
}

// AlnumTokens splits text into a set of lowercase letter/number runs,
// dropping punctuation entirely. This is the tokenizer used by MCP memory
// retrieval, which matches identifiers and change keys more strictly than
// free-text search does.
func AlnumTokens(text string) map[string]struct{} {
	tokens := map[string]struct{}{}
	for _, token := range strings.FieldsFunc(strings.ToLower(text), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsNumber(r)
	}) {
		if token != "" {
			tokens[token] = struct{}{}
		}
	}
	return tokens
}

// Overlap counts how many tokens in a are also present in b.
func Overlap(a, b map[string]struct{}) int {
	count := 0
	for token := range a {
		if _, ok := b[token]; ok {
			count++
		}
	}
	return count
}

// FieldWeights tunes how ScoreField rewards a query matching one piece of
// text.
type FieldWeights struct {
	// PhraseMatch is added once if the whole query appears verbatim.
	PhraseMatch int
	// TokenHit is added once per distinct query token found, regardless of
	// how many times it occurs.
	TokenHit int
	// TokenCount is added once per occurrence of each query token found.
	TokenCount int
	// MinTokenLength discards query tokens shorter than this before
	// scoring TokenHit/TokenCount (0 means no minimum).
	MinTokenLength int
}

// ScoreField scores freeform text against a query using Words tokenization
// and the given weights. It is the shared primitive behind vault.Search and
// wiki.SearchIndex.
func ScoreField(content, query string, weights FieldWeights) int {
	if strings.TrimSpace(query) == "" {
		return 0
	}
	contentLower := strings.ToLower(content)
	queryLower := strings.ToLower(query)

	score := 0
	if weights.PhraseMatch != 0 && strings.Contains(contentLower, queryLower) {
		score += weights.PhraseMatch
	}
	if weights.TokenHit == 0 && weights.TokenCount == 0 {
		return score
	}
	for _, token := range Words(query) {
		if len(token) < weights.MinTokenLength {
			continue
		}
		count := strings.Count(contentLower, token)
		if count == 0 {
			continue
		}
		if weights.TokenHit != 0 {
			score += weights.TokenHit
		}
		if weights.TokenCount != 0 {
			score += count * weights.TokenCount
		}
	}
	return score
}
