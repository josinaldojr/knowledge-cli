package retrieval

import "testing"

func TestFuseRankedRewardsAgreement(t *testing.T) {
	lexical := []Scored{{ID: "a", Score: 9}, {ID: "b", Score: 5}, {ID: "c", Score: 1}}
	semantic := []Scored{{ID: "b", Score: 0.9}, {ID: "a", Score: 0.4}}

	fused := FuseRanked(lexical, semantic)
	if len(fused) != 3 {
		t.Fatalf("expected 3 fused results, got %#v", fused)
	}
	// "a" and "b" each rank in both lists; "c" only ranks in one, so it must
	// not outrank either even though this test does not assert an exact score.
	ids := map[string]int{}
	for i, r := range fused {
		ids[r.ID] = i
	}
	if ids["c"] <= ids["a"] || ids["c"] <= ids["b"] {
		t.Errorf("expected 'c' (single-list match) to rank behind 'a' and 'b' (double-list matches), got %#v", fused)
	}
}

func TestFuseRankedSingleList(t *testing.T) {
	fused := FuseRanked([]Scored{{ID: "x", Score: 1}, {ID: "y", Score: 0.5}})
	if len(fused) != 2 || fused[0].ID != "x" || fused[1].ID != "y" {
		t.Errorf("expected order preserved for a single input list, got %#v", fused)
	}
}

func TestFuseRankedEmpty(t *testing.T) {
	if got := FuseRanked(); len(got) != 0 {
		t.Errorf("expected no results for no input lists, got %#v", got)
	}
	if got := FuseRanked(nil, nil); len(got) != 0 {
		t.Errorf("expected no results for empty input lists, got %#v", got)
	}
}
