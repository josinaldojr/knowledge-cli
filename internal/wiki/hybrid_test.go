package wiki

import (
	"errors"
	"strings"
	"testing"
)

// fakeEmbedder returns pre-programmed vectors keyed by exact text match, so
// tests can control cosine similarity without a real model.
type fakeEmbedder struct {
	vectors map[string][]float32
	err     error
}

func (f *fakeEmbedder) Embed(texts []string) ([][]float32, error) {
	if f.err != nil {
		return nil, f.err
	}
	out := make([][]float32, len(texts))
	for i, text := range texts {
		out[i] = f.vectors[text]
	}
	return out, nil
}

func buildTestIndex() []IndexEntry {
	return []IndexEntry{
		{Path: "lexical.md", Title: "Lexical Match", Content: "stripe checkout stripe checkout stripe checkout"},
		{Path: "semantic.md", Title: "Payments Overview", Content: "how customers complete a purchase and pay for an order"},
	}
}

func containsPath(results []SearchResult, path string) bool {
	for _, r := range results {
		if r.Entry.Path == path {
			return true
		}
	}
	return false
}

func TestHybridSearchIndexFallsBackToLexicalWithoutEmbedder(t *testing.T) {
	results := HybridSearchIndex(buildTestIndex(), "stripe checkout", 5, nil)
	if !containsPath(results, "lexical.md") {
		t.Fatalf("expected the lexical match to be found, got %#v", results)
	}
	if containsPath(results, "semantic.md") {
		t.Errorf("expected the lexically-unrelated doc to be absent without an embedder, got %#v", results)
	}
}

func TestHybridSearchIndexSurfacesSemanticMatch(t *testing.T) {
	embedder := &fakeEmbedder{vectors: map[string][]float32{
		docText(buildTestIndex()[0]): {0, 1}, // orthogonal to the query: no semantic signal
		docText(buildTestIndex()[1]): {1, 0}, // identical to the query: strong semantic signal
		"stripe checkout":            {1, 0},
	}}

	results := HybridSearchIndex(buildTestIndex(), "stripe checkout", 5, embedder)
	if !containsPath(results, "lexical.md") {
		t.Errorf("expected the lexical match to still be present, got %#v", results)
	}
	if !containsPath(results, "semantic.md") {
		t.Errorf("expected the embedder to surface the semantically related doc that lexical search alone misses, got %#v", results)
	}
}

func TestHybridSearchIndexDegradesWhenEmbedderErrors(t *testing.T) {
	embedder := &fakeEmbedder{err: errTest}
	results := HybridSearchIndex(buildTestIndex(), "stripe checkout", 5, embedder)
	if !containsPath(results, "lexical.md") {
		t.Fatalf("expected lexical-only fallback to still work when the embedder errors, got %#v", results)
	}
}

func TestHybridSearchIndexEmptyInputs(t *testing.T) {
	if got := HybridSearchIndex(nil, "query", 5, nil); got != nil {
		t.Errorf("expected nil for an empty index, got %#v", got)
	}
	if got := HybridSearchIndex(buildTestIndex(), "  ", 5, nil); got != nil {
		t.Errorf("expected nil for a blank query, got %#v", got)
	}
}

// docText mirrors the document text HybridSearchIndex builds internally, so
// tests can key fakeEmbedder's canned vectors by the exact text it will see.
func docText(entry IndexEntry) string {
	return entry.Title + "\n" + strings.Join(entry.Tags, " ") + "\n" + entry.Content
}

var errTest = errors.New("embedder unavailable")
