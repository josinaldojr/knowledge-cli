package memory

import (
	"context"
	"sort"

	"kv/internal/store"
)

type ChangeCandidateRepository interface {
	ListMemoryCandidatesForChange(context.Context, string, string) ([]store.MemoryCandidate, error)
}

// ConsolidatedView is a deterministic current view. It does not delete or
// mutate the transformations and temporal memories from which it was derived.
type ConsolidatedView struct {
	ChangeKey          string
	FinalScope         []Result
	CurrentDecisions   []Result
	FinalRequirements  []Result
	AcceptedDeviations []Result
	SupersededHistory  []Result
}

type Consolidator struct{ repository ChangeCandidateRepository }

func NewConsolidator(repository ChangeCandidateRepository) *Consolidator {
	return &Consolidator{repository: repository}
}

func (c *Consolidator) Consolidate(ctx context.Context, workspaceID, changeKey string) (ConsolidatedView, error) {
	candidates, err := c.repository.ListMemoryCandidatesForChange(ctx, workspaceID, changeKey)
	if err != nil {
		return ConsolidatedView{}, err
	}
	view := ConsolidatedView{ChangeKey: changeKey}
	seen := map[string]struct{}{}
	for _, candidate := range candidates {
		if _, exists := seen[candidate.ID]; exists {
			continue
		}
		seen[candidate.ID] = struct{}{}
		result := resultFromCandidate(candidate)
		if candidate.ValidTo != nil || candidate.SupersedesID != "" || candidate.Status == "superseded" {
			view.SupersededHistory = append(view.SupersededHistory, result)
			continue
		}
		switch candidate.Kind {
		case "scope":
			view.FinalScope = append(view.FinalScope, result)
		case "decision":
			view.CurrentDecisions = append(view.CurrentDecisions, result)
		case "requirement":
			view.FinalRequirements = append(view.FinalRequirements, result)
		case "deviation":
			if candidate.Status == "accepted" || candidate.Status == "current" {
				view.AcceptedDeviations = append(view.AcceptedDeviations, result)
			}
		}
	}
	for _, records := range [][]Result{view.FinalScope, view.CurrentDecisions, view.FinalRequirements, view.AcceptedDeviations, view.SupersededHistory} {
		sortResults(records)
	}
	return view, nil
}

func resultFromCandidate(candidate store.MemoryCandidate) Result {
	return Result{ID: candidate.ID, Kind: candidate.Kind, Status: candidate.Status, Summary: candidate.Summary, Current: candidate.ValidTo == nil && candidate.Status == "current", CreatedAt: candidate.CreatedAt, ChangeKey: candidate.ChangeKey, ArtifactPath: candidate.ArtifactPath, RevisionID: candidate.RevisionID, RevisionHash: candidate.RevisionHash}
}
func sortResults(records []Result) {
	sort.Slice(records, func(i, j int) bool {
		if records[i].CreatedAt.Equal(records[j].CreatedAt) {
			return records[i].ID < records[j].ID
		}
		return records[i].CreatedAt.Before(records[j].CreatedAt)
	})
}
