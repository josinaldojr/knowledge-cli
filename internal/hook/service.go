package hook

import "context"

// EventRepository owns durable idempotent hook event storage.
type EventRepository interface {
	FindByIdempotencyKey(ctx context.Context, key string) (HookResult, bool, error)
	StoreProcessed(ctx context.Context, envelope HookEnvelope, result HookResult) error
}

// SessionRepository owns provider-neutral session persistence.
type SessionRepository interface {
	Ensure(ctx context.Context, provider ProviderIdentity, workspace WorkspaceIdentity) (ProviderSession, error)
	UpdateActivity(ctx context.Context, sessionID string, status SessionStatus) error
}

// OperationRepository persists before/after correlations without exposing a transport.
type OperationRepository interface {
	Save(ctx context.Context, operation Operation) error
	Find(ctx context.Context, operationID string) (Operation, bool, error)
}

// HookService is the application boundary used by MCP and provider adapters.
type HookService interface {
	Handle(ctx context.Context, envelope HookEnvelope) (HookResult, error)
}
