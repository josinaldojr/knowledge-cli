package lifecycle

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"kv/internal/hook"
	"kv/internal/memory"
	"kv/internal/store"
)

func TestProvidersReceiveEquivalentMemoryForSharedChange(t *testing.T) {
	ctx := context.Background()
	databasePath := filepath.Join(t.TempDir(), "kv.db")
	database, err := store.Open(ctx, databasePath)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if err := database.Migrate(ctx, databasePath); err != nil {
		t.Fatal(err)
	}
	repository := store.NewRepository(database)
	workspace := t.TempDir()
	sessions := NewSessionService(repository)
	seed, err := sessions.EnsureSession(ctx, workspace, hook.ProviderIdentity{Kind: hook.ProviderClaudeCode, NativeSessionID: "seed"})
	if err != nil {
		t.Fatal(err)
	}
	workspaceID := seed.Workspace.LogicalID
	changeID, err := repository.EnsureOpenSpecChange(ctx, workspaceID, "shared-change")
	if err != nil {
		t.Fatal(err)
	}
	revisionID, err := repository.CreateArtifactRevision(ctx, changeID, filepath.Join(workspace, "design.md"), "design-hash")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repository.CreateTemporalMemory(ctx, workspaceID, store.MemoryRecord{Kind: "decision", Status: "current", Summary: "Use evidence-backed memory", RevisionIDs: []string{revisionID}}); err != nil {
		t.Fatal(err)
	}
	service := NewHookApplicationService(sessions, repository).WithMemoryRetriever(memory.NewRetriever(repository))
	makeEnvelope := func(provider hook.ProviderIdentity, id string) hook.HookEnvelope {
		return hook.HookEnvelope{SchemaVersion: hook.SchemaVersion, EventID: id, IdempotencyKey: id, Event: hook.EventBeforeApply, OccurredAt: time.Now(), Provider: provider, Workspace: hook.WorkspaceIdentity{CWD: workspace}, Subject: hook.HookSubject{ChangeID: "shared-change", Operation: hook.Operation{ID: id, Kind: "apply"}}}
	}
	claude, err := service.Handle(ctx, makeEnvelope(hook.ProviderIdentity{Kind: hook.ProviderClaudeCode, NativeSessionID: "claude"}, "claude-apply"))
	if err != nil {
		t.Fatal(err)
	}
	codex, err := service.Handle(ctx, makeEnvelope(hook.ProviderIdentity{Kind: hook.ProviderCodex, CorrelationID: "codex"}, "codex-apply"))
	if err != nil {
		t.Fatal(err)
	}
	if len(claude.Memory) != 1 || len(codex.Memory) != 1 || claude.Memory[0].Summary != codex.Memory[0].Summary || claude.Memory[0].Source != codex.Memory[0].Source {
		t.Fatalf("provider contexts diverged: claude=%#v codex=%#v", claude.Memory, codex.Memory)
	}
	history, err := repository.ChangeHistory(ctx, workspaceID)
	if err != nil || len(history) != 1 || history[0].Key != "shared-change" {
		t.Fatalf("shared genealogy = %#v, err = %v", history, err)
	}
}
