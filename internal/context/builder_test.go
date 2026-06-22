package context

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
)

func TestBuildContext(t *testing.T) {
	tmpDir, err := ioutil.TempDir("", "context-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	workflowSlug := "test-flow"
	taskID := "001-setup"

	workflowDir := filepath.Join(tmpDir, ".kv", "workflows", workflowSlug)
	contextDir := filepath.Join(workflowDir, "context")
	err = os.MkdirAll(contextDir, 0755)
	if err != nil {
		t.Fatalf("failed to create context dir: %v", err)
	}

	contextPackPath := filepath.Join(contextDir, taskID+".context.md")
	expectedContent := "my context pack content"
	err = ioutil.WriteFile(contextPackPath, []byte(expectedContent), 0644)
	if err != nil {
		t.Fatalf("failed to write mock context pack: %v", err)
	}

	err = BuildContext(tmpDir, workflowSlug, taskID)
	if err != nil {
		t.Fatalf("BuildContext failed: %v", err)
	}

	opencodeFile := filepath.Join(tmpDir, ".opencode", "context.md")
	if _, err := os.Stat(opencodeFile); os.IsNotExist(err) {
		t.Fatalf(".opencode/context.md not created")
	}

	data, err := ioutil.ReadFile(opencodeFile)
	if err != nil {
		t.Fatalf("failed to read .opencode/context.md: %v", err)
	}

	if string(data) != expectedContent {
		t.Errorf("expected '%s', got '%s'", expectedContent, string(data))
	}
}
