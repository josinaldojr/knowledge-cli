package fsutil

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
)

func TestPathHelpers(t *testing.T) {
	tmpDir, err := ioutil.TempDir("", "fsutil-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Test Exists & IsDir on directory
	if !Exists(tmpDir) {
		t.Errorf("expected temp dir to exist")
	}
	if !IsDir(tmpDir) {
		t.Errorf("expected temp dir to be a directory")
	}
	if IsFile(tmpDir) {
		t.Errorf("temp dir should not be a file")
	}

	// Test WriteFile and IsFile on file
	filePath := filepath.Join(tmpDir, "test.txt")
	err = WriteFile(filePath, []byte("hello"), 0644)
	if err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	if !Exists(filePath) {
		t.Errorf("expected file to exist")
	}
	if IsDir(filePath) {
		t.Errorf("file should not be a directory")
	}
	if !IsFile(filePath) {
		t.Errorf("expected path to be a file")
	}

	// Test ResolveAbs
	abs, err := ResolveAbs(filePath)
	if err != nil {
		t.Errorf("ResolveAbs failed: %v", err)
	}
	if !filepath.IsAbs(abs) {
		t.Errorf("resolved path is not absolute: %s", abs)
	}

	// Test ResolveRel
	rel := ResolveRel(tmpDir, filePath)
	if rel != "test.txt" {
		t.Errorf("expected relative path to be 'test.txt', got '%s'", rel)
	}
}

func TestWriteWithBackup(t *testing.T) {
	tmpDir, err := ioutil.TempDir("", "backup-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	filePath := filepath.Join(tmpDir, "document.txt")

	// First write (no backup since it doesn't exist)
	backedUp, err := WriteWithBackup(filePath, []byte("version 1"), 0644)
	if err != nil {
		t.Fatalf("WriteWithBackup 1 failed: %v", err)
	}
	if backedUp {
		t.Errorf("expected no backup on first write")
	}

	// Second write (should trigger backup)
	backedUp, err = WriteWithBackup(filePath, []byte("version 2"), 0644)
	if err != nil {
		t.Fatalf("WriteWithBackup 2 failed: %v", err)
	}
	if !backedUp {
		t.Errorf("expected backup to be created on second write")
	}

	// Verify new content
	content, err := ioutil.ReadFile(filePath)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}
	if string(content) != "version 2" {
		t.Errorf("expected content to be 'version 2', got '%s'", string(content))
	}

	// Verify backup content
	bakContent, err := ioutil.ReadFile(filePath + ".bak")
	if err != nil {
		t.Fatalf("failed to read backup file: %v", err)
	}
	if string(bakContent) != "version 1" {
		t.Errorf("expected backup content to be 'version 1', got '%s'", string(bakContent))
	}
}
