package workspace

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
)

func TestMarkerReadWrite(t *testing.T) {
	tmpDir, err := ioutil.TempDir("", "workspace-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Write marker
	backedUp, err := WriteMarker(tmpDir, "my-vault-path")
	if err != nil {
		t.Fatalf("WriteMarker failed: %v", err)
	}
	if backedUp {
		t.Errorf("expected backedUp to be false on first write")
	}

	// Read marker
	val, err := ReadMarker(tmpDir)
	if err != nil {
		t.Fatalf("ReadMarker failed: %v", err)
	}
	if val != "my-vault-path" {
		t.Errorf("expected read marker value 'my-vault-path', got '%s'", val)
	}

	// Overwrite marker (should backup)
	backedUp, err = WriteMarker(tmpDir, "new-vault-path")
	if err != nil {
		t.Fatalf("WriteMarker 2 failed: %v", err)
	}
	if !backedUp {
		t.Errorf("expected backedUp to be true on overwrite")
	}

	// Read new value
	val, err = ReadMarker(tmpDir)
	if err != nil {
		t.Fatalf("ReadMarker failed after overwrite: %v", err)
	}
	if val != "new-vault-path" {
		t.Errorf("expected read marker value 'new-vault-path', got '%s'", val)
	}

	// Read backup value
	bakPath := filepath.Join(tmpDir, MarkerFilename+".bak")
	bakData, err := ioutil.ReadFile(bakPath)
	if err != nil {
		t.Fatalf("failed to read backup file: %v", err)
	}
	if string(bakData) != "my-vault-path" {
		t.Errorf("expected backup content 'my-vault-path', got '%s'", string(bakData))
	}
}

func TestResolvePathSafe(t *testing.T) {
	base := "/Users/test/workspace"
	
	// absolute value
	val, err := ResolvePathSafe(base, "/absolute/vault")
	if err != nil {
		t.Fatalf("failed to resolve absolute path: %v", err)
	}
	expectedAbs := filepath.Clean("/absolute/vault")
	if val != expectedAbs {
		t.Errorf("expected '%s', got '%s'", expectedAbs, val)
	}

	// relative value
	val, err = ResolvePathSafe(base, "vault-dir")
	if err != nil {
		t.Fatalf("failed to resolve relative path: %v", err)
	}
	expectedRel := filepath.Clean("/Users/test/workspace/vault-dir")
	if val != expectedRel {
		t.Errorf("expected '%s', got '%s'", expectedRel, val)
	}
}
