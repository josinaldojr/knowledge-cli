package opencode

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCheckMCPReportsConfigurationAndAdapterCapabilities(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "opencode.json"), []byte(`{"mcp":{"kv":{"type":"local"}}}`), 0600); err != nil {
		t.Fatal(err)
	}
	report := CheckMCP(dir, "go")
	if !report.Binary || !report.MCPConfigured || !report.LifecycleAdapter || !report.SpoolRecovery {
		t.Fatalf("report = %#v", report)
	}
}
