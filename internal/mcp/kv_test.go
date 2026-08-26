package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"
	"time"

	"kv/internal/hook"
)

type testHookHandler struct{}

func (testHookHandler) Handle(context.Context, hook.HookEnvelope) (hook.HookResult, error) {
	return hook.HookResult{SessionID: "session", Status: "processed"}, nil
}

type testQueryHandler struct{}

func (testQueryHandler) Query(_ context.Context, name string, _ json.RawMessage) (any, error) {
	return map[string]string{"query": name}, nil
}

func TestKVHookToolMapsEnvelopeToHandler(t *testing.T) {
	envelope := hook.HookEnvelope{SchemaVersion: 1, EventID: "event", IdempotencyKey: "key", Event: hook.EventSessionObserved, OccurredAt: time.Now(), Provider: hook.ProviderIdentity{Kind: hook.ProviderOpenCode, NativeSessionID: "native"}, Workspace: hook.WorkspaceIdentity{CWD: "/workspace"}}
	arguments, err := json.Marshal(envelope)
	if err != nil {
		t.Fatal(err)
	}
	input := bytes.NewBufferString(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"kv_hook","arguments":` + string(arguments) + `}}` + "\n")
	var output bytes.Buffer
	if err := NewKV(testHookHandler{}).Serve(input, &output); err != nil {
		t.Fatal(err)
	}
	var response Response
	if err := json.Unmarshal(output.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Error != nil || response.Result == nil {
		t.Fatalf("response = %#v", response)
	}
}

func TestKVExposesMemoryToolsAndResources(t *testing.T) {
	input := bytes.NewBufferString(`{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}` + "\n" + `{"jsonrpc":"2.0","id":2,"method":"resources/list","params":{}}` + "\n" + `{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"kv_memory_search","arguments":{}}}` + "\n")
	var output bytes.Buffer
	if err := NewKVWithQueries(testHookHandler{}, testQueryHandler{}).Serve(input, &output); err != nil {
		t.Fatal(err)
	}
	var responses []Response
	for _, line := range bytes.Split(bytes.TrimSpace(output.Bytes()), []byte("\n")) {
		var response Response
		if err := json.Unmarshal(line, &response); err != nil {
			t.Fatal(err)
		}
		responses = append(responses, response)
	}
	if len(responses) != 3 || responses[0].Error != nil || responses[1].Error != nil || responses[2].Error != nil {
		t.Fatalf("responses = %#v", responses)
	}
}

func TestKVRejectsOversizedToolArguments(t *testing.T) {
	arguments := bytes.Repeat([]byte("x"), maxToolArgumentsBytes)
	input := bytes.NewBufferString(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"kv_memory_search","arguments":"` + string(arguments) + `"}}` + "\n")
	var output bytes.Buffer
	if err := NewKVWithQueries(testHookHandler{}, testQueryHandler{}).Serve(input, &output); err != nil {
		t.Fatal(err)
	}
	var response Response
	if err := json.Unmarshal(output.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Error == nil || response.Error.Message != "tool arguments exceed size limit" {
		t.Fatalf("response = %#v", response)
	}
}
