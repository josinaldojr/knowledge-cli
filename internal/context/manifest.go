package context

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"path/filepath"

	"kv/internal/fsutil"
)

// Manifest represents the structure of context-manifest.json.
type Manifest struct {
	SessionID         string       `json:"session_id"`
	GeneratedAt       string       `json:"generated_at"`
	GlobalContextPath string       `json:"global_context_path"`
	Apps              []AppContext `json:"apps"`
}

// AppContext holds manifest metadata for a single application's context file.
type AppContext struct {
	AppID       string `json:"app_id"`
	ContextPath string `json:"context_path"`
	VaultFile   string `json:"vault_file,omitempty"`
}

// SaveManifest marshals and writes the context manifest to .kv/sessions/<session-id>/context/context-manifest.json.
func SaveManifest(workspaceDir, sessionID string, manifest *Manifest) error {
	contextDir := filepath.Join(workspaceDir, ".kv", "sessions", sessionID, "context")
	if err := fsutil.EnsureDir(contextDir); err != nil {
		return fmt.Errorf("failed to ensure context directory: %v", err)
	}

	manifestPath := filepath.Join(contextDir, "context-manifest.json")
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal context manifest: %v", err)
	}

	if err := ioutil.WriteFile(manifestPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write context manifest: %v", err)
	}

	return nil
}

// LoadManifest reads and unmarshals the context-manifest.json.
func LoadManifest(workspaceDir, sessionID string) (*Manifest, error) {
	manifestPath := filepath.Join(workspaceDir, ".kv", "sessions", sessionID, "context", "context-manifest.json")
	data, err := ioutil.ReadFile(manifestPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read context manifest: %v", err)
	}

	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("failed to parse context manifest json: %v", err)
	}

	return &m, nil
}
