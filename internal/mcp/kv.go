package mcp

import (
	"context"
	"encoding/json"

	"kv/internal/hook"
)

type HookHandler interface {
	Handle(context.Context, hook.HookEnvelope) (hook.HookResult, error)
}

// QueryHandler is provider-neutral: query implementations decide how to
// resolve the evidence-backed read model without exposing storage internals.
type QueryHandler interface {
	Query(context.Context, string, json.RawMessage) (any, error)
}

const maxToolArgumentsBytes = 256 << 10

func NewKV(handler HookHandler) *Server {
	return NewKVWithQueries(handler, nil)
}

func NewKVWithQueries(handler HookHandler, queries QueryHandler) *Server {
	return NewWithContext(func(ctx context.Context, request Request) (any, *ResponseError) {
		switch request.Method {
		case "tools/list":
			return map[string]any{"tools": []map[string]any{{"name": "kv_hook", "description": "Submit a versioned KV lifecycle hook", "inputSchema": map[string]any{"type": "object"}}, {"name": "kv_memory_search", "description": "Search evidence-linked engineering memory", "inputSchema": map[string]any{"type": "object"}}, {"name": "kv_workspace_status", "description": "Read workspace and session status", "inputSchema": map[string]any{"type": "object"}}, {"name": "kv_adapter_diagnostics", "description": "Read adapter diagnostics", "inputSchema": map[string]any{"type": "object"}}}}, nil
		case "resources/list":
			return map[string]any{"resources": []map[string]any{{"uri": "kv://workspace/{workspace_id}/session", "name": "Current provider session"}, {"uri": "kv://workspace/{workspace_id}/changes", "name": "OpenSpec change history"}, {"uri": "kv://workspace/{workspace_id}/transformations", "name": "Transformation genealogy"}, {"uri": "kv://workspace/{workspace_id}/diagnostics", "name": "Adapter diagnostics"}}}, nil
		case "resources/read":
			if queries == nil {
				return nil, &ResponseError{Code: -32601, Message: "query service unavailable"}
			}
			result, err := queries.Query(ctx, "resource", request.Params)
			if err != nil {
				return nil, responseForError(err)
			}
			return result, nil
		case "tools/call":
			var call struct {
				Name      string          `json:"name"`
				Arguments json.RawMessage `json:"arguments"`
			}
			if err := json.Unmarshal(request.Params, &call); err != nil {
				return nil, &ResponseError{Code: -32602, Message: "invalid tool parameters"}
			}
			if len(call.Arguments) > maxToolArgumentsBytes {
				return nil, &ResponseError{Code: -32602, Message: "tool arguments exceed size limit", Data: map[string]any{"kind": ErrorValidation, "retryable": false}}
			}
			if call.Name != "kv_hook" {
				if queries == nil {
					return nil, &ResponseError{Code: -32601, Message: "tool not found"}
				}
				result, err := queries.Query(ctx, call.Name, call.Arguments)
				if err != nil {
					return nil, responseForError(err)
				}
				return result, nil
			}
			var envelope hook.HookEnvelope
			if err := json.Unmarshal(call.Arguments, &envelope); err != nil {
				return nil, &ResponseError{Code: -32602, Message: "invalid hook envelope"}
			}
			result, err := handler.Handle(ctx, envelope)
			if err != nil {
				return nil, responseForError(err)
			}
			return result, nil
		default:
			return nil, &ResponseError{Code: -32601, Message: "method not found"}
		}
	})
}
