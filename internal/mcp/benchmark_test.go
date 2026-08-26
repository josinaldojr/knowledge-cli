package mcp

import (
	"bytes"
	"testing"
)

func BenchmarkMCPStartup(b *testing.B) {
	request := []byte("{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"initialize\",\"params\":{}}\n")
	b.ReportAllocs()
	for b.Loop() {
		var output bytes.Buffer
		if err := New(nil).Serve(bytes.NewReader(request), &output); err != nil {
			b.Fatal(err)
		}
	}
}
