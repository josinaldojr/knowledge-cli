package context

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"kv/internal/workspace"
)

// BuildContext copies the context pack of the specified task to .opencode/context.md
func BuildContext(workspaceDir, workflowSlug, taskID string) error {
	// Resolve paths
	workflowDir := filepath.Join(workspaceDir, workspace.ConfigDirName, "workflows", workflowSlug)
	contextPackPath := filepath.Join(workflowDir, "context", taskID+".context.md")

	if _, err := os.Stat(contextPackPath); os.IsNotExist(err) {
		return fmt.Errorf("context pack file %s not found. Please run 'kv task enrich' first", contextPackPath)
	}

	data, err := ioutil.ReadFile(contextPackPath)
	if err != nil {
		return fmt.Errorf("failed to read context pack file: %v", err)
	}

	opencodeDir := filepath.Join(workspaceDir, ".opencode")
	if err := os.MkdirAll(opencodeDir, 0755); err != nil {
		return fmt.Errorf("failed to create .opencode directory: %v", err)
	}

	targetPath := filepath.Join(opencodeDir, "context.md")
	if err := ioutil.WriteFile(targetPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write .opencode/context.md file: %v", err)
	}

	return nil
}
