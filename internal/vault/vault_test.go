package vault

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"kv/internal/fsutil"
	"kv/internal/workspace"
)

func createFakeVault(t *testing.T, path string) {
	err := fsutil.EnsureDir(path)
	if err != nil {
		t.Fatalf("failed to create fake vault dir %s: %v", path, err)
	}
	marker := filepath.Join(path, ".kv-vault")
	err = ioutil.WriteFile(marker, []byte(`{"type": "knowledge-vault"}`), 0644)
	if err != nil {
		t.Fatalf("failed to write fake vault marker at %s: %v", marker, err)
	}
}

func TestFindVaultPrecedence(t *testing.T) {
	tmpDir, err := ioutil.TempDir("", "vault-find-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	absTmp, err := fsutil.ResolveAbs(tmpDir)
	if err != nil {
		t.Fatalf("failed to resolve absolute tmpDir: %v", err)
	}

	// Setup structures
	workspaceDir := filepath.Join(absTmp, "my-workspace")
	vaultDir := filepath.Join(absTmp, "my-vault")

	err = fsutil.EnsureDir(workspaceDir)
	if err != nil {
		t.Fatalf("failed to create workspace dir: %v", err)
	}

	// 1. Test not found
	_, err = FindVault(workspaceDir)
	if err == nil {
		t.Errorf("expected FindVault to return error when no vault exists")
	}

	// 2. Test Direct child named knowledge-vault
	childVault := filepath.Join(workspaceDir, "knowledge-vault")
	createFakeVault(t, childVault)
	res, err := FindVault(workspaceDir)
	if err != nil {
		t.Fatalf("FindVault child failed: %v", err)
	}
	if res.Path != childVault || res.Source != "child" {
		t.Errorf("expected child vault path '%s', got '%s' (source '%s')", childVault, res.Path, res.Source)
	}
	os.RemoveAll(childVault) // cleanup for next tests

	// 3. Test Current directory itself may be the vault
	createFakeVault(t, workspaceDir)
	res, err = FindVault(workspaceDir)
	if err != nil {
		t.Fatalf("FindVault self failed: %v", err)
	}
	if res.Path != workspaceDir || res.Source != "self" {
		t.Errorf("expected self vault path '%s', got '%s' (source '%s')", workspaceDir, res.Path, res.Source)
	}
	os.Remove(filepath.Join(workspaceDir, ".kv-vault")) // remove marker

	// 4. Test Workspace marker (.knowledge-vault)
	createFakeVault(t, vaultDir)
	// Write marker pointing from workspaceDir to vaultDir
	_, err = workspace.WriteMarker(workspaceDir, "../my-vault")
	if err != nil {
		t.Fatalf("failed to write marker: %v", err)
	}
	res, err = FindVault(workspaceDir)
	if err != nil {
		t.Fatalf("FindVault marker failed: %v", err)
	}
	if res.Path != vaultDir || res.Source != "marker" {
		t.Errorf("expected marker vault path '%s', got '%s' (source '%s')", vaultDir, res.Path, res.Source)
	}

	// 5. Test Environment variable wins
	envVault := filepath.Join(absTmp, "env-vault")
	createFakeVault(t, envVault)

	os.Setenv("KNOWLEDGE_VAULT_PATH", envVault)
	defer os.Unsetenv("KNOWLEDGE_VAULT_PATH")

	res, err = FindVault(workspaceDir)
	if err != nil {
		t.Fatalf("FindVault env failed: %v", err)
	}
	if res.Path != envVault || res.Source != "env" {
		t.Errorf("expected env vault path '%s', got '%s' (source '%s')", envVault, res.Path, res.Source)
	}
}

func TestEnsureVaultMarker(t *testing.T) {
	tmpDir, err := ioutil.TempDir("", "vault-ensure-marker-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Directory exists but has no marker
	err = EnsureVaultMarker(tmpDir)
	if err != nil {
		t.Fatalf("EnsureVaultMarker failed: %v", err)
	}

	// Verify .kv-vault exists
	markerPath := filepath.Join(tmpDir, ".kv-vault")
	if !fsutil.IsFile(markerPath) {
		t.Errorf(".kv-vault marker file was not created")
	}

	// Call again, should skip
	err = EnsureVaultMarker(tmpDir)
	if err != nil {
		t.Fatalf("EnsureVaultMarker on existing marker failed: %v", err)
	}
}

