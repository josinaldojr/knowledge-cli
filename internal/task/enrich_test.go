package task

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kv/internal/vault"
	"kv/internal/workspace"
)

func TestEnrichTask(t *testing.T) {
	tmpDir, err := ioutil.TempDir("", "enrich-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a mock vault
	vaultDir := filepath.Join(tmpDir, "my-vault")
	err = vault.Init(vaultDir)
	if err != nil {
		t.Fatalf("failed to init mock vault: %v", err)
	}

	// Create workspace config
	cfg := &workspace.Config{
		VaultPath: "my-vault",
	}
	err = workspace.SaveConfig(tmpDir, cfg)
	if err != nil {
		t.Fatalf("failed to save config: %v", err)
	}

	// Create a workflow
	workflowSlug := "auth-flow"
	workflowDir := filepath.Join(tmpDir, ".kv", "workflows", workflowSlug)
	err = os.MkdirAll(filepath.Join(workflowDir, "tasks"), 0755)
	if err != nil {
		t.Fatalf("failed to create tasks dir: %v", err)
	}
	err = os.MkdirAll(filepath.Join(workflowDir, "context"), 0755)
	if err != nil {
		t.Fatalf("failed to create context dir: %v", err)
	}

	// Write techspec.md with Validation Plan
	techspecContent := `# Techspec

## Validation Plan
Run go test to verify changes.
`
	err = ioutil.WriteFile(filepath.Join(workflowDir, "techspec.md"), []byte(techspecContent), 0644)
	if err != nil {
		t.Fatalf("failed to write techspec: %v", err)
	}

	// Write a source file in repo
	err = ioutil.WriteFile(filepath.Join(tmpDir, "main.go"), []byte("package main"), 0644)
	if err != nil {
		t.Fatalf("failed to write source file: %v", err)
	}

	// Write a task file
	taskID := "001-setup"
	taskContent := `---
id: 001-setup
title: "Setup env"
status: todo
type: chore
complexity: low
agent/runner: opencode
sources:
  - main.go
acceptanceCriteria:
  - "Ok"
---

This is my description.
`
	err = ioutil.WriteFile(filepath.Join(workflowDir, "tasks", taskID+".md"), []byte(taskContent), 0644)
	if err != nil {
		t.Fatalf("failed to write task file: %v", err)
	}

	// Run enrichment
	err = EnrichTask(tmpDir, workflowSlug, taskID)
	if err != nil {
		t.Fatalf("EnrichTask failed: %v", err)
	}

	// Check that the task status is updated to in-progress
	updatedTask, err := ParseTask(filepath.Join(workflowDir, "tasks", taskID+".md"))
	if err != nil {
		t.Fatalf("ParseTask failed: %v", err)
	}
	if updatedTask.Frontmatter.Status != "in-progress" {
		t.Errorf("expected status 'in-progress', got '%s'", updatedTask.Frontmatter.Status)
	}

	// Check that context file exists
	contextPackPath := filepath.Join(workflowDir, "context", taskID+".context.md")
	if _, err := os.Stat(contextPackPath); os.IsNotExist(err) {
		t.Fatalf("context pack file not created")
	}

	contextData, err := ioutil.ReadFile(contextPackPath)
	if err != nil {
		t.Fatalf("failed to read context pack file: %v", err)
	}

	contextStr := string(contextData)

	// Validate sections
	expectedSubstrings := []string{
		"## 1. Task Definition",
		"This is my description.",
		"## 2. Source Code Context",
		"package main",
		"## 3. Vault Knowledge",
		"## 4. Architectural Decisions",
		"## 5. Validation Plan & Runbooks",
		"Run go test to verify changes.",
	}

	for _, substr := range expectedSubstrings {
		if !strings.Contains(contextStr, substr) {
			t.Errorf("expected context pack to contain '%s', but it did not.\nFull content:\n%s", substr, contextStr)
		}
	}
}
