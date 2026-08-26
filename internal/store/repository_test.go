package store

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"kv/internal/hook"
)

func newTestRepository(t *testing.T) *Repository {
	t.Helper()
	path := filepath.Join(t.TempDir(), "kv.db")
	db, err := Open(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.Migrate(context.Background(), path); err != nil {
		t.Fatal(err)
	}
	return NewRepository(db)
}

func TestKnowledgeExportImportPreservesEvidenceWithoutPhysicalSessionData(t *testing.T) {
	ctx := context.Background()
	source := newTestRepository(t)
	workspaceID, err := source.EnsureWorkspace(ctx, "git:example/export")
	if err != nil {
		t.Fatal(err)
	}
	instanceID, err := source.EnsureWorkspaceInstance(ctx, workspaceID, "/private/workspace")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := source.EnsureProviderSession(ctx, instanceID, hook.ProviderIdentity{Kind: hook.ProviderOpenCode, NativeSessionID: "native-secret"}); err != nil {
		t.Fatal(err)
	}
	changeID, err := source.EnsureOpenSpecChange(ctx, workspaceID, "portable-memory")
	if err != nil {
		t.Fatal(err)
	}
	revisionID, err := source.CreateArtifactRevisionWithMetadata(ctx, changeID, "/private/workspace/openspec/changes/portable-memory/specs/memory.md", "a1b2", ArtifactRevisionMetadata{LogicalType: "spec", SizeBytes: 42, Status: "active", ParserVersion: "1"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := source.CreateTemporalMemory(ctx, workspaceID, MemoryRecord{Kind: "decision", Status: "current", Summary: "export verified evidence", RevisionIDs: []string{revisionID}}); err != nil {
		t.Fatal(err)
	}
	document, err := source.ExportKnowledge(ctx, workspaceID)
	if err != nil {
		t.Fatal(err)
	}
	encoded := fmt.Sprintf("%+v", document)
	if strings.Contains(encoded, "/private/workspace") || strings.Contains(encoded, "native-secret") {
		t.Fatalf("export leaked private metadata: %s", encoded)
	}
	if len(document.Memories) != 1 || len(document.Memories[0].Sources) != 1 || document.Memories[0].Sources[0].ChangeKey != "portable-memory" {
		t.Fatalf("exported evidence = %#v", document.Memories)
	}

	target := newTestRepository(t)
	if err := target.ImportKnowledge(ctx, document); err != nil {
		t.Fatal(err)
	}
	importedWorkspaceID, err := target.WorkspaceID(ctx, "git:example/export")
	if err != nil {
		t.Fatal(err)
	}
	candidates, err := target.ListMemoryCandidates(ctx, importedWorkspaceID)
	if err != nil {
		t.Fatal(err)
	}
	if len(candidates) != 1 || candidates[0].ChangeKey != "portable-memory" || candidates[0].ArtifactPath != "openspec/changes/portable-memory/specs/memory.md" || candidates[0].RevisionHash != "a1b2" {
		t.Fatalf("imported evidence = %#v", candidates)
	}
}

func TestRepositoryEnsuresStableIdentitiesAndEventIdempotency(t *testing.T) {
	r := newTestRepository(t)
	workspaceID, err := r.EnsureWorkspace(context.Background(), "git:example/repo")
	if err != nil {
		t.Fatal(err)
	}
	again, err := r.EnsureWorkspace(context.Background(), "git:example/repo")
	if err != nil || workspaceID != again {
		t.Fatalf("workspace identity = %q, %q, %v", workspaceID, again, err)
	}
	instanceID, err := r.EnsureWorkspaceInstance(context.Background(), workspaceID, "/repo")
	if err != nil {
		t.Fatal(err)
	}
	session, err := r.EnsureProviderSession(context.Background(), instanceID, hook.ProviderIdentity{Kind: hook.ProviderOpenCode, NativeSessionID: "native"})
	if err != nil {
		t.Fatal(err)
	}
	envelope := hook.HookEnvelope{SchemaVersion: hook.SchemaVersion, EventID: "event-1", IdempotencyKey: "key-1", Event: hook.EventSessionObserved, OccurredAt: time.Now(), Provider: hook.ProviderIdentity{Kind: hook.ProviderOpenCode, NativeSessionID: "native"}, Workspace: hook.WorkspaceIdentity{CWD: "/repo"}}
	first, duplicate, err := r.StoreOrGet(context.Background(), envelope, hook.HookResult{SessionID: session.ID, Status: "processed"})
	if err != nil || duplicate || first.SessionID != session.ID {
		t.Fatal(err)
	}
	replayed, duplicate, err := r.StoreOrGet(context.Background(), envelope, hook.HookResult{Status: "different"})
	if err != nil || !duplicate || replayed.SessionID != session.ID {
		t.Fatalf("duplicate = %t, result = %#v, err = %v", duplicate, replayed, err)
	}
	result, found, err := r.FindByIdempotencyKey(context.Background(), "key-1")
	if err != nil || !found || result.SessionID != session.ID {
		t.Fatalf("result = %#v, found = %t, err = %v", result, found, err)
	}
}

func TestMigrateUpgradesVersionOneStore(t *testing.T) {
	path := filepath.Join(t.TempDir(), "kv.db")
	db, err := Open(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.DB.Exec(`CREATE TABLE schema_migrations (version INTEGER PRIMARY KEY, applied_at TEXT NOT NULL)`); err != nil {
		t.Fatal(err)
	}
	for _, statement := range migrations[0].statements {
		if _, err := db.DB.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := db.DB.Exec(`INSERT INTO schema_migrations(version, applied_at) VALUES(1, '2026-01-01T00:00:00Z')`); err != nil {
		t.Fatal(err)
	}
	if err := db.Migrate(context.Background(), path); err != nil {
		t.Fatal(err)
	}
	var indexName string
	if err := db.DB.QueryRow(`SELECT name FROM sqlite_master WHERE type = 'index' AND name = 'idx_hook_events_idempotency_key'`).Scan(&indexName); err != nil {
		t.Fatal(err)
	}
}

func TestRepositoryRollsBackMemoryWithoutValidSource(t *testing.T) {
	r := newTestRepository(t)
	workspaceID, err := r.EnsureWorkspace(context.Background(), "local:/repo")
	if err != nil {
		t.Fatal(err)
	}
	_, err = r.CreateMemory(context.Background(), workspaceID, "decision", "current", "summary", []string{"missing"})
	if err == nil {
		t.Fatal("memory with missing source succeeded")
	}
	var count int
	if err := r.db.DB.QueryRow(`SELECT COUNT(*) FROM memories`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("rolled back memory count = %d", count)
	}
}

func TestRepositoryConcurrentWorkspaceEnsureIsIdempotent(t *testing.T) {
	r := newTestRepository(t)
	const workers = 12
	ids := make(chan string, workers)
	errs := make(chan error, workers)
	var group sync.WaitGroup
	for range workers {
		group.Add(1)
		go func() {
			defer group.Done()
			id, err := r.EnsureWorkspace(context.Background(), "git:example/concurrent")
			if err != nil {
				errs <- err
				return
			}
			ids <- id
		}()
	}
	group.Wait()
	close(ids)
	close(errs)
	for err := range errs {
		t.Fatal(err)
	}
	var first string
	for id := range ids {
		if first == "" {
			first = id
		}
		if id != first {
			t.Fatalf("got distinct workspace IDs %q and %q", first, id)
		}
	}
}
