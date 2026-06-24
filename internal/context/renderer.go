package context

import (
	"fmt"
	"path/filepath"
	"strings"

	"kv/internal/fsutil"
)

// GlobalContextData holds the variables needed to render global.context.md.
type GlobalContextData struct {
	SessionID    string
	Goal         string
	SelectedApps []string
	AllowedPaths []string
	DeniedPaths  []string
	Rules        []string
}

// AppContextData holds the variables needed to render <app-id>.context.md.
type AppContextData struct {
	ID             string
	Name           string
	Type           string
	Stack          string
	Path           string
	Commands       map[string]string
	VaultFile      string
	VaultSummary   string
	FileTree       string
	CandidateFiles []string
}

// RenderGlobalContext renders the Markdown content for global.context.md.
func RenderGlobalContext(data *GlobalContextData) string {
	var sb strings.Builder
	sb.WriteString("# Global Session Context\n\n")
	sb.WriteString(fmt.Sprintf("- **Session ID**: `%s`\n", data.SessionID))
	sb.WriteString(fmt.Sprintf("- **Goal**: %s\n\n", data.Goal))

	sb.WriteString("## Selected Applications\n")
	for _, app := range data.SelectedApps {
		sb.WriteString(fmt.Sprintf("- `%s`\n", app))
	}
	sb.WriteString("\n")

	sb.WriteString("## Boundaries & Security Constraints\n\n")
	sb.WriteString("### Allowed Paths (Allowed write/read execution zones):\n")
	for _, p := range data.AllowedPaths {
		sb.WriteString(fmt.Sprintf("- `%s`\n", p))
	}
	sb.WriteString("\n")

	sb.WriteString("### Denied Paths (Forbidden read/write zones):\n")
	for _, p := range data.DeniedPaths {
		sb.WriteString(fmt.Sprintf("- `%s`\n", p))
	}
	sb.WriteString("\n")

	sb.WriteString("## Agent Rules & Guidelines\n")
	for _, r := range data.Rules {
		sb.WriteString(fmt.Sprintf("- %s\n", r))
	}
	return sb.String()
}

// RenderAppContext renders the Markdown content for <app-id>.context.md.
func RenderAppContext(data *AppContextData) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# Application Context: %s\n\n", data.Name))
	sb.WriteString(fmt.Sprintf("- **ID**: `%s`\n", data.ID))
	sb.WriteString(fmt.Sprintf("- **Type**: `%s`\n", data.Type))
	sb.WriteString(fmt.Sprintf("- **Stack**: `%s`\n", data.Stack))
	sb.WriteString(fmt.Sprintf("- **Path**: `%s`\n\n", data.Path))

	sb.WriteString("## Configured Commands\n")
	if len(data.Commands) == 0 {
		sb.WriteString("*(No configured commands for this app.)*\n\n")
	} else {
		for cmdName, cmdLine := range data.Commands {
			sb.WriteString(fmt.Sprintf("- **%s**: `%s`\n", cmdName, cmdLine))
		}
		sb.WriteString("\n")
	}

	sb.WriteString("## Knowledge Vault documentation\n")
	if data.VaultFile == "" {
		sb.WriteString("*(No specific vault documentation file associated with this app.)*\n\n")
	} else {
		sb.WriteString(fmt.Sprintf("- **Vault Document**: `%s`\n\n", filepath.Clean(data.VaultFile)))
		sb.WriteString("### Document Summary / Contents:\n")
		sb.WriteString(data.VaultSummary + "\n\n")
	}

	sb.WriteString("## Relevant File Tree\n")
	sb.WriteString("```\n")
	sb.WriteString(data.FileTree)
	sb.WriteString("```\n\n")

	sb.WriteString("## Candidate Files for Modification\n")
	if len(data.CandidateFiles) == 0 {
		sb.WriteString("*(No specific files identified as candidates.)*\n")
	} else {
		for _, f := range data.CandidateFiles {
			sb.WriteString(fmt.Sprintf("- `%s`\n", f))
		}
	}
	return sb.String()
}

// SaveGlobalContext writes the global context markdown file.
func SaveGlobalContext(workspaceDir, sessionID, content string) error {
	path := filepath.Join(workspaceDir, ".kv", "sessions", sessionID, "context", "global.context.md")
	return fsutil.WriteFile(path, []byte(content), 0644)
}

// SaveAppContext writes an application context markdown file.
func SaveAppContext(workspaceDir, sessionID, appID, content string) error {
	path := filepath.Join(workspaceDir, ".kv", "sessions", sessionID, "context", "apps", appID+".context.md")
	return fsutil.WriteFile(path, []byte(content), 0644)
}
