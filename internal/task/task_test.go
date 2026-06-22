package task

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
)

func TestParseTaskContent(t *testing.T) {
	raw := `---
id: 001-setup
title: "Setup env"
status: todo
type: chore
complexity: low
dependencies:
  - 000-prev
agent/runner: opencode
sources:
  - main.go
acceptanceCriteria:
  - "Ok"
---

# My Task

Description here.
`
	task, err := ParseTaskContent(raw)
	if err != nil {
		t.Fatalf("ParseTaskContent failed: %v", err)
	}

	if task.Frontmatter.ID != "001-setup" {
		t.Errorf("expected ID '001-setup', got '%s'", task.Frontmatter.ID)
	}
	if task.Frontmatter.Title != "Setup env" {
		t.Errorf("expected Title 'Setup env', got '%s'", task.Frontmatter.Title)
	}
	if task.Frontmatter.AgentRunner != "opencode" {
		t.Errorf("expected agent/runner 'opencode', got '%s'", task.Frontmatter.AgentRunner)
	}
	if len(task.Frontmatter.Sources) != 1 || task.Frontmatter.Sources[0] != "main.go" {
		t.Errorf("unexpected sources: %v", task.Frontmatter.Sources)
	}
	if task.Content != "# My Task\n\nDescription here." {
		t.Errorf("unexpected Content: '%s'", task.Content)
	}
}

func TestSaveTask(t *testing.T) {
	tmpDir, err := ioutil.TempDir("", "task-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	task := &Task{
		Frontmatter: TaskFrontmatter{
			ID:                 "test-task",
			Title:              "My Test Task",
			Status:             "in-progress",
			Type:               "feature",
			Complexity:         "medium",
			AgentRunner:        "opencode",
			Sources:            []string{"src/file.go"},
			AcceptanceCriteria: []string{"Works"},
		},
		Content: "Task Body",
	}

	filePath := filepath.Join(tmpDir, "task.md")
	err = SaveTask(filePath, task)
	if err != nil {
		t.Fatalf("SaveTask failed: %v", err)
	}

	// Parse it back
	loaded, err := ParseTask(filePath)
	if err != nil {
		t.Fatalf("ParseTask failed: %v", err)
	}

	if loaded.Frontmatter.ID != "test-task" {
		t.Errorf("expected test-task, got %s", loaded.Frontmatter.ID)
	}
	if loaded.Frontmatter.AgentRunner != "opencode" {
		t.Errorf("expected opencode, got %s", loaded.Frontmatter.AgentRunner)
	}
	if loaded.Content != "Task Body" {
		t.Errorf("expected Content 'Task Body', got '%s'", loaded.Content)
	}
}
