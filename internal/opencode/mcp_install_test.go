package opencode

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallMCPMergesIdempotentlyAndUninstallsOnlyKV(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "opencode.json")
	if err := os.WriteFile(path, []byte(`{"mcp":{"user":{"type":"local","command":["user"]}},"theme":"dark"}`), 0600); err != nil {
		t.Fatal(err)
	}
	installed, err := InstallMCP(dir, "kv-bin")
	if err != nil || !installed {
		t.Fatalf("installed = %t, err = %v", installed, err)
	}
	installed, err = InstallMCP(dir, "kv-bin")
	if err != nil || installed {
		t.Fatalf("repeat installed = %t, err = %v", installed, err)
	}
	contents, _ := os.ReadFile(path)
	if !strings.Contains(string(contents), `"user"`) || !strings.Contains(string(contents), `"kv-bin"`) {
		t.Fatalf("config = %s", contents)
	}
	removed, err := UninstallMCP(dir)
	if err != nil || !removed {
		t.Fatalf("removed = %t, err = %v", removed, err)
	}
	contents, _ = os.ReadFile(path)
	if !strings.Contains(string(contents), `"user"`) || strings.Contains(string(contents), `"kv-bin"`) {
		t.Fatalf("config = %s", contents)
	}
}
