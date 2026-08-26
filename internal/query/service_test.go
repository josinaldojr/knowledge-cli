package query

import (
	"context"
	"kv/internal/memory"
	"kv/internal/store"
	"testing"
	"time"
)

type memories struct{}

func (memories) ListMemoryCandidates(_ context.Context, _ string) ([]store.MemoryCandidate, error) {
	return []store.MemoryCandidate{{ID: "memory", Status: "current", ChangeKey: "change", CreatedAt: time.Now()}}, nil
}

type diagnostics struct{}

func (diagnostics) ListDiagnostics(_ context.Context, _ string) ([]store.DiagnosticRecord, error) {
	return []store.DiagnosticRecord{{ID: "diagnostic", ChangeKey: "change", ArtifactPath: "/repo/spec.md", RevisionID: "revision", Status: "active", Derivation: "openspec_snapshot_analysis"}}, nil
}
func TestServiceReturnsEvidenceLinkedReadModels(t *testing.T) {
	service := New(memory.NewRetriever(memories{}), diagnostics{})
	results, err := service.Memories(context.Background(), memory.Query{WorkspaceID: "workspace", ChangeKey: "change"})
	if err != nil || len(results) != 1 || results[0].ChangeKey == "" {
		t.Fatalf("memories = %#v, err = %v", results, err)
	}
	found, err := service.Diagnostics(context.Background(), "workspace")
	if err != nil || len(found) != 1 || found[0].RevisionID == "" || found[0].Derivation == "" {
		t.Fatalf("diagnostics = %#v, err = %v", found, err)
	}
}
