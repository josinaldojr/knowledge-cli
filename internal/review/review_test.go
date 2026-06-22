package review

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kv/internal/task"
)

func TestReviewTaskWorkflow(t *testing.T) {
	tmpDir, err := ioutil.TempDir("", "review-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	workflowSlug := "auth-flow"
	taskID := "001-setup"

	workflowDir := filepath.Join(tmpDir, ".kv", "workflows", workflowSlug)
	err = os.MkdirAll(filepath.Join(workflowDir, "tasks"), 0755)
	if err != nil {
		t.Fatalf("failed to create tasks dir: %v", err)
	}
	err = os.MkdirAll(filepath.Join(workflowDir, "reviews"), 0755)
	if err != nil {
		t.Fatalf("failed to create reviews dir: %v", err)
	}
	err = os.MkdirAll(filepath.Join(workflowDir, "memory"), 0755)
	if err != nil {
		t.Fatalf("failed to create memory dir: %v", err)
	}

	taskFile := filepath.Join(workflowDir, "tasks", taskID+".md")
	taskContent := `---
id: 001-setup
title: "Setup task"
status: todo
agent/runner: opencode
acceptanceCriteria:
  - "Criterion 1"
  - "Criterion 2"
---

Body of task.
`
	err = ioutil.WriteFile(taskFile, []byte(taskContent), 0644)
	if err != nil {
		t.Fatalf("failed to write task file: %v", err)
	}

	// 1. Run first review: should create template files
	err = ReviewTask(tmpDir, workflowSlug, taskID)
	if err != nil {
		t.Fatalf("First ReviewTask failed: %v", err)
	}

	reviewFile := filepath.Join(workflowDir, "reviews", taskID+".review.md")
	memoryFile := filepath.Join(workflowDir, "memory", taskID+"-memory.md")

	if _, err := os.Stat(reviewFile); os.IsNotExist(err) {
		t.Errorf("review file not generated")
	}
	if _, err := os.Stat(memoryFile); os.IsNotExist(err) {
		t.Errorf("memory file not generated")
	}

	// 2. Try second review: should remain pending because checklist is empty
	err = ReviewTask(tmpDir, workflowSlug, taskID)
	if err != nil {
		t.Fatalf("Second ReviewTask failed: %v", err)
	}

	parsedTask, err := task.ParseTask(taskFile)
	if err != nil {
		t.Fatalf("failed to parse task: %v", err)
	}
	if parsedTask.Frontmatter.Status == "done" {
		t.Errorf("expected status to not be done yet")
	}

	// 3. Mark criteria as checked in review file
	reviewData, err := ioutil.ReadFile(reviewFile)
	if err != nil {
		t.Fatalf("failed to read review file: %v", err)
	}

	updatedReview := strings.ReplaceAll(string(reviewData), "- [ ]", "- [x]")
	err = ioutil.WriteFile(reviewFile, []byte(updatedReview), 0644)
	if err != nil {
		t.Fatalf("failed to write updated review: %v", err)
	}

	// 4. Run third review: should promote task status to done
	err = ReviewTask(tmpDir, workflowSlug, taskID)
	if err != nil {
		t.Fatalf("Third ReviewTask failed: %v", err)
	}

	parsedTask, err = task.ParseTask(taskFile)
	if err != nil {
		t.Fatalf("failed to parse task second time: %v", err)
	}
	if parsedTask.Frontmatter.Status != "done" {
		t.Errorf("expected task status to be promoted to done, got '%s'", parsedTask.Frontmatter.Status)
	}
}
