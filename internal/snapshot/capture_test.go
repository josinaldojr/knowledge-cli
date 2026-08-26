package snapshot

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"kv/internal/hook"
	"kv/internal/openspec"
	"kv/internal/store"
)

func TestCapturePersistsPreOperationRevisions(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "proposal.md")
	if err := os.WriteFile(path, []byte("## Why\n\nInitial scope"), 0600); err != nil {
		t.Fatal(err)
	}
	dbPath := filepath.Join(dir, "kv.db")
	db, err := store.Open(context.Background(), dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.Migrate(context.Background(), dbPath); err != nil {
		t.Fatal(err)
	}
	repository := store.NewRepository(db)
	workspaceID, err := repository.EnsureWorkspace(context.Background(), "workspace")
	if err != nil {
		t.Fatal(err)
	}
	changeID, err := repository.EnsureOpenSpecChange(context.Background(), workspaceID, "change")
	if err != nil {
		t.Fatal(err)
	}
	snapshots, err := Open(filepath.Join(dir, "snapshots"), 1024, nil)
	if err != nil {
		t.Fatal(err)
	}
	instanceID, err := repository.EnsureWorkspaceInstance(context.Background(), workspaceID, dir)
	if err != nil {
		t.Fatal(err)
	}
	session, err := repository.EnsureProviderSession(context.Background(), instanceID, hook.ProviderIdentity{Kind: hook.ProviderOpenCode, NativeSessionID: "test"})
	if err != nil {
		t.Fatal(err)
	}
	operationID, _, err := repository.EnsureOperation(context.Background(), changeID, session.ID, hook.Operation{ID: "operation", Kind: "apply"}, "pending")
	if err != nil {
		t.Fatal(err)
	}
	revisions, err := NewCapturer(snapshots, repository).capture(context.Background(), []openspec.Artifact{{LogicalType: "proposal", Path: path, Status: "observed"}}, changeID, operationID, "before", "head", time.Now())
	if err != nil || len(revisions) != 1 || revisions[0].ID == "" {
		t.Fatalf("revisions = %#v, err = %v", revisions, err)
	}
	var logicalType, parserVersion string
	if err := db.DB.QueryRow(`SELECT logical_type, parser_version FROM artifact_revisions WHERE id = ?`, revisions[0].ID).Scan(&logicalType, &parserVersion); err != nil || logicalType != "proposal" || parserVersion == "" {
		t.Fatalf("provenance = %q, %q, err = %v", logicalType, parserVersion, err)
	}
	if err := os.WriteFile(path, []byte("## Why\n\nExpanded scope"), 0600); err != nil {
		t.Fatal(err)
	}
	capturer := NewCapturer(snapshots, repository)
	if _, err := capturer.capture(context.Background(), []openspec.Artifact{{LogicalType: "proposal", Path: path, Status: "observed"}}, changeID, operationID, "after", "head", time.Now()); err != nil {
		t.Fatal(err)
	}
	changed, err := repository.OperationHasChanges(context.Background(), operationID)
	if err != nil || !changed {
		t.Fatalf("changed = %t, err = %v", changed, err)
	}
	transformation, err := capturer.RecordTransformation(context.Background(), operationID, workspaceID, hook.ProviderSummary{})
	if err != nil || transformation == nil || transformation.ID == "" {
		t.Fatalf("transformation = %#v, err = %v", transformation, err)
	}
	memories, err := repository.ListMemoryCandidates(context.Background(), workspaceID)
	if err != nil || len(memories) != 1 || memories[0].Kind != "scope" || memories[0].RevisionID == "" {
		t.Fatalf("memories = %#v, err = %v", memories, err)
	}
}

func TestOperationHasChangesDetectsCreatedRemovedRenamedAndUnchangedArtifacts(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "kv.db")
	db, err := store.Open(context.Background(), dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.Migrate(context.Background(), dbPath); err != nil {
		t.Fatal(err)
	}
	repository := store.NewRepository(db)
	workspaceID, err := repository.EnsureWorkspace(context.Background(), "workspace")
	if err != nil {
		t.Fatal(err)
	}
	changeID, err := repository.EnsureOpenSpecChange(context.Background(), workspaceID, "change")
	if err != nil {
		t.Fatal(err)
	}
	instanceID, err := repository.EnsureWorkspaceInstance(context.Background(), workspaceID, dir)
	if err != nil {
		t.Fatal(err)
	}
	session, err := repository.EnsureProviderSession(context.Background(), instanceID, hook.ProviderIdentity{Kind: hook.ProviderOpenCode, NativeSessionID: "test"})
	if err != nil {
		t.Fatal(err)
	}
	capturer := NewCapturer(mustSnapshotStore(t, dir), repository)
	artifact := func(name string) openspec.Artifact {
		return openspec.Artifact{LogicalType: "proposal", Path: filepath.Join(dir, name), Status: "observed"}
	}
	write := func(name string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(name), 0600); err != nil {
			t.Fatal(err)
		}
	}

	cases := []struct {
		name   string
		before []openspec.Artifact
		after  []openspec.Artifact
		want   bool
	}{
		{name: "created", after: []openspec.Artifact{artifact("created.md")}, want: true},
		{name: "removed", before: []openspec.Artifact{artifact("removed.md")}, want: true},
		{name: "renamed", before: []openspec.Artifact{artifact("old.md")}, after: []openspec.Artifact{artifact("new.md")}, want: true},
		{name: "unchanged", before: []openspec.Artifact{artifact("same.md")}, after: []openspec.Artifact{artifact("same.md")}, want: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for _, item := range append(append([]openspec.Artifact{}, tc.before...), tc.after...) {
				write(filepath.Base(item.Path))
			}
			operationID, _, err := repository.EnsureOperation(context.Background(), changeID, session.ID, hook.Operation{ID: tc.name, Kind: "update"}, "pending")
			if err != nil {
				t.Fatal(err)
			}
			if _, err := capturer.capture(context.Background(), tc.before, changeID, operationID, "before", "", time.Now()); err != nil {
				t.Fatal(err)
			}
			if _, err := capturer.capture(context.Background(), tc.after, changeID, operationID, "after", "", time.Now()); err != nil {
				t.Fatal(err)
			}
			changed, err := repository.OperationHasChanges(context.Background(), operationID)
			if err != nil || changed != tc.want {
				t.Fatalf("changed = %t, want %t, err = %v", changed, tc.want, err)
			}
		})
	}
}

func mustSnapshotStore(t *testing.T, root string) *Store {
	t.Helper()
	store, err := Open(filepath.Join(root, "snapshots"), 1024, nil)
	if err != nil {
		t.Fatal(err)
	}
	return store
}
