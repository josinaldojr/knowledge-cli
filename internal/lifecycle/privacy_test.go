package lifecycle

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"kv/internal/hook"
	"kv/internal/memory"
	"kv/internal/openspec"
	"kv/internal/snapshot"
	"kv/internal/store"
)

func TestGlobalStoreKeepsWorkspaceMemoryAndSnapshotsPrivate(t *testing.T) {
	ctx := context.Background()
	dataHome := t.TempDir()
	t.Setenv("KV_DATA_HOME", dataHome)
	t.Setenv("KV_CACHE_HOME", filepath.Join(dataHome, "cache"))
	paths, err := store.ResolvePaths()
	if err != nil {
		t.Fatal(err)
	}
	if paths.DataDir != dataHome || paths.SnapshotDir != filepath.Join(dataHome, "snapshots") {
		t.Fatalf("global paths = %#v", paths)
	}

	database, err := store.Open(ctx, paths.DatabasePath)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if err := database.Migrate(ctx, paths.DatabasePath); err != nil {
		t.Fatal(err)
	}
	repository := store.NewRepository(database)
	snapshots, err := snapshot.Open(paths.SnapshotDir, 1<<20, nil)
	if err != nil {
		t.Fatal(err)
	}

	workspaceA := t.TempDir()
	workspaceB := t.TempDir()
	proposalA := filepath.Join(workspaceA, "proposal.md")
	proposalB := filepath.Join(workspaceB, "proposal.md")
	for path, contents := range map[string]string{
		proposalA:                             "## Why\n\nWorkspace A constraint",
		proposalB:                             "## Why\n\nWorkspace B constraint",
		filepath.Join(workspaceA, "chat.log"): "provider transcript must not persist",
		filepath.Join(workspaceA, "reasoning.md"): "chain-of-thought must not persist",
		filepath.Join(workspaceA, "main.go"):      "package main // non-OpenSpec source must not persist",
	} {
		if err := os.WriteFile(path, []byte(contents), 0600); err != nil {
			t.Fatal(err)
		}
	}

	persistMemory := func(workspace, proposal, summary string) string {
		t.Helper()
		session, err := NewSessionService(repository).EnsureSession(ctx, workspace, hook.ProviderIdentity{Kind: hook.ProviderOpenCode, NativeSessionID: workspace})
		if err != nil {
			t.Fatal(err)
		}
		changeID, err := repository.EnsureOpenSpecChange(ctx, session.Workspace.LogicalID, "same-change")
		if err != nil {
			t.Fatal(err)
		}
		manifest, contents, err := snapshot.Build(openspec.Artifact{LogicalType: "proposal", Path: proposal, Status: "observed"}, "", time.Now())
		if err != nil {
			t.Fatal(err)
		}
		if _, err := snapshots.Put(manifest, contents); err != nil {
			t.Fatal(err)
		}
		revisionID, err := repository.CreateArtifactRevisionWithMetadata(ctx, changeID, manifest.CanonicalPath, manifest.ContentHash, store.ArtifactRevisionMetadata{LogicalType: manifest.LogicalType, SizeBytes: manifest.Size, Status: manifest.Status, ParserVersion: manifest.ParserVersion})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := repository.CreateTemporalMemory(ctx, session.Workspace.LogicalID, store.MemoryRecord{Kind: "decision", Status: "current", Summary: summary, RevisionIDs: []string{revisionID}}); err != nil {
			t.Fatal(err)
		}
		return session.Workspace.LogicalID
	}

	workspaceAID := persistMemory(workspaceA, proposalA, "shared token from workspace A")
	persistMemory(workspaceB, proposalB, "shared token from workspace B")

	service := NewHookApplicationService(NewSessionService(repository), repository).WithMemoryRetriever(memory.NewRetriever(repository))
	result, err := service.Handle(ctx, hook.HookEnvelope{SchemaVersion: hook.SchemaVersion, EventID: "workspace-a-before", IdempotencyKey: "workspace-a-before", Event: hook.EventBeforeApply, OccurredAt: time.Now(), Provider: hook.ProviderIdentity{Kind: hook.ProviderOpenCode, NativeSessionID: "query-a"}, Workspace: hook.WorkspaceIdentity{CWD: workspaceA}, Subject: hook.HookSubject{ChangeID: "same-change", Operation: hook.Operation{ID: "query-a", Kind: "apply"}}})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Memory) != 1 || result.Memory[0].Summary != "shared token from workspace A" {
		t.Fatalf("workspace A memory = %#v", result.Memory)
	}
	if candidates, err := repository.ListMemoryCandidates(ctx, workspaceAID); err != nil || len(candidates) != 1 || candidates[0].Summary != "shared token from workspace A" {
		t.Fatalf("workspace A candidates = %#v, err = %v", candidates, err)
	}

	rows, err := database.DB.QueryContext(ctx, `SELECT canonical_path FROM artifact_revisions ORDER BY canonical_path`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var persistedPaths []string
	for rows.Next() {
		var path string
		if err := rows.Scan(&path); err != nil {
			t.Fatal(err)
		}
		persistedPaths = append(persistedPaths, path)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	canonicalProposalA, err := filepath.EvalSymlinks(proposalA)
	if err != nil {
		t.Fatal(err)
	}
	canonicalProposalB, err := filepath.EvalSymlinks(proposalB)
	if err != nil {
		t.Fatal(err)
	}
	if len(persistedPaths) != 2 || persistedPaths[0] != canonicalProposalA || persistedPaths[1] != canonicalProposalB {
		t.Fatalf("persisted artifact paths = %#v", persistedPaths)
	}
	entries, err := os.ReadDir(filepath.Join(paths.SnapshotDir, "contents"))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("snapshot entries = %#v", entries)
	}
}
