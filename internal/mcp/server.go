// Package mcp implements KV's provider-neutral MCP stdio transport.
package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"os"
	"sync"

	"kv/internal/lifecycle"
	"kv/internal/memory"
	"kv/internal/query"
	"kv/internal/snapshot"
	"kv/internal/store"
)

type Server struct {
	handler func(context.Context, Request) (any, *ResponseError)
}

const MaxRequestBytes = 1 << 20

type Request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
}
type Response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  any             `json:"result,omitempty"`
	Error   *ResponseError  `json:"error,omitempty"`
}
type ResponseError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

func New(handler func(Request) (any, *ResponseError)) *Server {
	if handler == nil {
		return &Server{}
	}
	return NewWithContext(func(_ context.Context, request Request) (any, *ResponseError) { return handler(request) })
}
func NewWithContext(handler func(context.Context, Request) (any, *ResponseError)) *Server {
	return &Server{handler: handler}
}

func (s *Server) Serve(in io.Reader, out io.Writer) error {
	scanner := bufio.NewScanner(in)
	buffer := make([]byte, 64*1024)
	scanner.Buffer(buffer, MaxRequestBytes)
	encoder := json.NewEncoder(out)
	var writes sync.Mutex
	var pending sync.Map
	var workers sync.WaitGroup
	for scanner.Scan() {
		var request Request
		if err := json.Unmarshal(scanner.Bytes(), &request); err != nil {
			if err := encoder.Encode(Response{JSONRPC: "2.0", Error: &ResponseError{Code: -32700, Message: "parse error"}}); err != nil {
				return err
			}
			continue
		}
		if request.Method == "notifications/cancelled" {
			var cancellation struct {
				RequestID json.RawMessage `json:"requestId"`
			}
			_ = json.Unmarshal(request.Params, &cancellation)
			if value, found := pending.Load(string(cancellation.RequestID)); found {
				value.(context.CancelFunc)()
			}
			continue
		}
		ctx, cancel := context.WithCancel(context.Background())
		key := string(request.ID)
		if len(request.ID) != 0 {
			pending.Store(key, cancel)
		}
		workers.Add(1)
		go func(request Request, ctx context.Context, cancel context.CancelFunc, key string) {
			defer workers.Done()
			if len(request.ID) != 0 {
				defer pending.Delete(key)
			}
			defer cancel()
			result, responseError := s.handle(ctx, request)
			if len(request.ID) != 0 {
				writes.Lock()
				_ = encoder.Encode(Response{JSONRPC: "2.0", ID: request.ID, Result: result, Error: responseError})
				writes.Unlock()
			}
		}(request, ctx, cancel, key)
	}
	workers.Wait()
	return scanner.Err()
}

func (s *Server) handle(ctx context.Context, request Request) (any, *ResponseError) {
	if request.JSONRPC != "2.0" {
		return nil, &ResponseError{Code: -32600, Message: "invalid request"}
	}
	if request.Method == "initialize" {
		return map[string]any{"protocolVersion": "2024-11-05", "serverInfo": map[string]string{"name": "kv", "version": "0.1.0"}, "capabilities": map[string]any{"tools": map[string]any{}}}, nil
	}
	if s.handler == nil {
		return nil, &ResponseError{Code: -32601, Message: "method not found"}
	}
	return s.handler(ctx, request)
}

func ServeStdio() error {
	paths, err := store.ResolvePaths()
	if err != nil {
		return err
	}
	database, err := store.Open(context.Background(), paths.DatabasePath)
	if err != nil {
		return err
	}
	defer database.Close()
	if err := database.Migrate(context.Background(), paths.DatabasePath); err != nil {
		return err
	}
	repository := store.NewRepository(database)
	snapshots, err := snapshot.Open(paths.SnapshotDir, 1<<20, nil)
	if err != nil {
		return err
	}
	service := lifecycle.NewHookApplicationService(lifecycle.NewSessionService(repository), repository, snapshot.NewCapturer(snapshots, repository)).WithMemoryRetriever(memory.NewRetriever(repository)).WithArchiveConsolidator(memory.NewConsolidator(repository))
	queries := query.New(memory.NewRetriever(repository), repository)
	return NewKVWithQueries(service, queries).Serve(os.Stdin, os.Stdout)
}
