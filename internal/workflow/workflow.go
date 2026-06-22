package workflow

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"

	"kv/internal/workspace"
)

var slugRegex = regexp.MustCompile(`^[a-zA-Z0-9\-]+$`)

// NewWorkflow creates a new workflow inside the workspace.
func NewWorkflow(workspaceDir, slug string) error {
	if !slugRegex.MatchString(slug) {
		return fmt.Errorf("invalid workflow slug '%s'; only alphanumeric characters and dashes are allowed", slug)
	}

	workflowDir := filepath.Join(workspaceDir, workspace.ConfigDirName, "workflows", slug)

	// Create directories
	subdirs := []string{
		"tasks",
		"context",
		"reviews",
		"memory",
	}

	for _, subdir := range subdirs {
		dirPath := filepath.Join(workflowDir, subdir)
		if err := os.MkdirAll(dirPath, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %v", subdir, err)
		}
	}

	// Write markdown templates
	templates := map[string]string{
		"idea.md":     ideaTemplate(slug),
		"prd.md":      prdTemplate(slug),
		"techspec.md": techspecTemplate(slug),
	}

	for filename, content := range templates {
		filePath := filepath.Join(workflowDir, filename)
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			if err := ioutil.WriteFile(filePath, []byte(content), 0644); err != nil {
				return fmt.Errorf("failed to write template %s: %v", filename, err)
			}
		}
	}

	// Write default task 001-setup.md
	setupTaskPath := filepath.Join(workflowDir, "tasks", "001-setup.md")
	if _, err := os.Stat(setupTaskPath); os.IsNotExist(err) {
		if err := ioutil.WriteFile(setupTaskPath, []byte(defaultSetupTaskContent), 0644); err != nil {
			return fmt.Errorf("failed to write default task: %v", err)
		}
	}

	return nil
}

func ideaTemplate(slug string) string {
	return fmt.Sprintf(`# Idea: %s

## Overview
A brief summary of the proposed feature or changes.

## Problem Statement
What problem does this idea solve?

## Proposed Solution
High-level description of the solution.
`, slug)
}

func prdTemplate(slug string) string {
	return fmt.Sprintf(`# Product Requirement Document (PRD): %s

## User Value
Why is this feature important to the user?

## Key Requirements
- [ ] Requirement 1
- [ ] Requirement 2

## Out of Scope
What is NOT covered by this workflow.
`, slug)
}

func techspecTemplate(slug string) string {
	return fmt.Sprintf(`# Technical Specification: %s

## Proposed Architecture
Details on the packages, data models, or APIs to modify/create.

## Affected Components
- Component A: Description

## Validation Plan
Describe how these changes will be validated (e.g. commands, unit tests, manual checks).
`, slug)
}

const defaultSetupTaskContent = `---
id: 001-setup
title: "Setup workflow environment"
status: todo
type: chore
complexity: low
dependencies: []
agent/runner: opencode
sources: []
acceptanceCriteria:
  - "Workflow structure is created and verified"
---

# Setup Workflow Environment

Initialize, check configuration, and verify that the workflow directories exist.
`
