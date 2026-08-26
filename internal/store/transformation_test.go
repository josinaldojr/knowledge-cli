package store

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"kv/internal/hook"
)

func TestCreateTransformationPersistsItemsAtomically(t *testing.T) {
	path := filepath.Join(t.TempDir(), "kv.db")
	db, err := Open(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.Migrate(context.Background(), path); err != nil {
		t.Fatal(err)
	}
	repository := NewRepository(db)
	workspaceID, err := repository.EnsureWorkspace(context.Background(), "workspace")
	if err != nil {
		t.Fatal(err)
	}
	instanceID, err := repository.EnsureWorkspaceInstance(context.Background(), workspaceID, "/workspace")
	if err != nil {
		t.Fatal(err)
	}
	session, err := repository.EnsureProviderSession(context.Background(), instanceID, hook.ProviderIdentity{Kind: hook.ProviderOpenCode, NativeSessionID: "session"})
	if err != nil {
		t.Fatal(err)
	}
	changeID, err := repository.EnsureOpenSpecChange(context.Background(), workspaceID, "change")
	if err != nil {
		t.Fatal(err)
	}
	operationID, _, err := repository.EnsureOperation(context.Background(), changeID, session.ID, hook.Operation{ID: "operation", Kind: "apply"}, "completed")
	if err != nil {
		t.Fatal(err)
	}
	beforeID, err := repository.CreateArtifactRevision(context.Background(), changeID, "/workspace/proposal.md", "before")
	if err != nil {
		t.Fatal(err)
	}
	afterID, err := repository.CreateArtifactRevision(context.Background(), changeID, "/workspace/proposal.md", "after")
	if err != nil {
		t.Fatal(err)
	}
	if err := repository.LinkOperationRevision(context.Background(), operationID, beforeID, "before"); err != nil {
		t.Fatal(err)
	}
	if err := repository.LinkOperationRevision(context.Background(), operationID, afterID, "after"); err != nil {
		t.Fatal(err)
	}
	transformationID, err := repository.CreateTransformation(context.Background(), operationID, "Added a requirement", []TransformationItem{{Kind: "requirement", Status: "added", Summary: "Automatic session"}})
	if err != nil {
		t.Fatal(err)
	}
	var count int
	if err := db.DB.QueryRow(`SELECT COUNT(*) FROM transformation_items WHERE transformation_id = ?`, transformationID).Scan(&count); err != nil || count != 1 {
		t.Fatalf("items = %d, err = %v", count, err)
	}
}

func TestCreateTransformationRejectsNoChangeOperation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "kv.db")
	db, err := Open(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.Migrate(context.Background(), path); err != nil {
		t.Fatal(err)
	}
	repository := NewRepository(db)
	workspaceID, _ := repository.EnsureWorkspace(context.Background(), "workspace")
	instanceID, _ := repository.EnsureWorkspaceInstance(context.Background(), workspaceID, "/workspace")
	session, _ := repository.EnsureProviderSession(context.Background(), instanceID, hook.ProviderIdentity{Kind: hook.ProviderOpenCode, NativeSessionID: "session"})
	changeID, _ := repository.EnsureOpenSpecChange(context.Background(), workspaceID, "change")
	operationID, _, _ := repository.EnsureOperation(context.Background(), changeID, session.ID, hook.Operation{ID: "no-change", Kind: "apply"}, "completed")
	if _, err := repository.CreateTransformation(context.Background(), operationID, "ignored", nil); !errors.Is(err, ErrNoOperationChanges) {
		t.Fatalf("error = %v", err)
	}
	var count int
	if err := db.DB.QueryRow(`SELECT COUNT(*) FROM transformations`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("transformations = %d, err = %v", count, err)
	}
}

func TestCreateTemporalMemoryRequiresEvidenceAndSupersedes(t *testing.T) {
	path := filepath.Join(t.TempDir(), "kv.db")
	db, err := Open(context.Background(), path)
	if err != nil { t.Fatal(err) }
	defer db.Close()
	if err := db.Migrate(context.Background(), path); err != nil { t.Fatal(err) }
	repository := NewRepository(db)
	workspaceID, _ := repository.EnsureWorkspace(context.Background(), "workspace")
	if _, err := repository.CreateTemporalMemory(context.Background(), workspaceID, MemoryRecord{Kind: "decision"}); err == nil { t.Fatal("memory without evidence accepted") }
	changeID, _ := repository.EnsureOpenSpecChange(context.Background(), workspaceID, "change")
	revisionID, _ := repository.CreateArtifactRevision(context.Background(), changeID, "/workspace/design.md", "hash")
	first, err := repository.CreateTemporalMemory(context.Background(), workspaceID, MemoryRecord{Kind: "decision", Status: "current", Summary: "old", RevisionIDs: []string{revisionID}})
	if err != nil { t.Fatal(err) }
	second, err := repository.CreateTemporalMemory(context.Background(), workspaceID, MemoryRecord{Kind: "decision", Status: "current", Summary: "new", SupersedesID: first, RevisionIDs: []string{revisionID}})
	if err != nil || second == "" { t.Fatalf("memory = %q, err = %v", second, err) }
	var validTo string
	if err := db.DB.QueryRow(`SELECT valid_to FROM memories WHERE id = ?`, first).Scan(&validTo); err != nil || validTo == "" { t.Fatalf("valid_to = %q, err = %v", validTo, err) }
}
