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

type recordingCapturer struct{ afterOperationIDs []string }

type archiveConsolidator struct {
	called bool
	view   memory.ConsolidatedView
}

func (c *archiveConsolidator) Consolidate(_ context.Context, _, _ string) (memory.ConsolidatedView, error) {
	c.called = true
	return c.view, nil
}

func (c *recordingCapturer) CaptureBefore(context.Context, hook.HookEnvelope, string, string) ([]hook.ArtifactRevision, error) {
	return nil, nil
}

func (c *recordingCapturer) CaptureAfter(_ context.Context, _ hook.HookEnvelope, _ string, operationID string) ([]hook.ArtifactRevision, error) {
	c.afterOperationIDs = append(c.afterOperationIDs, operationID)
	return nil, nil
}

func TestEnsureSessionIsLazyAndIdempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "kv.db")
	db, err := store.Open(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.Migrate(context.Background(), path); err != nil {
		t.Fatal(err)
	}
	service := NewSessionService(store.NewRepository(db))
	provider := hook.ProviderIdentity{Kind: hook.ProviderCodex, CorrelationID: "fallback-session"}
	first, err := service.EnsureSession(context.Background(), t.TempDir(), provider)
	if err != nil {
		t.Fatal(err)
	}
	second, err := service.EnsureSession(context.Background(), first.Workspace.CWD, provider)
	if err != nil {
		t.Fatal(err)
	}
	if first.ID == "" || first.ID != second.ID || first.Status != hook.SessionActive {
		t.Fatalf("sessions = %#v, %#v", first, second)
	}
}

func TestSessionTransitions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "kv.db")
	db, err := store.Open(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.Migrate(context.Background(), path); err != nil {
		t.Fatal(err)
	}
	service := NewSessionService(store.NewRepository(db))
	session, err := service.EnsureSession(context.Background(), t.TempDir(), hook.ProviderIdentity{Kind: hook.ProviderOpenCode, NativeSessionID: "native"})
	if err != nil {
		t.Fatal(err)
	}
	if err := service.Transition(context.Background(), session.ID, hook.SessionCompleted); err != nil {
		t.Fatal(err)
	}
	if err := service.Transition(context.Background(), session.ID, hook.SessionActive); err == nil {
		t.Fatal("completed session became active")
	}
}

func TestHookIntakeReturnsPriorResultForDuplicateDelivery(t *testing.T) {
	path := filepath.Join(t.TempDir(), "kv.db")
	db, err := store.Open(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.Migrate(context.Background(), path); err != nil {
		t.Fatal(err)
	}
	repository := store.NewRepository(db)
	service := NewHookApplicationService(NewSessionService(repository), repository)
	envelope := hook.HookEnvelope{SchemaVersion: hook.SchemaVersion, EventID: "event-1", IdempotencyKey: "key-1", Event: hook.EventSessionObserved, OccurredAt: time.Now(), Provider: hook.ProviderIdentity{Kind: hook.ProviderOpenCode, NativeSessionID: "native"}, Workspace: hook.WorkspaceIdentity{CWD: t.TempDir()}}
	first, err := service.Handle(context.Background(), envelope)
	if err != nil {
		t.Fatal(err)
	}
	envelope.EventID = "event-2"
	second, err := service.Handle(context.Background(), envelope)
	if err != nil {
		t.Fatal(err)
	}
	if first.SessionID != second.SessionID || first.Status != second.Status {
		t.Fatalf("duplicate result = %#v, want %#v", second, first)
	}
}

func TestHookIntakeCorrelatesBeforeAndAfterOperation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "kv.db")
	db, err := store.Open(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.Migrate(context.Background(), path); err != nil {
		t.Fatal(err)
	}
	repository := store.NewRepository(db)
	service := NewHookApplicationService(NewSessionService(repository), repository)
	base := hook.HookEnvelope{SchemaVersion: hook.SchemaVersion, IdempotencyKey: "before", Event: hook.EventBeforeApply, OccurredAt: time.Now(), Provider: hook.ProviderIdentity{Kind: hook.ProviderOpenCode, NativeSessionID: "native"}, Workspace: hook.WorkspaceIdentity{CWD: t.TempDir()}, Subject: hook.HookSubject{ChangeID: "change", Operation: hook.Operation{ID: "op-correlation", Kind: "apply"}}}
	base.EventID = "before"
	before, err := service.Handle(context.Background(), base)
	if err != nil {
		t.Fatal(err)
	}
	base.EventID, base.IdempotencyKey, base.Event = "after", "after", hook.EventAfterApply
	after, err := service.Handle(context.Background(), base)
	if err != nil {
		t.Fatal(err)
	}
	if before.OperationID == "" || before.OperationID != after.OperationID {
		t.Fatalf("operations = %#v, %#v", before, after)
	}
	var status string
	if err := db.DB.QueryRow(`SELECT status FROM operations WHERE id = ?`, after.OperationID).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != "completed" {
		t.Fatalf("status = %q", status)
	}
}

func TestHookIntakeAcceptsOutOfOrderNoChangeAfterHook(t *testing.T) {
	path := filepath.Join(t.TempDir(), "kv.db")
	db, err := store.Open(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.Migrate(context.Background(), path); err != nil {
		t.Fatal(err)
	}
	repository := store.NewRepository(db)
	service := NewHookApplicationService(NewSessionService(repository), repository)
	envelope := hook.HookEnvelope{SchemaVersion: hook.SchemaVersion, EventID: "after-only", IdempotencyKey: "after-only", Event: hook.EventAfterApply, OccurredAt: time.Now(), Provider: hook.ProviderIdentity{Kind: hook.ProviderCodex, CorrelationID: "fallback"}, Workspace: hook.WorkspaceIdentity{CWD: t.TempDir()}, Subject: hook.HookSubject{ChangeID: "change", Operation: hook.Operation{ID: "out-of-order", Kind: "apply"}, NoChange: true}}
	result, err := service.Handle(context.Background(), envelope)
	if err != nil {
		t.Fatal(err)
	}
	var status string
	if err := db.DB.QueryRow(`SELECT status FROM operations WHERE id = ?`, result.OperationID).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != "no_change" {
		t.Fatalf("status = %q", status)
	}
}

func TestAfterArchiveReturnsConsolidatedEvidenceBackedMemory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "kv.db")
	db, err := store.Open(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.Migrate(context.Background(), path); err != nil {
		t.Fatal(err)
	}
	repository := store.NewRepository(db)
	consolidator := &archiveConsolidator{view: memory.ConsolidatedView{CurrentDecisions: []memory.Result{{ID: "decision", Kind: "decision", Status: "current", Summary: "Use WAL", ChangeKey: "change", ArtifactPath: "/workspace/design.md", RevisionID: "revision"}}}}
	service := NewHookApplicationService(NewSessionService(repository), repository).WithArchiveConsolidator(consolidator)
	envelope := hook.HookEnvelope{SchemaVersion: hook.SchemaVersion, EventID: "archive", IdempotencyKey: "archive", Event: hook.EventAfterArchive, OccurredAt: time.Now(), Provider: hook.ProviderIdentity{Kind: hook.ProviderOpenCode, NativeSessionID: "native"}, Workspace: hook.WorkspaceIdentity{CWD: t.TempDir()}, Subject: hook.HookSubject{ChangeID: "change", Operation: hook.Operation{ID: "archive-operation", Kind: "archive"}}}
	result, err := service.Handle(context.Background(), envelope)
	if err != nil {
		t.Fatal(err)
	}
	if !consolidator.called || len(result.Memory) != 1 || result.Memory[0].Source.RevisionID != "revision" {
		t.Fatalf("result = %#v, called = %t", result, consolidator.called)
	}
}

func TestNextRelatedHookReconcilesMissingAfterHook(t *testing.T) {
	path := filepath.Join(t.TempDir(), "kv.db")
	db, err := store.Open(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.Migrate(context.Background(), path); err != nil {
		t.Fatal(err)
	}
	repository := store.NewRepository(db)
	capturer := &recordingCapturer{}
	service := NewHookApplicationService(NewSessionService(repository), repository, capturer)
	base := hook.HookEnvelope{SchemaVersion: hook.SchemaVersion, OccurredAt: time.Now(), Provider: hook.ProviderIdentity{Kind: hook.ProviderOpenCode, NativeSessionID: "native"}, Workspace: hook.WorkspaceIdentity{CWD: t.TempDir()}, Subject: hook.HookSubject{ChangeID: "change", Operation: hook.Operation{ID: "missing-after", Kind: "apply"}}}
	base.EventID, base.IdempotencyKey, base.Event = "before", "before", hook.EventBeforeApply
	first, err := service.Handle(context.Background(), base)
	if err != nil {
		t.Fatal(err)
	}

	base.EventID, base.IdempotencyKey, base.Event = "next", "next", hook.EventBeforeUpdate
	base.Subject.Operation = hook.Operation{ID: "next-operation", Kind: "artifact_update"}
	result, err := service.Handle(context.Background(), base)
	if err != nil {
		t.Fatal(err)
	}
	if len(capturer.afterOperationIDs) != 1 || capturer.afterOperationIDs[0] != first.OperationID {
		t.Fatalf("reconciled operations = %#v, want %q", capturer.afterOperationIDs, first.OperationID)
	}
	if len(result.Diagnostics) != 1 || result.Diagnostics[0].Code != "missing_after_hook_reconciled" {
		t.Fatalf("diagnostics = %#v", result.Diagnostics)
	}
	var status string
	if err := db.DB.QueryRow(`SELECT status FROM operations WHERE id = ?`, first.OperationID).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != "no_change" {
		t.Fatalf("status = %q", status)
	}
}

func TestReconcileStalesAbandonedSessionAndPendingOperation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "kv.db")
	db, err := store.Open(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.Migrate(context.Background(), path); err != nil {
		t.Fatal(err)
	}
	repository := store.NewRepository(db)
	service := NewSessionService(repository)
	session, err := service.EnsureSession(context.Background(), t.TempDir(), hook.ProviderIdentity{Kind: hook.ProviderOpenCode, NativeSessionID: "old"})
	if err != nil {
		t.Fatal(err)
	}
	workspaceID := session.Workspace.LogicalID
	changeID, err := repository.EnsureOpenSpecChange(context.Background(), workspaceID, "change")
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := repository.EnsureOperation(context.Background(), changeID, session.ID, hook.Operation{ID: "pending", Kind: "apply"}, "pending"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.DB.Exec(`UPDATE provider_sessions SET last_seen_at = '2000-01-01T00:00:00Z'`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.DB.Exec(`UPDATE operations SET created_at = '2000-01-01T00:00:00Z'`); err != nil {
		t.Fatal(err)
	}
	sessions, operations, err := service.Reconcile(context.Background(), time.Hour, time.Now())
	if err != nil || sessions != 1 || operations != 1 {
		t.Fatalf("reconcile = %d, %d, %v", sessions, operations, err)
	}
}

func TestConcurrentProviderSessionsRemainDistinct(t *testing.T) {
	path := filepath.Join(t.TempDir(), "kv.db")
	db, err := store.Open(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.Migrate(context.Background(), path); err != nil {
		t.Fatal(err)
	}
	service := NewSessionService(store.NewRepository(db))
	cwd := t.TempDir()
	first, err := service.EnsureSession(context.Background(), cwd, hook.ProviderIdentity{Kind: hook.ProviderOpenCode, NativeSessionID: "one"})
	if err != nil {
		t.Fatal(err)
	}
	second, err := service.EnsureSession(context.Background(), cwd, hook.ProviderIdentity{Kind: hook.ProviderClaudeCode, NativeSessionID: "two"})
	if err != nil {
		t.Fatal(err)
	}
	if first.ID == second.ID {
		t.Fatal("different provider sessions shared an ID")
	}
	if err := service.Transition(context.Background(), first.ID, hook.SessionFailed); err != nil {
		t.Fatal(err)
	}
}
