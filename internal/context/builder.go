package context

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"kv/internal/fsutil"
	"kv/internal/session"
	"kv/internal/vault"
	"kv/internal/workspace"
)

// BuildContext copies the context pack of the specified task to .opencode/context.md (legacy command)
func BuildContext(workspaceDir, workflowSlug, taskID string) error {
	// Resolve paths
	workflowDir := filepath.Join(workspaceDir, workspace.ConfigDirName, "workflows", workflowSlug)
	contextPackPath := filepath.Join(workflowDir, "context", taskID+".context.md")

	if _, err := os.Stat(contextPackPath); os.IsNotExist(err) {
		return fmt.Errorf("context pack file %s not found. Please run 'kv task enrich' first", contextPackPath)
	}

	data, err := ioutil.ReadFile(contextPackPath)
	if err != nil {
		return fmt.Errorf("failed to read context pack file: %v", err)
	}

	opencodeDir := filepath.Join(workspaceDir, ".opencode")
	if err := os.MkdirAll(opencodeDir, 0755); err != nil {
		return fmt.Errorf("failed to create .opencode directory: %v", err)
	}

	targetPath := filepath.Join(opencodeDir, "context.md")
	if err := ioutil.WriteFile(targetPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write .opencode/context.md file: %v", err)
	}

	return nil
}

// BuildSessionContext compiles global context, app-specific contexts, and context-manifest.json for a session.
// Returns list of generated files, number of apps processed, warnings list, and error.
func BuildSessionContext(workspaceDir, sessionID string) ([]string, int, []string, error) {
	// 1. Load session
	sess, err := session.LoadSession(workspaceDir, sessionID)
	if err != nil {
		return nil, 0, nil, fmt.Errorf("session not found: %v", err)
	}

	// 2. Load workspace
	ws, err := workspace.LoadWorkspaceYaml(workspaceDir)
	if err != nil {
		return nil, 0, nil, fmt.Errorf("failed to load workspace: %v", err)
	}

	// Build app map from workspace
	appMap := make(map[string]workspace.App)
	for _, app := range ws.Workspace.Apps {
		appMap[app.ID] = app
	}

	// Discover vault
	var vaultPath string
	var vaultWarnings []string
	vaultRes, vaultErr := vault.FindVault(workspaceDir)
	if vaultErr != nil {
		vaultWarnings = append(vaultWarnings, "Warning: no active Knowledge Vault found.")
	} else {
		vaultPath = vaultRes.Path
	}

	var generatedFiles []string
	var warnings []string
	warnings = append(warnings, vaultWarnings...)
	var manifestApps []AppContext

	// 3. Process each app selected in session
	for _, appID := range sess.SelectedApps {
		app, ok := appMap[appID]
		if !ok {
			return nil, 0, nil, fmt.Errorf("app '%s' selected in session is not registered in the workspace", appID)
		}

		// Resolve app folder
		resolvedPath := app.Path
		if !filepath.IsAbs(resolvedPath) {
			resolvedPath = filepath.Join(workspaceDir, app.Path)
		}

		if !fsutil.IsDir(resolvedPath) {
			return nil, 0, nil, fmt.Errorf("app '%s' path '%s' is not a directory or does not exist", appID, resolvedPath)
		}

		// Walk app tree (respect depth = 4)
		tree, err := buildFileTree(resolvedPath, 4)
		if err != nil {
			return nil, 0, nil, fmt.Errorf("failed to construct file tree for app '%s': %v", appID, err)
		}

		// Find candidate files
		candidates, err := findCandidateFiles(resolvedPath, sess.Goal)
		if err != nil {
			return nil, 0, nil, fmt.Errorf("failed to identify candidate files for app '%s': %v", appID, err)
		}

		// Associated vault file check
		var assocVaultFile string
		var vaultSummary string
		if vaultPath != "" {
			assoc, err := findVaultFile(vaultPath, appID)
			if err == nil && assoc != "" {
				assocVaultFile = assoc
				// Read content up to size limit (10KB)
				summary, err := readTruncatedFile(assoc, 10*1024)
				if err == nil {
					vaultSummary = summary
				} else {
					vaultSummary = fmt.Sprintf("*(Failed to read vault file content: %v)*", err)
				}
			} else {
				warnings = append(warnings, fmt.Sprintf("Warning: vault file for app '%s' does not exist in active vault.", appID))
			}
		}

		// Render and save app context
		appData := &AppContextData{
			ID:             app.ID,
			Name:           app.Name,
			Type:           app.Type,
			Stack:          app.Stack,
			Path:           app.Path,
			Commands:       nil, // Commands can be added here or read from app config
			VaultFile:      assocVaultFile,
			VaultSummary:   vaultSummary,
			FileTree:       tree,
			CandidateFiles: candidates,
		}

		appMDContent := RenderAppContext(appData)
		if err := SaveAppContext(workspaceDir, sessionID, appID, appMDContent); err != nil {
			return nil, 0, nil, fmt.Errorf("failed to save context for app '%s': %v", appID, err)
		}

		appContextRel := filepath.Join(".kv", "sessions", sessionID, "context", "apps", appID+".context.md")
		generatedFiles = append(generatedFiles, filepath.Join(workspaceDir, appContextRel))

		manifestApps = append(manifestApps, AppContext{
			AppID:       appID,
			ContextPath: appContextRel,
			VaultFile:   assocVaultFile,
		})
	}

	// 4. Denied paths lists ignored folders
	deniedPaths := []string{
		"Any path outside allowed zones",
		"Specifically ignored subfolders: .git, node_modules, dist, build, vendor, coverage, .next, target",
	}

	// 5. Render and save global context
	globalData := &GlobalContextData{
		SessionID:    sessionID,
		Goal:         sess.Goal,
		SelectedApps: sess.SelectedApps,
		AllowedPaths: sess.Boundary.AllowedPaths,
		DeniedPaths:  deniedPaths,
		Rules: []string{
			"Do not modify files outside allowed paths.",
			"Do not create/modify files in ignored directories.",
			"Keep changes minimal and focused on the session goal.",
			"Verify changes by running the appropriate unit/integration tests.",
		},
	}

	globalMDContent := RenderGlobalContext(globalData)
	if err := SaveGlobalContext(workspaceDir, sessionID, globalMDContent); err != nil {
		return nil, 0, nil, fmt.Errorf("failed to save global context: %v", err)
	}

	globalContextRel := filepath.Join(".kv", "sessions", sessionID, "context", "global.context.md")
	generatedFiles = append(generatedFiles, filepath.Join(workspaceDir, globalContextRel))

	// 6. Save Manifest
	manifest := &Manifest{
		SessionID:         sessionID,
		GeneratedAt:       time.Now().Format(time.RFC3339),
		GlobalContextPath: globalContextRel,
		Apps:              manifestApps,
	}

	if err := SaveManifest(workspaceDir, sessionID, manifest); err != nil {
		return nil, 0, nil, fmt.Errorf("failed to save context manifest: %v", err)
	}

	manifestRel := filepath.Join(".kv", "sessions", sessionID, "context", "context-manifest.json")
	generatedFiles = append(generatedFiles, filepath.Join(workspaceDir, manifestRel))

	return generatedFiles, len(sess.SelectedApps), warnings, nil
}

// Helpers

func buildFileTree(dir string, maxDepth int) (string, error) {
	var sb strings.Builder
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}

		if relPath == "." {
			return nil
		}

		parts := strings.Split(relPath, string(filepath.Separator))
		depth := len(parts)

		name := info.Name()
		if info.IsDir() && isIgnoredDir(name) {
			return filepath.SkipDir
		}

		if depth > maxDepth {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		indent := strings.Repeat("  ", depth-1)
		if info.IsDir() {
			sb.WriteString(fmt.Sprintf("%s- %s/\n", indent, name))
		} else {
			sb.WriteString(fmt.Sprintf("%s- %s\n", indent, name))
		}

		return nil
	})
	return sb.String(), err
}

func isIgnoredDir(name string) bool {
	ignored := []string{".git", "node_modules", "dist", "build", "vendor", "coverage", ".next", "target"}
	for _, ign := range ignored {
		if name == ign {
			return true
		}
	}
	return false
}

func findCandidateFiles(dir string, goal string) ([]string, error) {
	words := strings.Fields(strings.ToLower(goal))
	var keywords []string
	stopWords := map[string]bool{
		"a": true, "an": true, "the": true, "and": true, "or": true, "but": true,
		"for": true, "to": true, "of": true, "in": true, "on": true, "at": true,
		"with": true, "by": true, "refactor": true, "implement": true, "build": true,
	}
	for _, w := range words {
		cleaned := strings.Trim(w, ",.?!;:()[]{}'\"")
		if len(cleaned) > 2 && !stopWords[cleaned] {
			keywords = append(keywords, cleaned)
		}
	}

	type candidate struct {
		path  string
		score int
	}
	var candidates []candidate

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			if isIgnoredDir(info.Name()) {
				return filepath.SkipDir
			}
			return nil
		}

		ext := strings.ToLower(filepath.Ext(path))
		validExts := map[string]bool{
			".go": true, ".ts": true, ".js": true, ".tsx": true, ".jsx": true,
			".py": true, ".yaml": true, ".yml": true, ".json": true, ".md": true,
			".cs": true, ".java": true, ".cpp": true, ".h": true, ".rb": true,
			".php": true, ".sh": true,
		}
		if !validExts[ext] {
			return nil
		}

		rel, _ := filepath.Rel(dir, path)
		score := 0
		lowerRel := strings.ToLower(rel)
		lowerName := strings.ToLower(info.Name())

		for _, kw := range keywords {
			if strings.Contains(lowerName, kw) {
				score += 5
			} else if strings.Contains(lowerRel, kw) {
				score += 2
			}
		}

		if score > 0 {
			candidates = append(candidates, candidate{path: rel, score: score})
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	// Sort candidates by score descending
	for i := 0; i < len(candidates); i++ {
		for j := i + 1; j < len(candidates); j++ {
			if candidates[j].score > candidates[i].score {
				candidates[i], candidates[j] = candidates[j], candidates[i]
			}
		}
	}

	var results []string
	for _, c := range candidates {
		results = append(results, c.path)
	}

	if len(results) > 10 {
		results = results[:10]
	}

	if len(results) == 0 {
		count := 0
		_ = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err != nil || count >= 5 {
				return nil
			}
			if info.IsDir() {
				if isIgnoredDir(info.Name()) {
					return filepath.SkipDir
				}
				return nil
			}
			ext := strings.ToLower(filepath.Ext(path))
			if ext == ".go" || ext == ".ts" || ext == ".js" || ext == ".py" || ext == ".json" {
				rel, _ := filepath.Rel(dir, path)
				results = append(results, rel)
				count++
			}
			return nil
		})
	}

	return results, nil
}

func findVaultFile(vaultPath, appID string) (string, error) {
	candidates := []string{
		filepath.Join(vaultPath, appID+".md"),
		filepath.Join(vaultPath, "apps", appID+".md"),
	}
	for _, c := range candidates {
		if fsutil.IsFile(c) {
			return c, nil
		}
	}
	return "", nil
}

func readTruncatedFile(path string, limit int) (string, error) {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return "", err
	}
	if len(data) <= limit {
		return string(data), nil
	}
	return string(data[:limit]) + "\n\n*(Content truncated: size limit reached)*", nil
}
