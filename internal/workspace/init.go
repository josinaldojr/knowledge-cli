package workspace

import (
	"fmt"
	"os"
	"path/filepath"

	"kv/internal/fsutil"
)

// Init initializes the current workspace with a pointer to the given vault path.
func Init(vaultRawPath string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current working directory: %v", err)
	}

	// 1. Resolve absolute vault path
	absVault, err := fsutil.ResolveAbs(vaultRawPath)
	if err != nil {
		return fmt.Errorf("failed to resolve absolute path of vault: %v", err)
	}

	// 2. Validate that it contains the .kv-vault file
	markerPath := filepath.Join(absVault, ".kv-vault")
	if !fsutil.IsFile(markerPath) {
		return fmt.Errorf("the path '%s' is not a valid Knowledge Vault (missing '.kv-vault' file)", absVault)
	}

	// 3. Determine the path to write to .knowledge-vault (relative preferred)
	relPath, err := filepath.Rel(cwd, absVault)
	writePath := relPath
	if err != nil {
		// If relative path fails (e.g., crossing drive roots on Windows), use absolute path
		writePath = absVault
	}

	// Clean the write path to use standard system separator
	writePath = filepath.Clean(writePath)

	// 4. Write to .knowledge-vault in cwd with backup if already exists
	backedUp, err := WriteMarker(cwd, writePath)
	if err != nil {
		return fmt.Errorf("failed to write marker file: %v", err)
	}

	// 5. Copy OpenCode templates
	fmt.Println("Installing OpenCode workspace templates...")
	if err := copyOpencodeTemplates(cwd); err != nil {
		return fmt.Errorf("failed to copy OpenCode templates: %v", err)
	}
	fmt.Println()

	// 6. Display output
	fmt.Printf("Workspace initialized.\n\n")
	fmt.Printf("Workspace:       %s\n", cwd)
	fmt.Printf("Vault resolved:  %s\n", absVault)
	markerFile := filepath.Join(cwd, MarkerFilename)
	if backedUp {
		fmt.Printf("Marker created:  %s (backup created at %s%s)\n", markerFile, markerFile, BackupSuffix)
	} else {
		fmt.Printf("Marker created:  %s\n", markerFile)
	}
	fmt.Printf("Marker content:  %s\n", writePath)

	return nil
}

// copyOpencodeTemplates writes all the OpenCode templates to .opencode/ folder if they don't already exist.
func copyOpencodeTemplates(cwd string) error {
	opencodeDir := filepath.Join(cwd, ".opencode")
	if err := fsutil.EnsureDir(opencodeDir); err != nil {
		return err
	}

	files := map[string]string{
		"opencode.json":            OpencodeJsonTemplate,
		"AGENTS.md":                AgentsMdTemplate,
		"commands/kv-plan.md":      CommandPlanTemplate,
		"commands/kv-implement.md": CommandImplementTemplate,
		"commands/kv-review.md":    CommandReviewTemplate,
		"commands/kv-sync.md":      CommandSyncTemplate,
		"agents/architect.md":      AgentArchitectTemplate,
		"agents/backend.md":        AgentBackendTemplate,
		"agents/frontend.md":       AgentFrontendTemplate,
		"agents/knowledge.md":      AgentKnowledgeTemplate,
		"agents/orchestrator.md":   AgentOrchestratorTemplate,
		"agents/reviewer.md":       AgentReviewerTemplate,
	}

	for relPath, content := range files {
		targetPath := filepath.Join(opencodeDir, relPath)
		if fsutil.Exists(targetPath) {
			fmt.Printf("[SKIP] %s (already exists)\n", filepath.Join(".opencode", relPath))
		} else {
			if err := fsutil.WriteFile(targetPath, []byte(content), 0644); err != nil {
				return err
			}
			fmt.Printf("[CREATE] %s\n", filepath.Join(".opencode", relPath))
		}
	}
	return nil
}
