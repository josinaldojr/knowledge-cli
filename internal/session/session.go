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

// Session represents a workspace multi-app session.
type Session struct {
	ID           string    `yaml:"id"`
	Goal         string    `yaml:"goal"`
	SelectedApps []string  `yaml:"selected_apps"`
	CreatedAt    time.Time `yaml:"created_at"`
	Boundary     Boundary  `yaml:"boundary"`
	Status       string    `yaml:"status"`
}

// Boundary holds execution boundary constraints.
type Boundary struct {
	AllowedPaths []string `yaml:"allowed_paths"`
	DeniedPaths  []string `yaml:"denied_paths,omitempty"`
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

// StartSession creates, validates, and initializes a new session.
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
		Boundary: Boundary{
			AllowedPaths: allowedPaths,
		},
		Status:       "active",
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
