// Package lifecycle coordinates provider-neutral session lifecycle actions.
package lifecycle

import (
	"context"
	"fmt"
	"time"

	"kv/internal/discovery"
	"kv/internal/hook"
	"kv/internal/store"
)

type SessionService struct{ repository *store.Repository }

func NewSessionService(repository *store.Repository) *SessionService {
	return &SessionService{repository: repository}
}

func (s *SessionService) Reconcile(ctx context.Context, timeout time.Duration, now time.Time) (int64, int64, error) {
	if timeout <= 0 {
		return 0, 0, fmt.Errorf("reconciliation timeout must be positive")
	}
	return s.repository.ReconcileStale(ctx, now.Add(-timeout))
}

// EnsureSession lazily creates exactly one session for a provider session and
// a local workspace instance. No `.kv` directory is read or written.
func (s *SessionService) EnsureSession(ctx context.Context, cwd string, provider hook.ProviderIdentity) (hook.ProviderSession, error) {
	if provider.NativeSessionID == "" && provider.CorrelationID == "" {
		return hook.ProviderSession{}, fmt.Errorf("provider requires native_session_id or correlation_id")
	}
	identity, err := discovery.DeriveIdentity(cwd)
	if err != nil {
		return hook.ProviderSession{}, err
	}
	workspaceID, err := s.repository.EnsureWorkspace(ctx, identity.LogicalID)
	if err != nil {
		return hook.ProviderSession{}, err
	}
	instanceID, err := s.repository.EnsureWorkspaceInstance(ctx, workspaceID, identity.Git.CanonicalCWD)
	if err != nil {
		return hook.ProviderSession{}, err
	}
	session, err := s.repository.EnsureProviderSession(ctx, instanceID, provider)
	if err != nil {
		return hook.ProviderSession{}, err
	}
	session.Workspace.LogicalID = workspaceID
	session.Workspace.InstanceID = instanceID
	session.Workspace.CWD = identity.Git.CanonicalCWD
	return session, nil
}

func (s *SessionService) Transition(ctx context.Context, sessionID string, next hook.SessionStatus) error {
	current, err := s.repository.SessionStatus(ctx, sessionID)
	if err != nil {
		return err
	}
	if !canTransition(current, next) {
		return fmt.Errorf("invalid session transition %s -> %s", current, next)
	}
	return s.repository.UpdateSession(ctx, sessionID, next)
}

func canTransition(current, next hook.SessionStatus) bool {
	if current == next {
		return true
	}
	switch current {
	case hook.SessionActive:
		return next == hook.SessionCompleted || next == hook.SessionFailed || next == hook.SessionStale
	case hook.SessionStale:
		return next == hook.SessionActive
	default:
		return false
	}
}
