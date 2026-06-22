package review

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"kv/internal/task"
	"kv/internal/workspace"
)

type ReviewFrontmatter struct {
	TaskID     string `yaml:"task_id"`
	Status     string `yaml:"status"`
	ReviewedAt string `yaml:"reviewed_at"`
}

type Review struct {
	Frontmatter ReviewFrontmatter
	Content     string
}

// ReviewTask generates or processes a task review.
func ReviewTask(workspaceDir, workflowSlug, taskID string) error {
	workflowDir := filepath.Join(workspaceDir, workspace.ConfigDirName, "workflows", workflowSlug)
	taskFile := filepath.Join(workflowDir, "tasks", taskID+".md")
	if _, err := os.Stat(taskFile); os.IsNotExist(err) {
		return fmt.Errorf("task file %s not found", taskFile)
	}

	parsedTask, err := task.ParseTask(taskFile)
	if err != nil {
		return fmt.Errorf("failed to parse task: %v", err)
	}

	reviewFile := filepath.Join(workflowDir, "reviews", taskID+".review.md")

	if _, err := os.Stat(reviewFile); os.IsNotExist(err) {
		// 1. Generate review file
		return generateReview(workspaceDir, workflowSlug, taskID, parsedTask, reviewFile)
	}

	// 2. Process existing review
	return processExistingReview(taskFile, parsedTask, reviewFile)
}

func generateReview(workspaceDir, workflowSlug, taskID string, t *task.Task, reviewFile string) error {
	fmt.Printf("Generating review for task %s in workflow %s...\n", taskID, workflowSlug)

	// Fetch git diff for sources
	var gitDiff string
	if len(t.Frontmatter.Sources) > 0 {
		args := append([]string{"diff", "HEAD", "--"}, t.Frontmatter.Sources...)
		cmd := exec.Command("git", args...)
		cmd.Dir = workspaceDir
		var out bytes.Buffer
		cmd.Stdout = &out
		if err := cmd.Run(); err == nil {
			gitDiff = out.String()
		} else {
			// Try without HEAD or fallback
			cmd2 := exec.Command("git", append([]string{"diff", "--"}, t.Frontmatter.Sources...)...)
			cmd2.Dir = workspaceDir
			var out2 bytes.Buffer
			cmd2.Stdout = &out2
			if err := cmd2.Run(); err == nil {
				gitDiff = out2.String()
			} else {
				gitDiff = "*(No git diff available or files not modified)*"
			}
		}
	} else {
		gitDiff = "*(No source files defined for this task)*"
	}

	if gitDiff == "" {
		gitDiff = "*(No changes detected in source files)*"
	}

	// Build review content
	var sb strings.Builder
	sb.WriteString("---\n")
	sb.WriteString(fmt.Sprintf("task_id: %s\n", taskID))
	sb.WriteString("status: pending-review\n")
	sb.WriteString(fmt.Sprintf("reviewed_at: %s\n", time.Now().Format(time.RFC3339)))
	sb.WriteString("---\n\n")

	sb.WriteString(fmt.Sprintf("# Review: %s\n\n", t.Frontmatter.Title))

	sb.WriteString("## 1. Acceptance Criteria Verification\n\n")
	if len(t.Frontmatter.AcceptanceCriteria) == 0 {
		sb.WriteString("- [ ] Verify task implementation\n\n")
	} else {
		for _, ac := range t.Frontmatter.AcceptanceCriteria {
			sb.WriteString(fmt.Sprintf("- [ ] %s\n", ac))
		}
		sb.WriteString("\n")
	}

	sb.WriteString("## 2. Git Diff Summary\n\n")
	sb.WriteString("```diff\n")
	sb.WriteString(gitDiff)
	sb.WriteString("\n```\n\n")

	sb.WriteString("## 3. Review Notes & Decisions\n\n")
	sb.WriteString("- Add decisions made during implementation.\n\n")

	sb.WriteString("## 4. Learnings & Memory Candidates\n\n")
	sb.WriteString("- Describe reusable patterns or findings.\n")

	// Write review file
	if err := ioutil.WriteFile(reviewFile, []byte(sb.String()), 0644); err != nil {
		return fmt.Errorf("failed to write review file: %v", err)
	}

	// Generate template memory file
	workflowDir := filepath.Join(workspaceDir, workspace.ConfigDirName, "workflows", workflowSlug)
	memoryFile := filepath.Join(workflowDir, "memory", taskID+"-memory.md")
	if _, err := os.Stat(memoryFile); os.IsNotExist(err) {
		memContent := fmt.Sprintf(`---
title: "Learning from task %s"
tags: []
status: draft
---

# Reusable Learning from %s

Write down what was learned during this task.
`, t.Frontmatter.Title, taskID)

		_ = ioutil.WriteFile(memoryFile, []byte(memContent), 0644)
	}

	fmt.Printf("Created review template: %s\n", reviewFile)
	fmt.Printf("Created memory template: %s\n", memoryFile)
	fmt.Println("Please verify acceptance criteria checklist, add notes, and run 'kv task review' again.")
	return nil
}

func processExistingReview(taskFile string, t *task.Task, reviewFile string) error {
	data, err := ioutil.ReadFile(reviewFile)
	if err != nil {
		return fmt.Errorf("failed to read review file: %v", err)
	}

	raw := string(data)
	normalized := strings.ReplaceAll(raw, "\r\n", "\n")

	if !strings.HasPrefix(normalized, "---\n") {
		return fmt.Errorf("invalid review file format: missing frontmatter")
	}

	parts := strings.SplitN(normalized, "---\n", 3)
	if len(parts) < 3 {
		return fmt.Errorf("invalid review file format: incomplete frontmatter")
	}

	var fm ReviewFrontmatter
	if err := yaml.Unmarshal([]byte(parts[1]), &fm); err != nil {
		return fmt.Errorf("failed to parse review frontmatter: %v", err)
	}

	body := parts[2]
	lines := strings.Split(body, "\n")

	totalCheckboxes := 0
	checkedCheckboxes := 0

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "- [") || strings.HasPrefix(trimmed, "* [") {
			// Find position of ]
			idx := strings.Index(trimmed, "]")
			if idx > 2 && idx < 6 {
				status := strings.ToLower(strings.TrimSpace(trimmed[3:idx]))
				totalCheckboxes++
				if status == "x" {
					checkedCheckboxes++
				}
			}
		}
	}

	fmt.Printf("Review Checklist: %d of %d criteria verified.\n", checkedCheckboxes, totalCheckboxes)

	if totalCheckboxes > 0 && checkedCheckboxes == totalCheckboxes {
		// All criteria verified!
		fmt.Println("All acceptance criteria verified! Promoting task to DONE...")

		// Update review status to approved
		fm.Status = "approved"
		fm.ReviewedAt = time.Now().Format(time.RFC3339)

		newFmBytes, err := yaml.Marshal(&fm)
		if err != nil {
			return fmt.Errorf("failed to marshal updated review frontmatter: %v", err)
		}

		var newReviewContent strings.Builder
		newReviewContent.WriteString("---\n")
		newReviewContent.Write(newFmBytes)
		newReviewContent.WriteString("---\n")
		newReviewContent.WriteString(body)

		if err := ioutil.WriteFile(reviewFile, []byte(newReviewContent.String()), 0644); err != nil {
			return fmt.Errorf("failed to write updated review file: %v", err)
		}

		// Update task status to done
		t.Frontmatter.Status = "done"
		if err := task.SaveTask(taskFile, t); err != nil {
			return fmt.Errorf("failed to save updated task: %v", err)
		}

		fmt.Printf("Task %s status promoted to DONE.\n", t.Frontmatter.ID)
		return nil
	}

	fmt.Println("Pending review. Some criteria are not yet checked. Please update the review checklist and run again.")
	return nil
}
