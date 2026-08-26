package portable

import (
	"strings"
	"testing"
	"time"
)

func TestDocumentValidateRejectsUnsafeOrUntraceableMemory(t *testing.T) {
	now := time.Now().UTC()
	document := Document{
		SchemaVersion: SchemaVersion,
		ExportedAt:    now,
		Workspace:     Workspace{Identity: "git:example/repo"},
		Sessions:      []Session{{ID: "session-1", Provider: "opencode", Status: "completed", StartedAt: now, LastSeenAt: now}},
		Memories:      []Memory{{ID: "memory-1", Kind: "decision", Status: "current", Summary: "use SQLite", CreatedAt: now, Sources: []Source{{ChangeKey: "add-store", ArtifactPath: "specs/store.md", RevisionHash: "abc"}}}},
	}
	if err := document.Validate(); err != nil {
		t.Fatal(err)
	}
	document.Memories[0].Sources[0].ArtifactPath = "/private/workspace/spec.md"
	if err := document.Validate(); err == nil || !strings.Contains(err.Error(), "invalid evidence") {
		t.Fatalf("Validate() error = %v", err)
	}
}
