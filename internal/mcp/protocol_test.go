package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"sync/atomic"
	"testing"
)

func TestServerCancelsInFlightRequestAndShutsDownCleanly(t *testing.T) {
	var cancelled atomic.Bool
	server := NewWithContext(func(ctx context.Context, _ Request) (any, *ResponseError) {
		<-ctx.Done()
		cancelled.Store(true)
		return nil, nil
	})
	input := bytes.NewBufferString(`{"jsonrpc":"2.0","id":1,"method":"work","params":{}}` + "\n" + `{"jsonrpc":"2.0","method":"notifications/cancelled","params":{"requestId":1}}` + "\n")
	var output bytes.Buffer
	if err := server.Serve(input, &output); err != nil {
		t.Fatal(err)
	}
	if !cancelled.Load() {
		t.Fatal("request was not cancelled")
	}
	var response Response
	if err := json.Unmarshal(output.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
}
