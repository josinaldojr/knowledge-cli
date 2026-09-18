// Package memory provides deterministic, evidence-backed engineering-memory retrieval.
package memory

import (
	"context"
	"sort"
	"time"

	"kv/internal/retrieval"
	"kv/internal/store"
)

type CandidateRepository interface {
	ListMemoryCandidates(context.Context, string) ([]store.MemoryCandidate, error)
}

type Query struct {
	WorkspaceID, ChangeKey, Text string
	Limit                        int
}

type Result struct {
	ID, Kind, Status, Summary                         string
	Score                                             int
	Current                                           bool
	CreatedAt                                         time.Time
	ChangeKey, ArtifactPath, RevisionID, RevisionHash string
}

type Retriever struct{ repository CandidateRepository }

func NewRetriever(repository CandidateRepository) *Retriever {
	return &Retriever{repository: repository}
}

// Retrieve is lexical and deterministic. It never queries outside the supplied
// logical workspace, even where text or change identifiers match.
func (r *Retriever) Retrieve(ctx context.Context, query Query) ([]Result, error) {
	candidates, err := r.repository.ListMemoryCandidates(ctx, query.WorkspaceID)
	if err != nil {
		return nil, err
	}
	tokens := retrieval.AlnumTokens(query.Text)
	results := make([]Result, 0, len(candidates))
	for _, candidate := range candidates {
		score := 0
		sameChange := query.ChangeKey != "" && candidate.ChangeKey == query.ChangeKey
		if sameChange {
			score += 1000
		}
		matches := retrieval.Overlap(tokens, retrieval.AlnumTokens(candidate.Summary+" "+candidate.Kind+" "+candidate.ChangeKey+" "+candidate.ArtifactPath))
		if !sameChange && matches == 0 {
			continue
		}
		score += matches * 100
		current := candidate.Status == "current" && candidate.ValidTo == nil
		if current {
			score += 50
		} else {
			score -= 50
		}
		if candidate.SupersedesID != "" || candidate.ValidTo != nil {
			score -= 100
		}
		if score <= 0 {
			continue
		}
		results = append(results, Result{ID: candidate.ID, Kind: candidate.Kind, Status: candidate.Status, Summary: candidate.Summary, Score: score, Current: current, CreatedAt: candidate.CreatedAt, ChangeKey: candidate.ChangeKey, ArtifactPath: candidate.ArtifactPath, RevisionID: candidate.RevisionID, RevisionHash: candidate.RevisionHash})
	}
	sort.SliceStable(results, func(i, j int) bool {
		if results[i].Score != results[j].Score {
			return results[i].Score > results[j].Score
		}
		if !results[i].CreatedAt.Equal(results[j].CreatedAt) {
			return results[i].CreatedAt.After(results[j].CreatedAt)
		}
		if results[i].ID != results[j].ID {
			return results[i].ID < results[j].ID
		}
		return results[i].RevisionID < results[j].RevisionID
	})
	if query.Limit > 0 && len(results) > query.Limit {
		results = results[:query.Limit]
	}
	return results, nil
}
