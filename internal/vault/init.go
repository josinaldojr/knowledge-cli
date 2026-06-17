package vault

import (
	"encoding/json"
	"fmt"
	"path/filepath"

	"kv/internal/fsutil"
)

// VaultMetadata defines the structure of the .kv-vault file.
type VaultMetadata struct {
	Type      string `json: "type"`
	Version   int    `json: "version"`
	Name      string `json: "name"`
	CreatedBy string `json: "created_by"`
	Layout    string `json: "layout"`
}

// Init creates a new Knowledge Vault at the specified path.
func Init(vaultPath string) error {
	absPath, err := fsutil.ResolveAbs(vaultPath)
	if err != nil {
		return fmt.Errorf("failed to resolve absolute path of vault: %v", err)
	}

	fmt.Printf("Initializing Knowledge Vault at: %s\n\n", absPath)

	// 1. Create directory structure
	folders := []string{
		"00-inbox",
		"01-global",
		"02-domains",
		"03-projects",
		"04-systems",
		"05-decisions",
		"06-agents",
		"07-runbooks",
		"08-prompts",
		"09-templates",
		"10-references",
	}

	for _, folder := range folders {
		folderPath := filepath.Join(absPath, folder)
		if fsutil.IsDir(folderPath) {
			fmt.Printf("[SKIP] Folder %s/ (already exists)\n", folder)
		} else {
			if err := fsutil.EnsureDir(folderPath); err != nil {
				return fmt.Errorf("failed to create directory %s: %v", folder, err)
			}
			fmt.Printf("[CREATE] Folder %s/\n", folder)
		}
	}

	// 2. Create .kv-vault file
	metaPath := filepath.Join(absPath, ".kv-vault")
	if fsutil.IsFile(metaPath) {
		fmt.Println("[SKIP] .kv-vault (already exists)")
	} else {
		metadata := VaultMetadata{
			Type:      "knowledge-vault",
			Version:   1,
			Name:      "knowledge-vault",
			CreatedBy: "kv",
			Layout:    "default",
		}
		data, err := json.MarshalIndent(metadata, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal vault metadata: %v", err)
		}
		// add trailing newline
		data = append(data, '\n')
		if err := fsutil.WriteFile(metaPath, data, 0644); err != nil {
			return fmt.Errorf("failed to write .kv-vault: %v", err)
		}
		fmt.Println("[CREATE] .kv-vault")
	}

	// Helper to write files if they don't exist
	writeFileIfMissing := func(filename string, content string) error {
		filePath := filepath.Join(absPath, filename)
		if fsutil.IsFile(filePath) {
			fmt.Printf("[SKIP] %s (already exists)\n", filename)
			return nil
		}
		if err := fsutil.WriteFile(filePath, []byte(content), 0644); err != nil {
			return fmt.Errorf("failed to write %s: %v", filename, err)
		}
		fmt.Printf("[CREATE] %s\n", filename)
		return nil
	}

	// 3. Create root files
	if err := writeFileIfMissing("README.md", defaultReadme); err != nil {
		return err
	}
	if err := writeFileIfMissing("AGENTS.md", defaultAgentsGuide); err != nil {
		return err
	}

	// 4. Create nested files
	nestedFiles := map[string]string{
		"01-global/documentation-standards.md": defaultDocStandards,
		"06-agents/backend-agent.md":            defaultBackendAgent,
		"06-agents/frontend-agent.md":           defaultFrontendAgent,
		"06-agents/qa-agent.md":                 defaultQaAgent,
		"09-templates/system-context.md":        defaultSystemContextTemplate,
		"09-templates/adr.md":                   defaultAdrTemplate,
		"09-templates/project-manifest.md":      defaultProjectManifestTemplate,
	}

	for relPath, content := range nestedFiles {
		if err := writeFileIfMissing(relPath, content); err != nil {
			return err
		}
	}

	fmt.Println("\nKnowledge Vault initialized successfully.")
	return nil
}

// Default file contents
const defaultReadme = `# Knowledge Vault

Welcome to your Knowledge Vault. This directory is the single source of truth for architecture, systems, domains, decisions, and documentation.

## Vault Layout

* **00-inbox/**: New/unorganized thoughts, scratch notes, and temporary uploads.
* **01-global/**: Standards, guides, design systems, and cross-cutting concepts.
* **02-domains/**: Business sub-domains and core language definitions.
* **03-projects/**: Multi-system projects and initiatives.
* **04-systems/**: System architectures, service boundaries, and APIs.
* **05-decisions/**: Architectural Decision Records (ADRs).
* **06-agents/**: Instructions, instructions files, and personas for AI agents.
* **07-runbooks/**: Operational runbooks, setup, and deployment guides.
* **08-prompts/**: Reusable prompts for LLMs and assistant systems.
* **09-templates/**: Templates for systems, ADRs, projects, and domains.
* **10-references/**: Third-party specs, papers, and reference documents.
`

const defaultAgentsGuide = `# AI Agent Guidelines

This document outlines how AI agents interact with this Knowledge Vault.

## Core Rules

1. **Be Precise**: Documentation must be grounded in facts from the source code.
2. **YAML Frontmatter**: All canonical markdown notes should start with yaml frontmatter containing metadata:
   ` + "```yaml" + `
   status: canon | draft | needs-review
   created_at: YYYY-MM-DD
   updated_at: YYYY-MM-DD
   ` + "```" + `
3. **No Deletions**: Do not delete existing documentation unless explicitly requested. If documentation is obsolete, archive it or mark it as deprecated.
4. **Needs Review**: When in doubt or missing information, write it down and tag it as ` + "`needs-review`" + `.
`

const defaultDocStandards = `# Documentation Standards

We follow these guidelines to keep our documentation clear, searchable, and maintainable.

## Frontmatter

Every markdown file in this vault should start with YAML frontmatter containing:
* ` + "`status`" + `: ` + "`canon`" + ` (official), ` + "`draft`" + ` (work in progress), or ` + "`needs-review`" + ` (requires validation).
* ` + "`created_at`" + `: Creation date (YYYY-MM-DD).
* ` + "`updated_at`" + `: Last modification date (YYYY-MM-DD).

Example:
` + "```yaml" + `
status: canon
created_at: 2026-06-17
updated_at: 2026-06-17
` + "```" + `

## File Naming

* Use kebab-case for file names (e.g., ` + "`documentation-standards.md`" + `).
* Place files in their respective numbered category directories.
`

const defaultBackendAgent = `# Backend Engineering Agent Instructions

You are a Senior Backend AI assistant. Your goal is to write robust, maintainable, and well-documented backend systems.

## Documentation Guidelines

* Standardize endpoint definitions using OpenAPI or clear markdown schemas.
* Maintain DB migrations records and document schema changes.
* Describe authentication and authorization structures.
`

const defaultFrontendAgent = `# Frontend Engineering Agent Instructions

You are a Senior Frontend AI assistant. Your goal is to build premium, modern, accessible, and performant user interfaces.

## Documentation Guidelines

* Document design system tokens and components.
* Maintain clear state management and component structures description.
* Detail routing and build scripts configurations.
`

const defaultQaAgent = `# Quality Assurance Agent Instructions

You are a Senior QA AI assistant. Your goal is to verify correctness and assert high-quality deliverables.

## Documentation Guidelines

* Document test matrices and automation configurations.
* Detail setup instructions for running integration and end-to-end (E2E) tests.
`

const defaultSystemContextTemplate = `---
status: draft
created_at: 2026-06-17
updated_at: 2026-06-17
---

# System Context: [System Name]

## Overview

[Provide a high-level summary of the system and its business goals.]

## Architecture

[Describe the tech stack, databases, third-party libraries, and frameworks used.]

## Integrations

[List APIs, event buses, or queues this system consumes or exposes.]
`

const defaultAdrTemplate = `---
status: draft
created_at: 2026-06-17
updated_at: 2026-06-17
---

# ADR-[Number]: [Title]

## Context

[Context and problem statement describing the forces and constraints.]

## Decision

[The chosen solution and rationale.]

## Consequences

[What becomes easier or harder as a result of this decision.]
`

const defaultProjectManifestTemplate = `---
status: draft
created_at: 2026-06-17
updated_at: 2026-06-17
---

# Project Manifest: [Project Name]

## Objectives

[What are we trying to accomplish in this project?]

## In-Scope Systems

* [List of system/repositories impacted]

## Timeline & Status

* [Draft / Active / Completed]
`
