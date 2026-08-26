// Package query exposes evidence-linked read models without leaking storage details.
package query

import (
	"context"
	"encoding/json"
	"fmt"

	"kv/internal/memory"
	"kv/internal/store"
)

type DiagnosticRepository interface {
	ListDiagnostics(context.Context, string) ([]store.DiagnosticRecord, error)
}
type ReadRepository interface {
	GetWorkspaceStatus(context.Context, string) (store.WorkspaceStatus, error)
	CurrentSessions(context.Context, string) ([]store.SessionRecord, error)
	ChangeHistory(context.Context, string) ([]store.ChangeRecord, error)
	TransformationGenealogy(context.Context, string) ([]store.TransformationRecord, error)
}

type Service struct {
	retriever   *memory.Retriever
	diagnostics DiagnosticRepository
	read        ReadRepository
}

func New(retriever *memory.Retriever, diagnostics DiagnosticRepository) *Service {
	service := &Service{retriever: retriever, diagnostics: diagnostics}
	if read, ok := diagnostics.(ReadRepository); ok {
		service.read = read
	}
	return service
}

// Query implements the MCP read boundary. Every request requires a workspace
// ID; records are resolved by repository joins constrained to that workspace.
func (s *Service) Query(ctx context.Context, name string, raw json.RawMessage) (any, error) {
	if s.read == nil {
		return nil, fmt.Errorf("query read model unavailable")
	}
	var request struct {
		WorkspaceID string `json:"workspace_id"`
		ChangeKey   string `json:"change_key"`
		Text        string `json:"text"`
		URI         string `json:"uri"`
		Path        string `json:"path"`
	}
	if err := json.Unmarshal(raw, &request); err != nil {
		return nil, fmt.Errorf("invalid query request")
	}
	if request.WorkspaceID == "" {
		return nil, fmt.Errorf("workspace_id is required")
	}
	// Query callers cannot supply filesystem paths. Snapshot and artifact paths
	// are resolved only from OpenSpec manifests captured by the server.
	if request.Path != "" {
		return nil, fmt.Errorf("filesystem paths are not accepted by query APIs")
	}
	switch name {
	case "kv_workspace_status":
		return s.read.GetWorkspaceStatus(ctx, request.WorkspaceID)
	case "kv_memory_search":
		return s.Memories(ctx, memory.Query{WorkspaceID: request.WorkspaceID, ChangeKey: request.ChangeKey, Text: request.Text})
	case "kv_adapter_diagnostics":
		return s.Diagnostics(ctx, request.WorkspaceID)
	case "resource":
		switch {
		case contains(request.URI, "/session"):
			return s.read.CurrentSessions(ctx, request.WorkspaceID)
		case contains(request.URI, "/changes"):
			return s.read.ChangeHistory(ctx, request.WorkspaceID)
		case contains(request.URI, "/transformations"):
			return s.read.TransformationGenealogy(ctx, request.WorkspaceID)
		case contains(request.URI, "/diagnostics"):
			return s.Diagnostics(ctx, request.WorkspaceID)
		}
	}
	return nil, fmt.Errorf("unsupported query")
}
func contains(value, part string) bool {
	for index := 0; index+len(part) <= len(value); index++ {
		if value[index:index+len(part)] == part {
			return true
		}
	}
	return false
}
func (s *Service) Memories(ctx context.Context, request memory.Query) ([]memory.Result, error) {
	return s.retriever.Retrieve(ctx, request)
}
func (s *Service) Diagnostics(ctx context.Context, workspaceID string) ([]store.DiagnosticRecord, error) {
	return s.diagnostics.ListDiagnostics(ctx, workspaceID)
}
