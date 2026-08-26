// Package hook contains the provider-neutral lifecycle contract used by KV.
package hook

import "time"

const SchemaVersion = 1

type ProviderKind string

const (
	ProviderOpenCode   ProviderKind = "opencode"
	ProviderClaudeCode ProviderKind = "claude-code"
	ProviderCodex      ProviderKind = "codex"
	ProviderOther      ProviderKind = "other"
)

type EventName string

const (
	EventSessionObserved EventName = "provider.session_observed"
	EventSessionFinished EventName = "provider.session_finished"
	EventSessionFailed   EventName = "provider.session_failed"
	EventBeforePropose   EventName = "openspec.before.propose"
	EventAfterPropose    EventName = "openspec.after.propose"
	EventBeforeUpdate    EventName = "openspec.before.artifact_update"
	EventAfterUpdate     EventName = "openspec.after.artifact_update"
	EventBeforeApply     EventName = "openspec.before.apply"
	EventAfterApply      EventName = "openspec.after.apply"
	EventBeforeArchive   EventName = "openspec.before.archive"
	EventAfterArchive    EventName = "openspec.after.archive"
)

type ProviderIdentity struct {
	Kind            ProviderKind `json:"kind"`
	NativeSessionID string       `json:"native_session_id,omitempty"`
	CorrelationID   string       `json:"correlation_id,omitempty"`
}

type WorkspaceIdentity struct {
	LogicalID  string `json:"logical_id,omitempty"`
	InstanceID string `json:"instance_id,omitempty"`
	CWD        string `json:"cwd"`
}

type ProviderSession struct {
	ID         string            `json:"id"`
	Provider   ProviderIdentity  `json:"provider"`
	Workspace  WorkspaceIdentity `json:"workspace"`
	Status     SessionStatus     `json:"status"`
	StartedAt  time.Time         `json:"started_at"`
	LastSeenAt time.Time         `json:"last_seen_at"`
}

type SessionStatus string

const (
	SessionActive    SessionStatus = "active"
	SessionCompleted SessionStatus = "completed"
	SessionFailed    SessionStatus = "failed"
	SessionStale     SessionStatus = "stale"
)

type Operation struct {
	ID          string   `json:"id,omitempty"`
	Kind        string   `json:"kind,omitempty"`
	ArtifactIDs []string `json:"artifact_ids,omitempty"`
}

type HookSubject struct {
	ChangeID        string          `json:"change_id,omitempty"`
	Operation       Operation       `json:"operation,omitempty"`
	ProviderSummary ProviderSummary `json:"provider_summary,omitempty"`
	NoChange        bool            `json:"no_change,omitempty"`
}

// ProviderSummary contains claims which can be verified against an artifact
// delta. Free-form provider prose is retained only within an identified claim.
type ProviderSummary struct {
	Version int             `json:"version"`
	Claims  []ProviderClaim `json:"claims,omitempty"`
}

type ProviderClaim struct {
	Kind      string `json:"kind"`
	ID        string `json:"id"`
	Status    string `json:"status"`
	Statement string `json:"statement,omitempty"`
}

// HookEnvelope is the only provider-adapter input accepted by the domain.
type HookEnvelope struct {
	SchemaVersion  int               `json:"schema_version"`
	EventID        string            `json:"event_id"`
	IdempotencyKey string            `json:"idempotency_key"`
	Event          EventName         `json:"event"`
	OccurredAt     time.Time         `json:"occurred_at"`
	Provider       ProviderIdentity  `json:"provider"`
	Workspace      WorkspaceIdentity `json:"workspace"`
	Subject        HookSubject       `json:"subject,omitempty"`
}

type ArtifactRevision struct {
	ID            string    `json:"id"`
	ArtifactID    string    `json:"artifact_id"`
	CanonicalPath string    `json:"canonical_path"`
	ContentHash   string    `json:"content_hash"`
	CapturedAt    time.Time `json:"captured_at"`
}

type Transformation struct {
	ID          string `json:"id"`
	OperationID string `json:"operation_id"`
	Summary     string `json:"summary"`
}

type MemorySource struct {
	ChangeID   string `json:"change_id"`
	ArtifactID string `json:"artifact_id"`
	RevisionID string `json:"revision_id"`
}

// MemoryContext is derived assistance, never an authoritative OpenSpec change.
// Every returned record carries its evidence path and immutable revision ID.
type MemoryContext struct {
	ID      string       `json:"id"`
	Kind    string       `json:"kind"`
	Status  string       `json:"status"`
	Summary string       `json:"summary"`
	Score   int          `json:"score"`
	Current bool         `json:"current"`
	Source  MemorySource `json:"source"`
}

type Diagnostic struct {
	Code     string `json:"code"`
	Severity string `json:"severity"`
	Message  string `json:"message"`
}

type HookResult struct {
	SessionID      string             `json:"session_id,omitempty"`
	OperationID    string             `json:"operation_id,omitempty"`
	Status         string             `json:"status"`
	Transformation *Transformation    `json:"transformation,omitempty"`
	Revisions      []ArtifactRevision `json:"revisions,omitempty"`
	Diagnostics    []Diagnostic       `json:"diagnostics,omitempty"`
	Memory         []MemoryContext    `json:"memory"`
	Degraded       bool               `json:"degraded,omitempty"`
}
