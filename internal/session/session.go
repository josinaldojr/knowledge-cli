package session

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strings"
	"time"

	"kv/internal/fsutil"
	"kv/internal/workspace"

	"gopkg.in/yaml.v3"
)

// AppContract represents an application in a session contract.
type AppContract struct {
	Name  string `yaml:"name"`
	Path  string `yaml:"path"`
	Stack string `yaml:"stack"`
}

// VaultContract represents vault config in a session contract.
type VaultContract struct {
	Enabled bool     `yaml:"enabled"`
	Sources []string `yaml:"sources"`
}

// AgentContract represents agent configuration.
type AgentContract struct {
	Provider      string `yaml:"provider"`
	Mode          string `yaml:"mode"`
	ContextBudget int    `yaml:"context_budget"`
}

// QualityContract represents quality gates.
type QualityContract struct {
	Enabled  bool     `yaml:"enabled"`
	Commands []string `yaml:"commands"`
}

// PolicyContract represents basic policy constraints.
type PolicyContract struct {
	Network                bool   `yaml:"network"`
	AllowEnvRead           bool   `yaml:"allow_env_read"`
	AllowDeleteFiles       bool   `yaml:"allow_delete_files"`
	AllowDependencyInstall string `yaml:"allow_dependency_install"`
	AllowMigrations        string `yaml:"allow_migrations"`
	AllowDocker            bool   `yaml:"allow_docker"`
}

// Boundary holds execution boundary constraints.
type Boundary struct {
	AllowedPaths  []string `yaml:"allowed_paths"`
	DeniedPaths   []string `yaml:"denied_paths,omitempty"`
	WritablePaths []string `yaml:"writable_paths,omitempty"`
	ReadonlyPaths []string `yaml:"readonly_paths,omitempty"`
}

// Session represents a workspace multi-app session.
type Session struct {
	ID           string          `yaml:"id"`
	Goal         string          `yaml:"goal"`
	SelectedApps []string        `yaml:"selected_apps"`
	CreatedAt    time.Time       `yaml:"created_at"`
	Apps         []AppContract   `yaml:"apps"`
	Vault        VaultContract   `yaml:"vault"`
	Boundary     Boundary        `yaml:"boundary"`
	Agent        AgentContract   `yaml:"agent"`
	Quality      QualityContract `yaml:"quality"`
	Policy       PolicyContract  `yaml:"policy"`
	Status       string          `yaml:"status"`
}

// GenerateSessionID generates a unique ID for a session.
// Format: sess-20060102-150405-<4 random hex chars>
func GenerateSessionID() string {
	now := time.Now()
	bytes := make([]byte, 2)
	_, _ = rand.Read(bytes)
	randomHex := hex.EncodeToString(bytes)
	return fmt.Sprintf("sess-%s-%s", now.Format("20060102-150405"), randomHex)
}

// InitSession creates a session based on explicit contract flags.
func InitSession(workspaceDir string, id, goal string, appsMap map[string]string, vaultSources []string, writablePaths []string) (*Session, error) {
	if strings.TrimSpace(goal) == "" {
		return nil, fmt.Errorf("session goal cannot be empty")
	}

	sessionID := id
	if strings.TrimSpace(sessionID) == "" {
		sessionID = GenerateSessionID()
	}

	// Create Apps Contract
	var apps []AppContract
	var selectedApps []string
	hasGo := false
	hasNode := false

	for appName, appPath := range appsMap {
		stack := "unknown"
		// Simple auto-detect based on folder contents or path
		fullAppPath := appPath
		if !filepath.IsAbs(fullAppPath) {
			fullAppPath = filepath.Join(workspaceDir, appPath)
		}
		if fsutil.IsFile(filepath.Join(fullAppPath, "go.mod")) {
			stack = "go"
			hasGo = true
		} else if fsutil.IsFile(filepath.Join(fullAppPath, "package.json")) {
			stack = "node"
			hasNode = true
		}

		apps = append(apps, AppContract{
			Name:  appName,
			Path:  appPath,
			Stack: stack,
		})
		selectedApps = append(selectedApps, appName)
	}

	// Allowed paths = app paths + vault sources
	var allowedPaths []string
	for _, app := range apps {
		allowedAbs := app.Path
		if !filepath.IsAbs(allowedAbs) {
			allowedAbs = filepath.Join(workspaceDir, app.Path)
		}
		allowedPaths = append(allowedPaths, filepath.Clean(allowedAbs))
	}
	for _, vs := range vaultSources {
		vsAbs := vs
		if !filepath.IsAbs(vsAbs) {
			vsAbs = filepath.Join(workspaceDir, vs)
		}
		allowedPaths = append(allowedPaths, filepath.Clean(vsAbs))
	}

	// Writable paths clean
	var writableClean []string
	for _, wp := range writablePaths {
		wpAbs := wp
		if !filepath.IsAbs(wpAbs) {
			wpAbs = filepath.Join(workspaceDir, wp)
		}
		writableClean = append(writableClean, filepath.Clean(wpAbs))
	}

	// Readonly paths = allowed paths that are not writable
	var readonlyClean []string
	isWritable := func(p string) bool {
		for _, w := range writableClean {
			if w == p {
				return true
			}
		}
		return false
	}
	for _, ap := range allowedPaths {
		if !isWritable(ap) {
			readonlyClean = append(readonlyClean, ap)
		}
	}

	// Quality commands
	var qCmds []string
	if hasGo {
		qCmds = append(qCmds, "go test ./...", "go vet ./...")
	}
	if hasNode {
		qCmds = append(qCmds, "npm test")
	}

	sess := &Session{
		ID:           sessionID,
		Goal:         goal,
		SelectedApps: selectedApps,
		CreatedAt:    time.Now().Round(time.Second),
		Apps:         apps,
		Vault: VaultContract{
			Enabled: len(vaultSources) > 0,
			Sources: vaultSources,
		},
		Boundary: Boundary{
			AllowedPaths:  allowedPaths,
			WritablePaths: writableClean,
			ReadonlyPaths: readonlyClean,
		},
		Agent: AgentContract{
			Provider:      "opencode",
			Mode:          "implementation",
			ContextBudget: 12000,
		},
		Quality: QualityContract{
			Enabled:  true,
			Commands: qCmds,
		},
		Policy: PolicyContract{
			Network:                false,
			AllowEnvRead:           false,
			AllowDeleteFiles:       false,
			AllowDependencyInstall: "ask",
			AllowMigrations:        "ask",
			AllowDocker:            true,
		},
		Status: "active",
	}

	if err := SaveSession(workspaceDir, sess); err != nil {
		return nil, fmt.Errorf("failed to save session: %v", err)
	}

	return sess, nil
}

// StartSession creates, validates, and initializes a new session. (Legacy compatible helper)
func StartSession(workspaceDir string, wsYaml *workspace.WorkspaceYaml, goal string, appIDs []string) (*Session, error) {
	if strings.TrimSpace(goal) == "" {
		return nil, fmt.Errorf("session goal cannot be empty")
	}
	if len(appIDs) == 0 {
		return nil, fmt.Errorf("at least one app must be selected")
	}

	// Create a map of registered apps
	appMap := make(map[string]workspace.App)
	for _, app := range wsYaml.Workspace.Apps {
		appMap[app.ID] = app
	}

	var allowedPaths []string
	var selectedAppsClean []string
	var apps []AppContract

	for _, appID := range appIDs {
		trimmedID := strings.TrimSpace(appID)
		if trimmedID == "" {
			continue
		}
		app, exists := appMap[trimmedID]
		if !exists {
			return nil, fmt.Errorf("app '%s' is not registered in the workspace", trimmedID)
		}

		// Resolve app path to absolute representation
		resolvedPath := app.Path
		if !filepath.IsAbs(resolvedPath) {
			resolvedPath = filepath.Join(workspaceDir, app.Path)
		}
		cleanPath := filepath.Clean(resolvedPath)
		allowedPaths = append(allowedPaths, cleanPath)
		selectedAppsClean = append(selectedAppsClean, trimmedID)

		apps = append(apps, AppContract{
			Name:  app.Name,
			Path:  app.Path,
			Stack: app.Stack,
		})
	}

	if len(selectedAppsClean) == 0 {
		return nil, fmt.Errorf("at least one valid app must be selected")
	}

	sessionID := GenerateSessionID()
	sess := &Session{
		ID:           sessionID,
		Goal:         goal,
		SelectedApps: selectedAppsClean,
		CreatedAt:    time.Now().Round(time.Second), // Rounded to second for cleaner YAML
		Apps:         apps,
		Boundary: Boundary{
			AllowedPaths: allowedPaths,
		},
		Agent: AgentContract{
			Provider:      "opencode",
			Mode:          "implementation",
			ContextBudget: 12000,
		},
		Status: "active",
	}

	if err := SaveSession(workspaceDir, sess); err != nil {
		return nil, fmt.Errorf("failed to save session: %v", err)
	}

	return sess, nil
}

// SaveSession writes a Session to .kv/sessions/<session-id>/session.yaml in the workspace.
func SaveSession(workspaceDir string, sess *Session) error {
	sessionDir := filepath.Join(workspaceDir, ".kv", "sessions", sess.ID)
	if err := fsutil.EnsureDir(sessionDir); err != nil {
		return fmt.Errorf("failed to create session directory: %v", err)
	}

	sessionFile := filepath.Join(sessionDir, "session.yaml")
	data, err := yaml.Marshal(sess)
	if err != nil {
		return fmt.Errorf("failed to marshal session: %v", err)
	}

	if err := ioutil.WriteFile(sessionFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write session file: %v", err)
	}

	return nil
}

// LoadSession loads a session from .kv/sessions/<session-id>/session.yaml.
func LoadSession(workspaceDir, sessionID string) (*Session, error) {
	sessionFile := filepath.Join(workspaceDir, ".kv", "sessions", sessionID, "session.yaml")
	data, err := ioutil.ReadFile(sessionFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read session file: %v", err)
	}

	var sess Session
	if err := yaml.Unmarshal(data, &sess); err != nil {
		return nil, fmt.Errorf("failed to parse session yaml: %v", err)
	}

	return &sess, nil
}
