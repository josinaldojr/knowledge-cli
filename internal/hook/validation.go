package hook

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

const maxProviderSummaryBytes = 16 * 1024

// Validate checks contract invariants before an envelope reaches storage.
func (e HookEnvelope) Validate(now time.Time) error {
	if e.SchemaVersion != SchemaVersion {
		return fmt.Errorf("unsupported hook schema version %d", e.SchemaVersion)
	}
	if strings.TrimSpace(e.EventID) == "" || strings.TrimSpace(e.IdempotencyKey) == "" {
		return fmt.Errorf("event_id and idempotency_key are required")
	}
	if !validEvent(e.Event) {
		return fmt.Errorf("unsupported hook event %q", e.Event)
	}
	if e.OccurredAt.IsZero() || e.OccurredAt.After(now.Add(5*time.Minute)) {
		return fmt.Errorf("occurred_at must be present and not more than five minutes in the future")
	}
	if !validProvider(e.Provider.Kind) {
		return fmt.Errorf("unsupported provider kind %q", e.Provider.Kind)
	}
	if e.Provider.NativeSessionID == "" && e.Provider.CorrelationID == "" {
		return fmt.Errorf("provider requires native_session_id or correlation_id")
	}
	if e.Workspace.CWD == "" || !filepath.IsAbs(e.Workspace.CWD) {
		return fmt.Errorf("workspace.cwd must be an absolute path")
	}
	if err := e.Subject.ProviderSummary.Validate(); err != nil {
		return err
	}
	if strings.HasPrefix(string(e.Event), "openspec.") {
		if e.Subject.ChangeID == "" || e.Subject.Operation.ID == "" || e.Subject.Operation.Kind == "" {
			return fmt.Errorf("openspec hooks require change_id, operation.id, and operation.kind")
		}
	}
	return nil
}

func (s ProviderSummary) Validate() error {
	if s.Version == 0 && len(s.Claims) == 0 {
		return nil
	}
	if s.Version != 1 {
		return fmt.Errorf("unsupported provider_summary version %d", s.Version)
	}
	encoded, err := json.Marshal(s)
	if err != nil || len(encoded) > maxProviderSummaryBytes {
		return fmt.Errorf("provider_summary exceeds %d bytes", maxProviderSummaryBytes)
	}
	for _, claim := range s.Claims {
		if strings.TrimSpace(claim.Kind) == "" || strings.TrimSpace(claim.ID) == "" || !validClaimStatus(claim.Status) || strings.ContainsRune(claim.Statement, '\x00') {
			return fmt.Errorf("provider_summary contains an invalid claim")
		}
	}
	return nil
}

func validClaimStatus(status string) bool {
	switch status {
	case "added", "modified", "removed", "renamed", "unchanged":
		return true
	default:
		return false
	}
}

func validProvider(kind ProviderKind) bool {
	return kind == ProviderOpenCode || kind == ProviderClaudeCode || kind == ProviderCodex || kind == ProviderOther
}

func validEvent(event EventName) bool {
	switch event {
	case EventSessionObserved, EventSessionFinished, EventSessionFailed,
		EventBeforePropose, EventAfterPropose, EventBeforeUpdate, EventAfterUpdate,
		EventBeforeApply, EventAfterApply, EventBeforeArchive, EventAfterArchive:
		return true
	default:
		return false
	}
}
