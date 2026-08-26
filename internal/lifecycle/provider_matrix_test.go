package lifecycle

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"kv/internal/adapter/claudecode"
	"kv/internal/adapter/codex"
	"kv/internal/adapter/opencode"
	"kv/internal/hook"
	"kv/internal/memory"
	"kv/internal/snapshot"
	"kv/internal/store"
)

type providerEnvelope func(string, string, string, string, hook.EventName) (hook.HookEnvelope, error)

func TestProviderLifecycleMatrixEndToEnd(t *testing.T) {
	providers := []struct {
		name string
		new  func() providerEnvelope
	}{
		{
			name: "opencode",
			new: func() providerEnvelope {
				adapter := opencode.New()
				return func(cwd, session, change, operation string, event hook.EventName) (hook.HookEnvelope, error) {
					return adapter.Envelope(opencode.Event{CWD: cwd, NativeSessionID: session, ChangeID: change, OperationID: operation, OperationKind: "artifact_update", Name: event})
				}
			},
		},
		{
			name: "claude-code",
			new: func() providerEnvelope {
				adapter := claudecode.New()
				return func(cwd, session, change, operation string, event hook.EventName) (hook.HookEnvelope, error) {
					return adapter.Envelope(claudecode.Event{CWD: cwd, ClaudeSessionID: session, ChangeID: change, OperationID: operation, OperationKind: "artifact_update", Name: event})
				}
			},
		},
		{
			name: "codex",
			new: func() providerEnvelope {
				adapter := codex.New()
				return func(cwd, _, change, operation string, event hook.EventName) (hook.HookEnvelope, error) {
					return adapter.Envelope(codex.Event{CWD: cwd, ChangeID: change, OperationID: operation, OperationKind: "artifact_update", Name: event})
				}
			},
		},
	}

	for _, provider := range providers {
		t.Run(provider.name, func(t *testing.T) {
			ctx := context.Background()
			workspace := t.TempDir()
			proposal := filepath.Join(workspace, "proposal.md")
			if err := os.WriteFile(proposal, []byte("## Why\n\nInitial provider matrix scope\n"), 0600); err != nil {
				t.Fatal(err)
			}
			installProviderMatrixOpenSpecStub(t, workspace, proposal)

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
			snapshots, err := snapshot.Open(filepath.Join(t.TempDir(), "snapshots"), 1<<20, nil)
			if err != nil {
				t.Fatal(err)
			}
			sessions := NewSessionService(repository)
			service := NewHookApplicationService(sessions, repository, snapshot.NewCapturer(snapshots, repository)).WithMemoryRetriever(memory.NewRetriever(repository)).WithArchiveConsolidator(memory.NewConsolidator(repository))
			makeEnvelope := provider.new()

			before, err := makeEnvelope(workspace, "first", "provider-matrix", "change", hook.EventBeforeUpdate)
			if err != nil {
				t.Fatal(err)
			}
			first, err := service.Handle(ctx, before)
			if err != nil || first.SessionID == "" || first.OperationID == "" {
				t.Fatalf("automatic session registration = %#v, err = %v", first, err)
			}
			if _, err := os.Stat(filepath.Join(workspace, ".kv")); !os.IsNotExist(err) {
				t.Fatalf("zero-init discovery created project-local .kv: %v", err)
			}

			retry := before
			retry.EventID = before.EventID + "-retry"
			retried, err := service.Handle(ctx, retry)
			if err != nil || retried.SessionID != first.SessionID || retried.OperationID != first.OperationID {
				t.Fatalf("idempotent retry = %#v, err = %v", retried, err)
			}

			if err := os.WriteFile(proposal, []byte("## Why\n\nExpanded provider matrix scope\n"), 0600); err != nil {
				t.Fatal(err)
			}
			after, err := makeEnvelope(workspace, "first", "provider-matrix", "change", hook.EventAfterUpdate)
			if err != nil {
				t.Fatal(err)
			}
			transformed, err := service.Handle(ctx, after)
			if err != nil || transformed.Transformation == nil || transformed.Transformation.ID == "" {
				t.Fatalf("verified transformation = %#v, err = %v", transformed, err)
			}
			if !hasDiagnostic(transformed.Diagnostics, "scope_spec_drift") {
				t.Fatalf("post-operation diagnostics = %#v", transformed.Diagnostics)
			}

			noChangeBefore, err := makeEnvelope(workspace, "first", "provider-matrix", "no-change", hook.EventBeforeApply)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := service.Handle(ctx, noChangeBefore); err != nil {
				t.Fatal(err)
			}
			noChangeAfter, err := makeEnvelope(workspace, "first", "provider-matrix", "no-change", hook.EventAfterApply)
			if err != nil {
				t.Fatal(err)
			}
			noChange, err := service.Handle(ctx, noChangeAfter)
			if err != nil || noChange.Status != "no_change" || noChange.Transformation != nil {
				t.Fatalf("no-change operation = %#v, err = %v", noChange, err)
			}

			laterEnvelope := provider.new()
			later, err := laterEnvelope(workspace, "later", "provider-matrix", "retrieve", hook.EventBeforeApply)
			if err != nil {
				t.Fatal(err)
			}
			retrieved, err := service.Handle(ctx, later)
			if err != nil || len(retrieved.Memory) == 0 || retrieved.Memory[0].Source.RevisionID == "" {
				t.Fatalf("later-session retrieval = %#v, err = %v", retrieved, err)
			}

			if _, err := database.DB.Exec(`UPDATE provider_sessions SET last_seen_at = '2000-01-01T00:00:00Z' WHERE id = ?`, first.SessionID); err != nil {
				t.Fatal(err)
			}
			stale, _, err := sessions.Reconcile(ctx, time.Hour, time.Now())
			if err != nil || stale != 1 {
				t.Fatalf("stale-session reconciliation = %d, err = %v", stale, err)
			}

			archive, err := laterEnvelope(workspace, "later", "provider-matrix", "archive", hook.EventAfterArchive)
			if err != nil {
				t.Fatal(err)
			}
			archived, err := service.Handle(ctx, archive)
			if err != nil || archived.Degraded || len(archived.Memory) == 0 {
				t.Fatalf("archive consolidation = %#v, err = %v", archived, err)
			}
		})
	}
}

func hasDiagnostic(diagnostics []hook.Diagnostic, code string) bool {
	for _, diagnostic := range diagnostics {
		if diagnostic.Code == code {
			return true
		}
	}
	return false
}

func installProviderMatrixOpenSpecStub(t *testing.T, workspace, proposal string) {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "bin")
	if err := os.MkdirAll(bin, 0700); err != nil {
		t.Fatal(err)
	}
	status := fmt.Sprintf(`{"artifactPaths":{"proposal":{"existingOutputPaths":[%q]}}}`, proposal)
	instructions := fmt.Sprintf(`{"contextFiles":{"proposal":[%q]}}`, proposal)
	script := fmt.Sprintf("#!/bin/sh\ncase \"$1\" in\nstatus) printf '%%s' '%s' ;;\ninstructions) printf '%%s' '%s' ;;\nesac\n", status, instructions)
	if err := os.WriteFile(filepath.Join(bin, "openspec"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
}
