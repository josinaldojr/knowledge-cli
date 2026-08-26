package claudecode

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type managedConfig struct {
	Hash string `json:"hash"`
}

// InstallMCP adds KV only when the user has not already configured an entry.
// It is intentionally not CLI-wired: Claude Code configuration remains manual
// per the provider capability matrix.
func InstallMCP(configPath, binary string) (bool, error) {
	if binary == "" {
		binary = "kv"
	}
	contents, err := os.ReadFile(configPath)
	if err != nil && !os.IsNotExist(err) {
		return false, err
	}
	config := map[string]json.RawMessage{}
	if len(contents) > 0 && json.Unmarshal(contents, &config) != nil {
		return false, fmt.Errorf("parse Claude Code settings")
	}
	servers := map[string]json.RawMessage{}
	if raw := config["mcpServers"]; len(raw) > 0 && json.Unmarshal(raw, &servers) != nil {
		return false, fmt.Errorf("parse Claude Code mcpServers")
	}
	if servers == nil {
		servers = map[string]json.RawMessage{}
	}
	if _, exists := servers["kv"]; exists {
		return false, nil
	}
	entry := json.RawMessage(fmt.Sprintf(`{"command":%q,"args":["mcp"]}`, binary))
	servers["kv"] = entry
	encodedServers, err := json.Marshal(servers)
	if err != nil {
		return false, err
	}
	config["mcpServers"] = encodedServers
	encoded, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return false, err
	}
	if err := os.MkdirAll(filepath.Dir(configPath), 0700); err != nil {
		return false, err
	}
	if err := os.WriteFile(configPath, append(encoded, '\n'), 0600); err != nil {
		return false, err
	}
	return true, writeManagedConfig(configPath, entry)
}

// UninstallMCP removes a KV entry only when the ownership sidecar proves KV
// created it and its value has not subsequently been customized by the user.
func UninstallMCP(configPath string) (bool, error) {
	managed, err := readManagedConfig(configPath)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	contents, err := os.ReadFile(configPath)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	config := map[string]json.RawMessage{}
	if json.Unmarshal(contents, &config) != nil {
		return false, fmt.Errorf("parse Claude Code settings")
	}
	servers := map[string]json.RawMessage{}
	if raw := config["mcpServers"]; json.Unmarshal(raw, &servers) != nil {
		return false, fmt.Errorf("parse Claude Code mcpServers")
	}
	entry, exists := servers["kv"]
	if !exists || managed.Hash != configHash(entry) {
		return false, nil
	}
	delete(servers, "kv")
	encodedServers, err := json.Marshal(servers)
	if err != nil {
		return false, err
	}
	config["mcpServers"] = encodedServers
	encoded, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return false, err
	}
	if err := os.WriteFile(configPath, append(encoded, '\n'), 0600); err != nil {
		return false, err
	}
	if err := os.Remove(managedConfigPath(configPath)); err != nil {
		return false, err
	}
	return true, nil
}

func managedConfigPath(configPath string) string { return configPath + ".kv-managed" }

func writeManagedConfig(configPath string, entry json.RawMessage) error {
	contents, err := json.Marshal(managedConfig{Hash: configHash(entry)})
	if err != nil {
		return err
	}
	return os.WriteFile(managedConfigPath(configPath), append(contents, '\n'), 0600)
}

func readManagedConfig(configPath string) (managedConfig, error) {
	contents, err := os.ReadFile(managedConfigPath(configPath))
	if err != nil {
		return managedConfig{}, err
	}
	var managed managedConfig
	if err := json.Unmarshal(contents, &managed); err != nil || managed.Hash == "" {
		return managedConfig{}, fmt.Errorf("parse KV-managed Claude Code configuration")
	}
	return managed, nil
}

func configHash(entry json.RawMessage) string {
	var normalized bytes.Buffer
	if json.Compact(&normalized, entry) != nil {
		return ""
	}
	hash := sha256.Sum256(normalized.Bytes())
	return hex.EncodeToString(hash[:])
}
