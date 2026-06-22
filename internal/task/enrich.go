package task

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"kv/internal/fsutil"
	"kv/internal/vault"
	"kv/internal/workspace"
)

// EnrichTask gathers context from repository source files, vault documentation, decisions,
// and validation plans, and compiles them into a markdown context pack file.
func EnrichTask(workspaceDir, workflowSlug, taskID string) error {
	// 1. Load config
	cfg, err := workspace.LoadConfig(workspaceDir)
	if err != nil {
		return fmt.Errorf("failed to load workspace config: %v", err)
	}

	// 2. Locate and parse task file
	workflowDir := filepath.Join(workspaceDir, workspace.ConfigDirName, "workflows", workflowSlug)
	taskFile := filepath.Join(workflowDir, "tasks", taskID+".md")
	if _, err := os.Stat(taskFile); os.IsNotExist(err) {
		return fmt.Errorf("task file %s not found", taskFile)
	}

	parsedTask, err := ParseTask(taskFile)
	if err != nil {
		return fmt.Errorf("failed to parse task: %v", err)
	}

	// Update task status if it was todo
	if parsedTask.Frontmatter.Status == "todo" || parsedTask.Frontmatter.Status == "" {
		parsedTask.Frontmatter.Status = "in-progress"
		if err := SaveTask(taskFile, parsedTask); err != nil {
			return fmt.Errorf("failed to update task status: %v", err)
		}
	}

	var sb strings.Builder
	sb.WriteString("---\n")
	sb.WriteString(fmt.Sprintf("task_id: %s\n", taskID))
	sb.WriteString(fmt.Sprintf("workflow: %s\n", workflowSlug))
	sb.WriteString(fmt.Sprintf("generated_at: %s\n", time.Now().Format(time.RFC3339)))
	sb.WriteString("---\n\n")

	sb.WriteString(fmt.Sprintf("# Context Pack: %s\n\n", parsedTask.Frontmatter.Title))

	// Section 1: Task Definition
	sb.WriteString("## 1. Task Definition\n\n")
	sb.WriteString(fmt.Sprintf("- **ID**: %s\n", taskID))
	sb.WriteString(fmt.Sprintf("- **Title**: %s\n", parsedTask.Frontmatter.Title))
	sb.WriteString(fmt.Sprintf("- **Status**: %s\n", parsedTask.Frontmatter.Status))
	sb.WriteString(fmt.Sprintf("- **Type**: %s\n", parsedTask.Frontmatter.Type))
	sb.WriteString(fmt.Sprintf("- **Complexity**: %s\n", parsedTask.Frontmatter.Complexity))
	sb.WriteString(fmt.Sprintf("- **Agent/Runner**: %s\n\n", parsedTask.Frontmatter.AgentRunner))

	sb.WriteString("### Description\n")
	sb.WriteString(parsedTask.Content + "\n\n")

	sb.WriteString("### Acceptance Criteria\n")
	if len(parsedTask.Frontmatter.AcceptanceCriteria) == 0 {
		sb.WriteString("*(Nenhum critério especificado)*\n\n")
	} else {
		for _, ac := range parsedTask.Frontmatter.AcceptanceCriteria {
			sb.WriteString(fmt.Sprintf("- [ ] %s\n", ac))
		}
		sb.WriteString("\n")
	}

	// Section 2: Source Code Context
	sb.WriteString("## 2. Source Code Context\n\n")
	if len(parsedTask.Frontmatter.Sources) == 0 {
		sb.WriteString("*(Nenhum arquivo de código fonte associado a esta task)*\n\n")
	} else {
		for _, src := range parsedTask.Frontmatter.Sources {
			sb.WriteString(fmt.Sprintf("### File: `%s`\n\n", src))
			content, err := workspace.ReadRepoFile(workspaceDir, src)
			if err != nil {
				sb.WriteString(fmt.Sprintf("*(Erro ao ler arquivo: %v)*\n\n", err))
			} else {
				ext := filepath.Ext(src)
				lang := strings.TrimPrefix(ext, ".")
				if lang == "" {
					lang = "text"
				}
				sb.WriteString(fmt.Sprintf("```%s\n%s\n```\n\n", lang, content))
			}
		}
	}

	// Section 3 & 4: Vault Knowledge and Decisions
	var vaultKnowledgeResults []vault.SearchResult
	var decisionsResults []vault.SearchResult

	if cfg.VaultPath != "" {
		absVault, err := filepath.Abs(filepath.Join(workspaceDir, cfg.VaultPath))
		if err == nil && fsutil.IsDir(absVault) {
			query := parsedTask.Frontmatter.Title
			searchResults, err := vault.Search(absVault, query, 10)
			if err == nil {
				for _, r := range searchResults {
					rel, _ := filepath.Rel(absVault, r.File)
					// Classify decisions
					if strings.Contains(rel, "05-decisions") || strings.Contains(strings.ToLower(r.Category), "decision") {
						decisionsResults = append(decisionsResults, r)
					} else {
						vaultKnowledgeResults = append(vaultKnowledgeResults, r)
					}
				}
			}
		}
	}

	sb.WriteString("## 3. Vault Knowledge\n\n")
	if len(vaultKnowledgeResults) == 0 {
		sb.WriteString("*(Nenhum documento relevante encontrado no vault)*\n\n")
	} else {
		for _, r := range vaultKnowledgeResults {
			rel := r.File
			if cfg.VaultPath != "" {
				if absVault, err := filepath.Abs(filepath.Join(workspaceDir, cfg.VaultPath)); err == nil {
					if relPath, err := filepath.Rel(absVault, r.File); err == nil {
						rel = relPath
					}
				}
			}
			sb.WriteString(fmt.Sprintf("### %s (`%s`)\n", r.Title, rel))
			sb.WriteString(fmt.Sprintf("- **Score**: %d\n", r.Score))
			sb.WriteString(fmt.Sprintf("> %s\n\n", r.Snippet))
		}
	}

	sb.WriteString("## 4. Architectural Decisions\n\n")
	if len(decisionsResults) == 0 {
		sb.WriteString("*(Nenhuma decisão de arquitetura relevante encontrada)*\n\n")
	} else {
		for _, r := range decisionsResults {
			rel := r.File
			if cfg.VaultPath != "" {
				if absVault, err := filepath.Abs(filepath.Join(workspaceDir, cfg.VaultPath)); err == nil {
					if relPath, err := filepath.Rel(absVault, r.File); err == nil {
						rel = relPath
					}
				}
			}
			sb.WriteString(fmt.Sprintf("### %s (`%s`)\n", r.Title, rel))
			sb.WriteString(fmt.Sprintf("- **Score**: %d\n", r.Score))
			sb.WriteString(fmt.Sprintf("> %s\n\n", r.Snippet))
		}
	}

	// Section 5: Validation Plan & Runbooks
	sb.WriteString("## 5. Validation Plan & Runbooks\n\n")
	validationAdded := false

	// Attempt to load from techspec.md
	techspecPath := filepath.Join(workflowDir, "techspec.md")
	if data, err := ioutil.ReadFile(techspecPath); err == nil {
		plan := extractSection(string(data), "Validation Plan")
		if plan != "" {
			sb.WriteString("### Techspec Validation Plan\n\n")
			sb.WriteString(plan + "\n\n")
			validationAdded = true
		}
	}

	if !validationAdded {
		sb.WriteString("*(Nenhum plano de validação específico encontrado em techspec.md)*\n\n")
	}

	// Write context pack file
	contextDir := filepath.Join(workflowDir, "context")
	contextFile := filepath.Join(contextDir, taskID+".context.md")
	if err := ioutil.WriteFile(contextFile, []byte(sb.String()), 0644); err != nil {
		return fmt.Errorf("failed to write context pack file: %v", err)
	}

	return nil
}

func extractSection(markdown, sectionTitle string) string {
	lines := strings.Split(markdown, "\n")
	start := -1
	headingLevel := 0

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") {
			level := 0
			for _, char := range trimmed {
				if char == '#' {
					level++
				} else {
					break
				}
			}
			titleText := strings.TrimSpace(trimmed[level:])
			if strings.EqualFold(titleText, sectionTitle) {
				start = i
				headingLevel = level
				break
			}
		}
	}

	if start == -1 {
		return ""
	}

	var sectionLines []string
	for i := start + 1; i < len(lines); i++ {
		line := lines[i]
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") {
			level := 0
			for _, char := range trimmed {
				if char == '#' {
					level++
				} else {
					break
				}
			}
			if level <= headingLevel {
				break
			}
		}
		sectionLines = append(sectionLines, line)
	}

	return strings.TrimSpace(strings.Join(sectionLines, "\n"))
}
