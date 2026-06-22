package vault

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"kv/internal/fsutil"
	"kv/internal/workspace"
)

// IsValidVault checks if a path contains the vault marker '.kv-vault'
func IsValidVault(path string) bool {
	markerPath := filepath.Join(path, ".kv-vault")
	return fsutil.IsFile(markerPath)
}

// FindResult holds the info about the resolved vault.
type FindResult struct {
	Path       string // Absolute path of the discovered vault
	Source     string // "env", "marker", "self", or "child"
	MarkerPath string // Path to the marker (.knowledge-vault) if Source is "marker"
}

// FindVault finds the Knowledge Vault using the standard precedence:
// 1. Env variable KNOWLEDGE_VAULT_PATH
// 2. File .knowledge-vault in current directory or parent directories
// 3. Current directory itself
// 4. Directory knowledge-vault in current/parent directories
func FindVault(startDir string) (*FindResult, error) {
	// 1. Environment variable wins
	if envVal := os.Getenv("KNOWLEDGE_VAULT_PATH"); envVal != "" {
		candidate, err := fsutil.ResolveAbs(envVal)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve KNOWLEDGE_VAULT_PATH: %v", err)
		}
		if IsValidVault(candidate) {
			return &FindResult{
				Path:   candidate,
				Source: "env",
			}, nil
		}
		return nil, fmt.Errorf("KNOWLEDGE_VAULT_PATH is set but does not point to a valid Knowledge Vault. Expected marker '.kv-vault' at: %s", candidate)
	}

	// Resolve starting directory to absolute
	absStart, err := fsutil.ResolveAbs(startDir)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve starting path: %v", err)
	}

	// 2. Workspace config check (.kv/config.yaml)
	if wsDir, err := workspace.FindWorkspaceDir(absStart); err == nil {
		cfg, err := workspace.LoadConfig(wsDir)
		if err == nil && cfg.VaultPath != "" {
			candidate, err := workspace.ResolvePathSafe(wsDir, cfg.VaultPath)
			if err == nil && IsValidVault(candidate) {
				return &FindResult{
					Path:   candidate,
					Source: "workspace-config",
				}, nil
			}
		}
	}

	current := absStart
	for {
		// 3. Workspace marker: .knowledge-vault
		markerFile := filepath.Join(current, workspace.MarkerFilename)
		if fsutil.IsFile(markerFile) {
			raw, err := workspace.ReadMarker(current)
			if err != nil {
				return nil, fmt.Errorf("failed to read marker file at %s: %v", markerFile, err)
			}
			raw = strings.TrimSpace(raw)
			if raw == "" {
				return nil, fmt.Errorf(".knowledge-vault at %s is empty", markerFile)
			}

			candidate, err := workspace.ResolvePathSafe(current, raw)
			if err != nil {
				return nil, fmt.Errorf("failed to resolve path in marker %s: %v", markerFile, err)
			}

			if IsValidVault(candidate) {
				return &FindResult{
					Path:       candidate,
					Source:     "marker",
					MarkerPath: markerFile,
				}, nil
			}
			return nil, fmt.Errorf(".knowledge-vault points to an invalid Knowledge Vault. Expected marker '.kv-vault' at: %s", candidate)
		}

		// Fallback workspace marker: .kv/config.json (legacy/TS CLI format)
		kvConfigJson := filepath.Join(current, ".kv", "config.json")
		if fsutil.IsFile(kvConfigJson) {
			data, err := ioutil.ReadFile(kvConfigJson)
			if err != nil {
				return nil, fmt.Errorf("failed to read TS config file at %s: %v", kvConfigJson, err)
			}
			var cfg struct {
				VaultPath string `json:"vaultPath"`
			}
			if err := json.Unmarshal(data, &cfg); err != nil {
				return nil, fmt.Errorf("failed to parse TS config JSON at %s: %v", kvConfigJson, err)
			}
			cfg.VaultPath = strings.TrimSpace(cfg.VaultPath)
			if cfg.VaultPath == "" {
				return nil, fmt.Errorf(".kv/config.json at %s is empty or missing vaultPath", kvConfigJson)
			}

			candidate, err := workspace.ResolvePathSafe(current, cfg.VaultPath)
			if err != nil {
				return nil, fmt.Errorf("failed to resolve path in TS config %s: %v", kvConfigJson, err)
			}

			if IsValidVault(candidate) {
				return &FindResult{
					Path:       candidate,
					Source:     "marker",
					MarkerPath: kvConfigJson,
				}, nil
			}
			return nil, fmt.Errorf(".kv/config.json points to an invalid Knowledge Vault. Expected marker '.kv-vault' at: %s", candidate)
		}

		// 3. Current directory itself may be the vault
		if IsValidVault(current) {
			return &FindResult{
				Path:   current,
				Source: "self",
			}, nil
		}

		// 4. Direct child named knowledge-vault
		childCandidate := filepath.Join(current, "knowledge-vault")
		if IsValidVault(childCandidate) {
			return &FindResult{
				Path:   childCandidate,
				Source: "child",
			}, nil
		}

		// Traverse up
		parent := filepath.Dir(current)
		if parent == current {
			break
		}
		current = parent
	}

	// Not found
	return nil, fmt.Errorf("Knowledge Vault not found")
}
