package workflow

import (
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"kv/internal/runner"
)

func TestRunWorkflow(t *testing.T) {
	tmpDir, err := ioutil.TempDir("", "workflow-run-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	slug := "test-flow"

	// Create workspace directories and config
	kvDir := filepath.Join(tmpDir, ".kv")
	err = os.MkdirAll(kvDir, 0755)
	if err != nil {
		t.Fatalf("failed to create .kv dir: %v", err)
	}

	configYaml := `vault_path: "./vault"
`
	err = ioutil.WriteFile(filepath.Join(kvDir, "config.yaml"), []byte(configYaml), 0644)
	if err != nil {
		t.Fatalf("failed to write config.yaml: %v", err)
	}

	workspaceYaml := `workspace:
  name: "test-workspace"
  apps: []
`
	err = ioutil.WriteFile(filepath.Join(tmpDir, "kv-workspace.yaml"), []byte(workspaceYaml), 0644)
	if err != nil {
		t.Fatalf("failed to write kv-workspace.yaml: %v", err)
	}

	// Make sure vault dir exists
	err = os.MkdirAll(filepath.Join(tmpDir, "vault"), 0755)
	if err != nil {
		t.Fatalf("failed to create vault dir: %v", err)
	}

	// Mock runCommandOverride for workflow package
	oldWorkflowOverride := runCommandOverride
	defer func() { runCommandOverride = oldWorkflowOverride }()

	// Mock RunCommandOverride for runner package
	oldRunnerOverride := runner.RunCommandOverride
	defer func() { runner.RunCommandOverride = oldRunnerOverride }()

	invocations := 0

	mockCommander := func(name string, arg ...string) *exec.Cmd {
		invocations++
		prompt := arg[1]

		workflowDir := filepath.Join(tmpDir, ".kv", "workflows", slug)

		if strings.Contains(prompt, "IDEA phase") {
			// Phase 1: Write idea.md
			ideaPath := filepath.Join(workflowDir, "idea.md")
			_ = ioutil.WriteFile(ideaPath, []byte("# Idea\nProcessed"), 0644)
		} else if strings.Contains(prompt, "PRD phase") {
			// Phase 2: Write prd.md
			prdPath := filepath.Join(workflowDir, "prd.md")
			_ = ioutil.WriteFile(prdPath, []byte("# PRD\n- [ ] Req 1"), 0644)
		} else if strings.Contains(prompt, "SPECS phase") {
			// Phase 3: Write techspec.md and tasks/002-implement.md
			techspecPath := filepath.Join(workflowDir, "techspec.md")
			_ = ioutil.WriteFile(techspecPath, []byte("# Techspec\n## Validation Plan\nrun tests"), 0644)

			taskContent := `---
id: 002-implement
title: "Implement feature"
status: todo
type: feature
complexity: low
agent/runner: opencode
acceptanceCriteria:
  - "Criteria 1"
---

Implement the feature logic.
`
			tasksDir := filepath.Join(workflowDir, "tasks")
			_ = os.MkdirAll(tasksDir, 0755)
			_ = ioutil.WriteFile(filepath.Join(tasksDir, "002-implement.md"), []byte(taskContent), 0644)

		} else if strings.Contains(prompt, "Please read the task context file") {
			// Phase 4: Implementation of 002-implement
		} else if strings.Contains(prompt, "REVIEW phase") {
			// Phase 5: Review
			reviewPath := filepath.Join(workflowDir, "reviews", "002-implement.review.md")
			data, err := ioutil.ReadFile(reviewPath)
			if err == nil {
				content := strings.ReplaceAll(string(data), "- [ ]", "- [x]")
				_ = ioutil.WriteFile(reviewPath, []byte(content), 0644)
			}
		} else if strings.Contains(prompt, "ADJUSTMENTS phase") {
			// Phase 6: Adjustments
		} else if strings.Contains(prompt, "MEMORIZE phase") {
			// Phase 7: Memorize
			memoryPath := filepath.Join(workflowDir, "memory", "002-implement-memory.md")
			_ = ioutil.WriteFile(memoryPath, []byte("# Memory\nLearned"), 0644)
		}

		return exec.Command("true")
	}

	runCommandOverride = mockCommander
	runner.RunCommandOverride = mockCommander

	// Execute RunWorkflow
	err = RunWorkflow(tmpDir, slug, "implement payment logic")
	if err != nil {
		t.Fatalf("RunWorkflow failed: %v", err)
	}

	// Verify all documents were generated
	workflowDir := filepath.Join(tmpDir, ".kv", "workflows", slug)

	filesToCheck := []string{
		"idea.md",
		"prd.md",
		"techspec.md",
		"tasks/002-implement.md",
		"reviews/002-implement.review.md",
		"memory/002-implement-memory.md",
	}

	for _, file := range filesToCheck {
		path := filepath.Join(workflowDir, file)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("expected file %s to be created, but it doesn't exist", file)
		}
	}

	if invocations < 6 {
		t.Errorf("expected at least 6 invocations of opencode, got %d", invocations)
	}
}
