package workspace

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
)

func TestConfigLoadSave(t *testing.T) {
	tmpDir, err := ioutil.TempDir("", "config-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cfg := &Config{
		VaultPath: "my-vault-path",
	}

	err = SaveConfig(tmpDir, cfg)
	if err != nil {
		t.Fatalf("SaveConfig failed: %v", err)
	}

	// Find config
	wsDir, err := FindWorkspaceDir(filepath.Join(tmpDir, "some/sub/dir"))
	if err != nil {
		t.Fatalf("FindWorkspaceDir failed: %v", err)
	}
	if wsDir != tmpDir {
		t.Errorf("expected workspace directory '%s', got '%s'", tmpDir, wsDir)
	}

	// Load config
	loaded, err := LoadConfig(wsDir)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if loaded.VaultPath != "my-vault-path" {
		t.Errorf("expected vault_path 'my-vault-path', got '%s'", loaded.VaultPath)
	}
}
