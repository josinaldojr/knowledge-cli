package memory

import (
	"context"
	"testing"
	"time"

	"kv/internal/store"
)

type changeRepository struct{ candidates []store.MemoryCandidate }

func (f changeRepository) ListMemoryCandidatesForChange(_ context.Context, _, _ string) ([]store.MemoryCandidate, error) {
	return f.candidates, nil
}

func TestConsolidatorSeparatesCurrentAndSupersededHistory(t *testing.T) {
	now := time.Now()
	closed := now.Add(-time.Minute)
	view, err := NewConsolidator(changeRepository{candidates: []store.MemoryCandidate{{ID: "scope", Kind: "scope", Status: "current", Summary: "final scope", CreatedAt: now}, {ID: "decision", Kind: "decision", Status: "current", Summary: "current decision", CreatedAt: now}, {ID: "requirement", Kind: "requirement", Status: "current", Summary: "final requirement", CreatedAt: now}, {ID: "deviation", Kind: "deviation", Status: "accepted", Summary: "accepted deviation", CreatedAt: now}, {ID: "old", Kind: "decision", Status: "current", Summary: "old decision", ValidTo: &closed, CreatedAt: now}}}).Consolidate(context.Background(), "workspace", "change")
	if err != nil {
		t.Fatal(err)
	}
	if len(view.FinalScope) != 1 || len(view.CurrentDecisions) != 1 || len(view.FinalRequirements) != 1 || len(view.AcceptedDeviations) != 1 || len(view.SupersededHistory) != 1 {
		t.Fatalf("view = %#v", view)
	}
}

func TestConsolidatorIsDeterministicAndRetainsDuplicateSourceHistory(t *testing.T) {
	now := time.Now()
	candidates := []store.MemoryCandidate{{ID: "b", Kind: "decision", Status: "current", Summary: "second", CreatedAt: now}, {ID: "a", Kind: "decision", Status: "current", Summary: "first", CreatedAt: now}, {ID: "a", Kind: "decision", Status: "current", Summary: "first", CreatedAt: now, RevisionID: "second-source"}}
	view, err := NewConsolidator(changeRepository{candidates: candidates}).Consolidate(context.Background(), "workspace", "change")
	if err != nil {
		t.Fatal(err)
	}
	if len(view.CurrentDecisions) != 2 || view.CurrentDecisions[0].ID != "a" || view.CurrentDecisions[1].ID != "b" {
		t.Fatalf("decisions = %#v", view.CurrentDecisions)
	}
}
