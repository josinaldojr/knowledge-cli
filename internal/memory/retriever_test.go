package memory

import (
	"context"
	"testing"
	"time"

	"kv/internal/store"
)

type fakeRepository struct{ candidates []store.MemoryCandidate }

func (f fakeRepository) ListMemoryCandidates(_ context.Context, _ string) ([]store.MemoryCandidate, error) {
	return f.candidates, nil
}

type observingRepository struct {
	workspaceID string
	candidates  []store.MemoryCandidate
}

func (r *observingRepository) ListMemoryCandidates(_ context.Context, workspaceID string) ([]store.MemoryCandidate, error) {
	r.workspaceID = workspaceID
	return r.candidates, nil
}

func TestRetrieverScopesEveryQueryToRequestedWorkspace(t *testing.T) {
	repository := &observingRepository{candidates: []store.MemoryCandidate{{ID: "memory", Status: "current", ChangeKey: "change", CreatedAt: time.Now()}}}
	if _, err := NewRetriever(repository).Retrieve(context.Background(), Query{WorkspaceID: "workspace-a", ChangeKey: "change"}); err != nil {
		t.Fatal(err)
	}
	if repository.workspaceID != "workspace-a" {
		t.Fatalf("workspace query = %q", repository.workspaceID)
	}
}

func TestRetrieverRanksSameChangeCurrentLexicalMemory(t *testing.T) {
	now := time.Now().UTC()
	repository := fakeRepository{candidates: []store.MemoryCandidate{{ID: "old", Status: "current", Summary: "Use SQLite WAL", ChangeKey: "other", CreatedAt: now.Add(-time.Hour)}, {ID: "same", Status: "current", Summary: "Use SQLite WAL for concurrent hooks", ChangeKey: "add-hooks", CreatedAt: now.Add(-2 * time.Hour)}, {ID: "superseded", Status: "current", Summary: "Use SQLite WAL", ChangeKey: "add-hooks", SupersedesID: "prior", CreatedAt: now}}}
	results, err := NewRetriever(repository).Retrieve(context.Background(), Query{WorkspaceID: "workspace", ChangeKey: "add-hooks", Text: "SQLite concurrent hooks"})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 3 || results[0].ID != "same" {
		t.Fatalf("results = %#v", results)
	}
}

func TestRetrieverReturnsEmptyWithoutChangeOrLexicalMatch(t *testing.T) {
	results, err := NewRetriever(fakeRepository{candidates: []store.MemoryCandidate{{ID: "memory", Status: "current", Summary: "unrelated", ChangeKey: "other", CreatedAt: time.Now()}}}).Retrieve(context.Background(), Query{WorkspaceID: "workspace", Text: "desired"})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 0 {
		t.Fatalf("results = %#v", results)
	}
}
