// Package claudecode translates Claude Code lifecycle hooks to KV's neutral contract.
package claudecode

import (
	"fmt"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"
	"kv/internal/hook"
)

// Adapter implements the L2 lifecycle contract from the capability matrix:
// CLAUDE_SESSION_ID is forwarded unchanged when supplied, and a stable
// correlation identity is retained for hook environments that omit it.
type Adapter struct {
	mu           sync.Mutex
	correlations map[string]string
}

func New() *Adapter { return &Adapter{correlations: map[string]string{}} }

type Event struct {
	ClaudeSessionID, CWD, ChangeID, OperationID, OperationKind string
	Name                                                       hook.EventName
}

func (a *Adapter) Envelope(event Event) (hook.HookEnvelope, error) {
	if !filepath.IsAbs(event.CWD) {
		return hook.HookEnvelope{}, fmt.Errorf("Claude Code cwd must be absolute")
	}
	provider := hook.ProviderIdentity{Kind: hook.ProviderClaudeCode, NativeSessionID: event.ClaudeSessionID}
	if provider.NativeSessionID == "" {
		a.mu.Lock()
		provider.CorrelationID = a.correlations[event.CWD]
		if provider.CorrelationID == "" {
			provider.CorrelationID = "claude-code-" + uuid.NewString()
			a.correlations[event.CWD] = provider.CorrelationID
		}
		a.mu.Unlock()
	}
	id := uuid.NewString()
	return hook.HookEnvelope{SchemaVersion: hook.SchemaVersion, EventID: id, IdempotencyKey: "claude-code/" + provider.NativeSessionID + provider.CorrelationID + "/" + id, Event: event.Name, OccurredAt: time.Now().UTC(), Provider: provider, Workspace: hook.WorkspaceIdentity{CWD: event.CWD}, Subject: hook.HookSubject{ChangeID: event.ChangeID, Operation: hook.Operation{ID: event.OperationID, Kind: event.OperationKind}}}, nil
}
