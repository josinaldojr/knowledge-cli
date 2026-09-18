package retrieval

import "testing"

func TestBM25RanksMoreRelevantDocHigher(t *testing.T) {
	idx := NewBM25Index(map[string]string{
		"payments": "Payments System handles checkout and subscription services using Stripe API.",
		"frontend": "Frontend setup guide for React applications and component libraries.",
		"auth":     "Auth flow describes JWT tokens and session checkout for payments callbacks.",
	})

	results := idx.Search("stripe checkout")
	if len(results) < 2 {
		t.Fatalf("expected at least 2 matches, got %d: %#v", len(results), results)
	}
	if results[0].ID != "payments" {
		t.Errorf("expected 'payments' to rank first, got %q (results=%#v)", results[0].ID, results)
	}
	for _, r := range results {
		if r.ID == "frontend" {
			t.Errorf("did not expect 'frontend' to match 'stripe checkout', got %#v", r)
		}
	}
}

func TestBM25NoMatchesReturnsNil(t *testing.T) {
	idx := NewBM25Index(map[string]string{"a": "some unrelated content about widgets"})
	if got := idx.Search("nonexistent-term-xyz"); len(got) != 0 {
		t.Errorf("expected no matches, got %#v", got)
	}
}

func TestBM25EmptyIndex(t *testing.T) {
	idx := NewBM25Index(map[string]string{})
	if got := idx.Search("anything"); len(got) != 0 {
		t.Errorf("expected no matches against an empty index, got %#v", got)
	}
}

func TestBM25DeterministicTieBreak(t *testing.T) {
	idx := NewBM25Index(map[string]string{
		"b-doc": "widget widget widget widget",
		"a-doc": "widget widget widget widget",
	})
	results := idx.Search("widget")
	if len(results) != 2 {
		t.Fatalf("expected 2 tied matches, got %#v", results)
	}
	if results[0].ID != "a-doc" || results[1].ID != "b-doc" {
		t.Errorf("expected deterministic tie-break by ID ('a-doc' before 'b-doc'), got %#v", results)
	}
}
