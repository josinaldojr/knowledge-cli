package workspace

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const WorkspaceYamlFileName = "kv-workspace.yaml"

// WorkspaceYaml represents the schema for kv-workspace.yaml.
type WorkspaceYaml struct {
	Workspace WorkspaceInfo `yaml:"workspace"`
}

// WorkspaceInfo contains the workspace name and its applications.
type WorkspaceInfo struct {
	Name string `yaml:"name"`
	Apps []App  `yaml:"apps"`
}

// App represents a single application registered in the workspace.
type App struct {
	ID    string `yaml:"id"`
	Name  string `yaml:"name"`
	Path  string `yaml:"path"`
	Type  string `yaml:"type"`
	Stack string `yaml:"stack"`
}

// FindWorkspaceYamlDir searches for the directory containing kv-workspace.yaml starting from startDir and walking up.
func FindWorkspaceYamlDir(startDir string) (string, error) {
	current := filepath.Clean(startDir)
	for {
		yamlPath := filepath.Join(current, WorkspaceYamlFileName)
		if info, err := os.Stat(yamlPath); err == nil && !info.IsDir() {
			return current, nil
		}
		parent := filepath.Dir(current)
		if parent == current {
			break
		}
		current = parent
	}
	return "", fmt.Errorf("workspace not initialized (kv-workspace.yaml not found in parent directories)")
}

// LoadWorkspaceYaml loads and parses the kv-workspace.yaml file.
func LoadWorkspaceYaml(workspaceDir string) (*WorkspaceYaml, error) {
	yamlPath := filepath.Join(workspaceDir, WorkspaceYamlFileName)
	data, err := ioutil.ReadFile(yamlPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %v", WorkspaceYamlFileName, err)
	}

	var ws WorkspaceYaml
	if err := yaml.Unmarshal(data, &ws); err != nil {
		return nil, fmt.Errorf("failed to parse yaml in %s: %v", WorkspaceYamlFileName, err)
	}

	return &ws, nil
}

// SaveWorkspaceYaml saves the WorkspaceYaml struct to kv-workspace.yaml.
func SaveWorkspaceYaml(workspaceDir string, ws *WorkspaceYaml) error {
	yamlPath := filepath.Join(workspaceDir, WorkspaceYamlFileName)
	data, err := yaml.Marshal(ws)
	if err != nil {
		return fmt.Errorf("failed to marshal workspace yaml: %v", err)
	}

	if err := ioutil.WriteFile(yamlPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write %s: %v", WorkspaceYamlFileName, err)
	}

	return nil
}

// Validate validates the workspace schema, ensures IDs are unique, and verifies path existence.
func (w *WorkspaceYaml) Validate(workspaceDir string) error {
	if strings.TrimSpace(w.Workspace.Name) == "" {
		return fmt.Errorf("workspace name cannot be empty")
	}

	seenIDs := make(map[string]bool)
	for i, app := range w.Workspace.Apps {
		appID := strings.TrimSpace(app.ID)
		if appID == "" {
			return fmt.Errorf("app at index %d is missing required field 'id'", i)
		}
		if seenIDs[appID] {
			return fmt.Errorf("duplicate app ID: %s", appID)
		}
		seenIDs[appID] = true

		if strings.TrimSpace(app.Name) == "" {
			return fmt.Errorf("app '%s' is missing required field 'name'", appID)
		}
		if strings.TrimSpace(app.Path) == "" {
			return fmt.Errorf("app '%s' is missing required field 'path'", appID)
		}
		if strings.TrimSpace(app.Type) == "" {
			return fmt.Errorf("app '%s' is missing required field 'type'", appID)
		}
		if strings.TrimSpace(app.Stack) == "" {
			return fmt.Errorf("app '%s' is missing required field 'stack'", appID)
		}

		// Resolve app path to check its existence on disk
		resolvedPath := app.Path
		if !filepath.IsAbs(resolvedPath) {
			resolvedPath = filepath.Join(workspaceDir, app.Path)
		}

		if _, err := os.Stat(resolvedPath); err != nil {
			if os.IsNotExist(err) {
				return fmt.Errorf("app '%s' path '%s' does not exist", appID, app.Path)
			}
			return fmt.Errorf("failed to validate app '%s' path '%s': %v", appID, app.Path, err)
		}
	}

	return nil
}
