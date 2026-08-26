// Package codex translates Codex MCP and wrapper signals to KV hooks.
package codex

import (
	"fmt"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"
	"kv/internal/hook"
)

// Adapter implements the L1 Codex capability-matrix fallback. Codex does not
// provide a supported native session ID, so one correlation ID is retained per
// observed working directory for the adapter process lifetime.
type Adapter struct {
	mu           sync.Mutex
	correlations map[string]string
}

func New() *Adapter { return &Adapter{correlations: map[string]string{}} }

type Event struct {
	CWD, ChangeID, OperationID, OperationKind string
	Name                                      hook.EventName
}

func (a *Adapter) Envelope(event Event) (hook.HookEnvelope, error) {
	if !filepath.IsAbs(event.CWD) {
		return hook.HookEnvelope{}, fmt.Errorf("Codex cwd must be absolute")
	}
	a.mu.Lock()
	correlationID := a.correlations[event.CWD]
	if correlationID == "" {
		correlationID = "codex-" + uuid.NewString()
		a.correlations[event.CWD] = correlationID
	}
	a.mu.Unlock()
	id := uuid.NewString()
	return hook.HookEnvelope{SchemaVersion: hook.SchemaVersion, EventID: id, IdempotencyKey: "codex/" + correlationID + "/" + id, Event: event.Name, OccurredAt: time.Now().UTC(), Provider: hook.ProviderIdentity{Kind: hook.ProviderCodex, CorrelationID: correlationID}, Workspace: hook.WorkspaceIdentity{CWD: event.CWD}, Subject: hook.HookSubject{ChangeID: event.ChangeID, Operation: hook.Operation{ID: event.OperationID, Kind: event.OperationKind}}}, nil
}
