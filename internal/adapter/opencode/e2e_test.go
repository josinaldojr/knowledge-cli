package opencode

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"kv/internal/hook"
	"kv/internal/lifecycle"
	"kv/internal/memory"
	"kv/internal/snapshot"
	"kv/internal/store"
)

func TestOpenCodeLifecycleEndToEnd(t *testing.T) {
	ctx := context.Background()
	workspace := t.TempDir()
	proposal := filepath.Join(workspace, "proposal.md")
	if err := os.WriteFile(proposal, []byte("## Why\n\nInitial automatic memory scope\n"), 0600); err != nil {
		t.Fatal(err)
	}
	installOpenSpecStub(t, workspace, proposal)

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
	service := lifecycle.NewHookApplicationService(lifecycle.NewSessionService(repository), repository, snapshot.NewCapturer(snapshots, repository)).WithMemoryRetriever(memory.NewRetriever(repository)).WithArchiveConsolidator(memory.NewConsolidator(repository))
	adapter := New()

	before, err := adapter.Envelope(Event{NativeSessionID: "opencode-first", CWD: workspace, ChangeID: "automatic-memory", OperationID: "propose-1", OperationKind: "propose", Name: hook.EventBeforePropose})
	if err != nil {
		t.Fatal(err)
	}
	first, err := service.Handle(ctx, before)
	if err != nil {
		t.Fatal(err)
	}
	if first.SessionID == "" || first.OperationID == "" {
		t.Fatalf("zero-init registration result = %#v", first)
	}
	if _, err := os.Stat(filepath.Join(workspace, ".kv")); !os.IsNotExist(err) {
		t.Fatalf("zero-init lifecycle created project-local .kv: %v", err)
	}

	if err := os.WriteFile(proposal, []byte("## Why\n\nExpanded automatic memory scope\n"), 0600); err != nil {
		t.Fatal(err)
	}
	after, err := adapter.Envelope(Event{NativeSessionID: "opencode-first", CWD: workspace, ChangeID: "automatic-memory", OperationID: "propose-1", OperationKind: "propose", Name: hook.EventAfterPropose})
	if err != nil {
		t.Fatal(err)
	}
	transformed, err := service.Handle(ctx, after)
	if err != nil {
		t.Fatal(err)
	}
	if transformed.Transformation == nil || transformed.Transformation.ID == "" {
		t.Fatalf("verified transformation = %#v", transformed)
	}

	later, err := adapter.Envelope(Event{NativeSessionID: "opencode-later", CWD: workspace, ChangeID: "automatic-memory", OperationID: "apply-1", OperationKind: "apply", Name: hook.EventBeforeApply})
	if err != nil {
		t.Fatal(err)
	}
	retrieved, err := service.Handle(ctx, later)
	if err != nil {
		t.Fatal(err)
	}
	if len(retrieved.Memory) == 0 || retrieved.Memory[0].Source.RevisionID == "" {
		t.Fatalf("later-session memory = %#v", retrieved.Memory)
	}

	archive, err := adapter.Envelope(Event{NativeSessionID: "opencode-later", CWD: workspace, ChangeID: "automatic-memory", OperationID: "archive-1", OperationKind: "archive", Name: hook.EventAfterArchive})
	if err != nil {
		t.Fatal(err)
	}
	archived, err := service.Handle(ctx, archive)
	if err != nil {
		t.Fatal(err)
	}
	if archived.Degraded || len(archived.Memory) == 0 {
		t.Fatalf("archive consolidation = %#v", archived)
	}
}

func installOpenSpecStub(t *testing.T, workspace, proposal string) {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "bin")
	if err := os.MkdirAll(bin, 0700); err != nil {
		t.Fatal(err)
	}
	status := fmt.Sprintf(`{"artifactPaths":{"proposal":{"existingOutputPaths":[%q]}}}`, proposal)
	instructions := fmt.Sprintf(`{"contextFiles":{"proposal":[%q]}}`, proposal)
	script := fmt.Sprintf("#!/bin/sh\ncase \"$1\" in\nstatus) printf '%%s' '%s' ;;\ninstructions) printf '%%s' '%s' ;;\nesac\n", status, instructions)
	path := filepath.Join(bin, "openspec")
	if err := os.WriteFile(path, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
}
