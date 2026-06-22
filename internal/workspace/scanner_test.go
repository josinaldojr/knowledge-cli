package workspace

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
)

func TestRepoScanner(t *testing.T) {
	tmpDir, err := ioutil.TempDir("", "scanner-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create test files
	filesToCreate := map[string]string{
		"file1.go":                 "content1",
		"pkg/file2.go":             "content2",
		".git/config":              "should ignore git",
		"node_modules/dep/main.js": "should ignore node_modules",
		".kv/config.yaml":          "should ignore .kv",
	}

	for relPath, content := range filesToCreate {
		fullPath := filepath.Join(tmpDir, relPath)
		err := os.MkdirAll(filepath.Dir(fullPath), 0755)
		if err != nil {
			t.Fatalf("failed to create dir: %v", err)
		}
		err = ioutil.WriteFile(fullPath, []byte(content), 0644)
		if err != nil {
			t.Fatalf("failed to write file: %v", err)
		}
	}

	files, err := ScanRepo(tmpDir)
	if err != nil {
		t.Fatalf("ScanRepo failed: %v", err)
	}

	// We expect only file1.go and pkg/file2.go
	expected := map[string]bool{
		"file1.go":     true,
		"pkg/file2.go": true,
	}

	if len(files) != len(expected) {
		t.Errorf("expected %d files, got %d: %v", len(expected), len(files), files)
	}

	for _, f := range files {
		if !expected[f] {
			t.Errorf("unexpected file returned: %s", f)
		}
	}

	// Read repo file test
	content, err := ReadRepoFile(tmpDir, "pkg/file2.go")
	if err != nil {
		t.Fatalf("ReadRepoFile failed: %v", err)
	}
	if content != "content2" {
		t.Errorf("expected 'content2', got '%s'", content)
	}

	// Traversal check
	_, err = ReadRepoFile(tmpDir, "../outside.go")
	if err == nil {
		t.Errorf("expected error for path traversal attempt, got nil")
	}
}
