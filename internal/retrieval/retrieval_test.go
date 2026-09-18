package retrieval

import "testing"

func TestWords(t *testing.T) {
	got := Words("Backend Development, Go!")
	want := []string{"backend", "development,", "go!"}
	if len(got) != len(want) {
		t.Fatalf("Words() = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("Words()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestAlnumTokensAndOverlap(t *testing.T) {
	a := AlnumTokens("Use SQLite WAL for concurrent hooks")
	b := AlnumTokens("SQLite concurrent hooks")
	if got := Overlap(b, a); got != 3 {
		t.Errorf("Overlap() = %d, want 3", got)
	}
	if _, ok := a["for"]; !ok {
		t.Errorf("expected 'for' to be tokenized")
	}
	if _, ok := a["sqlite"]; !ok {
		t.Errorf("expected case-insensitive 'sqlite' token")
	}
}

func TestScoreFieldTokenCount(t *testing.T) {
	content := "This is a backend development project. The backend uses Go."
	if got := ScoreField(content, "development", FieldWeights{TokenCount: 10}); got != 10 {
		t.Errorf("expected 10, got %d", got)
	}
	if got := ScoreField(content, "backend", FieldWeights{TokenCount: 10}); got != 20 {
		t.Errorf("expected 20, got %d", got)
	}
	if got := ScoreField(content, "BACKEND DEVELOPMENT", FieldWeights{TokenCount: 10}); got != 30 {
		t.Errorf("expected 30, got %d", got)
	}
}

func TestScoreFieldPhraseAndMinTokenLength(t *testing.T) {
	content := "Payments System handles checkout and subscription services."
	got := ScoreField(content, "checkout", FieldWeights{PhraseMatch: 50, TokenCount: 5, MinTokenLength: 2})
	if got != 55 { // phrase match (50) + single occurrence (5)
		t.Errorf("expected 55, got %d", got)
	}

	// A single-letter token must be ignored when MinTokenLength excludes it.
	got = ScoreField("a system", "a", FieldWeights{TokenHit: 40, MinTokenLength: 2})
	if got != 0 {
		t.Errorf("expected 0 for filtered short token, got %d", got)
	}
	got = ScoreField("a system", "a", FieldWeights{TokenHit: 40})
	if got != 40 {
		t.Errorf("expected 40 without a minimum length, got %d", got)
	}
}

func TestScoreFieldEmptyQuery(t *testing.T) {
	if got := ScoreField("anything", "   ", FieldWeights{TokenCount: 10, PhraseMatch: 5}); got != 0 {
		t.Errorf("expected 0 for blank query, got %d", got)
	}
}
