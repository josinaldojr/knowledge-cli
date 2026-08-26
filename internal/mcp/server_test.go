package mcp

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestServerInitializesOverStdio(t *testing.T) {
	input := bytes.NewBufferString(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}` + "\n")
	var output bytes.Buffer
	if err := New(nil).Serve(input, &output); err != nil {
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
