package runner

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
)

func TestRunTask(t *testing.T) {
	tmpDir, err := ioutil.TempDir("", "runner-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	workflowSlug := "test-flow"
	taskID := "001-setup"

	// Create context folder and context pack
	workflowDir := filepath.Join(tmpDir, ".kv", "workflows", workflowSlug)
	contextDir := filepath.Join(workflowDir, "context")
	err = os.MkdirAll(contextDir, 0755)
	if err != nil {
		t.Fatalf("failed to create context dir: %v", err)
	}

	contextPackPath := filepath.Join(contextDir, taskID+".context.md")
	err = ioutil.WriteFile(contextPackPath, []byte("my context pack"), 0644)
	if err != nil {
		t.Fatalf("failed to write context pack: %v", err)
	}

	// Run with unsupported runner
	err = RunTask(tmpDir, workflowSlug, taskID, "unsupported")
	if err == nil {
		t.Errorf("expected error for unsupported runner, got nil")
	}

	// Run with opencode
	err = RunTask(tmpDir, workflowSlug, taskID, "opencode")
	if err != nil {
		t.Fatalf("RunTask opencode failed: %v", err)
	}

	// Verify .opencode/context.md is generated
	opencodeFile := filepath.Join(tmpDir, ".opencode", "context.md")
	if _, err := os.Stat(opencodeFile); os.IsNotExist(err) {
		t.Fatalf(".opencode/context.md not created")
	}
}
