package hook

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestEnvelopeFixtures(t *testing.T) {
	cases := []struct {
		name  string
		valid bool
	}{
		{"valid-opencode.json", true},
		{"backward-compatible-v1.json", true},
		{"duplicate-opencode.json", true},
		{"valid-claude.json", true},
		{"claude-session-start.json", true},
		{"claude-session-end.json", true},
		{"valid-codex-fallback.json", true},
		{"invalid-missing-session.json", false},
		{"invalid-event.json", false},
		{"unsupported-version.json", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join("testdata", tc.name))
			if err != nil {
				t.Fatal(err)
			}
			var envelope HookEnvelope
			if err := json.Unmarshal(data, &envelope); err != nil {
				t.Fatal(err)
			}
			err = envelope.Validate(time.Date(2026, 8, 17, 13, 0, 0, 0, time.UTC))
			if tc.valid && err != nil {
				t.Fatalf("Validate() error = %v", err)
			}
			if !tc.valid && err == nil {
				t.Fatal("Validate() error = nil")
			}
		})
	}
}

func TestDuplicateEnvelopeHasStableIdempotencyIdentity(t *testing.T) {
	envelope := HookEnvelope{SchemaVersion: SchemaVersion, EventID: "event-1", IdempotencyKey: "provider/session/event-1", Event: EventSessionObserved, OccurredAt: time.Now(), Provider: ProviderIdentity{Kind: ProviderOpenCode, NativeSessionID: "native-1"}, Workspace: WorkspaceIdentity{CWD: t.TempDir()}}
	if err := envelope.Validate(time.Now()); err != nil {
		t.Fatal(err)
	}
	if envelope.IdempotencyKey != "provider/session/event-1" {
		t.Fatal("idempotency key changed")
	}
}

func TestProviderSummaryRequiresVersionedStructuredClaims(t *testing.T) {
	valid := ProviderSummary{Version: 1, Claims: []ProviderClaim{{Kind: "requirement", ID: "session", Status: "added", Statement: "Added session."}}}
	if err := valid.Validate(); err != nil {
		t.Fatalf("valid summary: %v", err)
	}
	if err := (ProviderSummary{Version: 1, Claims: []ProviderClaim{{Kind: "requirement", ID: "", Status: "added"}}}).Validate(); err == nil {
		t.Fatal("summary with an unreferenced claim was accepted")
	}
	if err := (ProviderSummary{Version: 2}).Validate(); err == nil {
		t.Fatal("unsupported summary version was accepted")
	}
}
