package workflow

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
)

func TestNewWorkflow(t *testing.T) {
	tmpDir, err := ioutil.TempDir("", "workflow-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	slug := "test-slug"
	err = NewWorkflow(tmpDir, slug)
	if err != nil {
		t.Fatalf("NewWorkflow failed: %v", err)
	}

	workflowDir := filepath.Join(tmpDir, ".kv", "workflows", slug)

	// Check folders exist
	subdirs := []string{"tasks", "context", "reviews", "memory"}
	for _, subdir := range subdirs {
		dirPath := filepath.Join(workflowDir, subdir)
		info, err := os.Stat(dirPath)
		if err != nil {
			t.Errorf("expected directory %s to exist, got error: %v", subdir, err)
		} else if !info.IsDir() {
			t.Errorf("expected %s to be a directory", subdir)
		}
	}

	// Check files exist
	files := []string{"idea.md", "prd.md", "techspec.md", "tasks/001-setup.md"}
	for _, file := range files {
		filePath := filepath.Join(workflowDir, file)
		info, err := os.Stat(filePath)
		if err != nil {
			t.Errorf("expected file %s to exist, got error: %v", file, err)
		} else if info.IsDir() {
			t.Errorf("expected %s to be a file", file)
		}
	}

	// Test invalid slug
	err = NewWorkflow(tmpDir, "invalid/slug")
	if err == nil {
		t.Errorf("expected invalid slug error, got nil")
	}
}
