package opencode

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// InstallMCP merges the KV stdio server into a global OpenCode config without
// overwriting other user keys or an existing user-managed KV entry.
func InstallMCP(configDir, binary string) (bool, error) {
	if binary == "" {
		binary = "kv"
	}
	if err := os.MkdirAll(configDir, 0700); err != nil {
		return false, err
	}
	path := filepath.Join(configDir, "opencode.json")
	config := map[string]any{"$schema": "https://opencode.ai/config.json"}
	if contents, err := os.ReadFile(path); err == nil {
		if err := json.Unmarshal(contents, &config); err != nil {
			return false, fmt.Errorf("parse OpenCode config: %w", err)
		}
	} else if !os.IsNotExist(err) {
		return false, err
	}
	mcp, ok := config["mcp"].(map[string]any)
	if !ok {
		mcp = map[string]any{}
		config["mcp"] = mcp
	}
	if _, exists := mcp["kv"]; exists {
		return false, nil
	}
	mcp["kv"] = map[string]any{"type": "local", "command": []string{binary, "mcp"}, "enabled": true}
	contents, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return false, err
	}
	if err := os.WriteFile(path, append(contents, '\n'), 0600); err != nil {
		return false, err
	}
	return true, nil
}

// UninstallMCP removes only the KV MCP entry and preserves all user settings.
func UninstallMCP(configDir string) (bool, error) {
	path := filepath.Join(configDir, "opencode.json")
	contents, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	config := map[string]any{}
	if err := json.Unmarshal(contents, &config); err != nil {
		return false, fmt.Errorf("parse OpenCode config: %w", err)
	}
	mcp, ok := config["mcp"].(map[string]any)
	if !ok {
		return false, nil
	}
	if _, exists := mcp["kv"]; !exists {
		return false, nil
	}
	delete(mcp, "kv")
	if len(mcp) == 0 {
		delete(config, "mcp")
	}
	updated, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return false, err
	}
	if err := os.WriteFile(path, append(updated, '\n'), 0600); err != nil {
		return false, err
	}
	return true, nil
}
