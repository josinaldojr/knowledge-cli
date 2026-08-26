// Package opencode translates supported OpenCode signals to KV's neutral hook contract.
package opencode

import (
	"fmt"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"
	"kv/internal/hook"
)

// Adapter implements the L1 capability-matrix fallback: use a native session
// ID when OpenCode provides one, otherwise retain one correlation ID per CWD.
type Adapter struct {
	mu           sync.Mutex
	correlations map[string]string
}

func New() *Adapter { return &Adapter{correlations: map[string]string{}} }

type Event struct {
	NativeSessionID, CWD, ChangeID, OperationID, OperationKind string
	Name                                                       hook.EventName
}

func (a *Adapter) Envelope(event Event) (hook.HookEnvelope, error) {
	if !filepath.IsAbs(event.CWD) {
		return hook.HookEnvelope{}, fmt.Errorf("OpenCode cwd must be absolute")
	}
	provider := hook.ProviderIdentity{Kind: hook.ProviderOpenCode, NativeSessionID: event.NativeSessionID}
	if provider.NativeSessionID == "" {
		a.mu.Lock()
		provider.CorrelationID = a.correlations[event.CWD]
		if provider.CorrelationID == "" {
			provider.CorrelationID = "opencode-" + uuid.NewString()
			a.correlations[event.CWD] = provider.CorrelationID
		}
		a.mu.Unlock()
	}
	id := uuid.NewString()
	return hook.HookEnvelope{SchemaVersion: hook.SchemaVersion, EventID: id, IdempotencyKey: "opencode/" + provider.NativeSessionID + provider.CorrelationID + "/" + id, Event: event.Name, OccurredAt: time.Now().UTC(), Provider: provider, Workspace: hook.WorkspaceIdentity{CWD: event.CWD}, Subject: hook.HookSubject{ChangeID: event.ChangeID, Operation: hook.Operation{ID: event.OperationID, Kind: event.OperationKind}}}, nil
}
