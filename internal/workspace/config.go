package workspace

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

const ConfigDirName = ".kv"
const ConfigFileName = "config.yaml"

// Config represents the workspace configuration.
type Config struct {
	VaultPath string `yaml:"vault_path"`
}

// FindWorkspaceDir traverses up from the startDir to find the directory containing the .kv/config.yaml file.
func FindWorkspaceDir(startDir string) (string, error) {
	current := filepath.Clean(startDir)
	for {
		configPath := filepath.Join(current, ConfigDirName, ConfigFileName)
		if info, err := os.Stat(configPath); err == nil && !info.IsDir() {
			return current, nil
		}
		parent := filepath.Dir(current)
		if parent == current {
			break
		}
		current = parent
	}
	return "", fmt.Errorf("workspace not initialized (.kv/config.yaml not found in parent directories)")
}

// LoadConfig loads the Config from .kv/config.yaml in the workspace directory.
func LoadConfig(workspaceDir string) (*Config, error) {
	configPath := filepath.Join(workspaceDir, ConfigDirName, ConfigFileName)
	data, err := ioutil.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %v", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse yaml config: %v", err)
	}

	return &cfg, nil
}

// SaveConfig writes the Config to .kv/config.yaml in the workspace directory.
func SaveConfig(workspaceDir string, cfg *Config) error {
	configDir := filepath.Join(workspaceDir, ConfigDirName)
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %v", err)
	}

	configPath := filepath.Join(configDir, ConfigFileName)
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %v", err)
	}

	if err := ioutil.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %v", err)
	}

	return nil
}
