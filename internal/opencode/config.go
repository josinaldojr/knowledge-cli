package opencode

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"
)

type OpenCodeConfig struct {
	Agent  map[string]interface{} `json:"agent"`
	Agents map[string]interface{} `json:"agents"`
}

// IsAgentDefined checks if the given agent is defined in the workspace's opencode.json
// or in the global ~/.config/opencode/opencode.json, or as a markdown template in
// .opencode/agents/<agentName>.md or ~/.config/opencode/agents/<agentName>.md.
func IsAgentDefined(workspaceDir, agentName string) (bool, error) {
	if agentName == "" {
		return true, nil // default agent is fine
	}

	// 1. Check workspace markdown definition file (.opencode/agents/<agentName>.md)
	wsAgentPath := filepath.Join(workspaceDir, ".opencode", "agents", agentName+".md")
	if info, err := os.Stat(wsAgentPath); err == nil && !info.IsDir() {
		return true, nil
	}

	// 2. Check workspace config (opencode.json)
	wsPath := filepath.Join(workspaceDir, ".opencode", "opencode.json")
	if data, err := ioutil.ReadFile(wsPath); err == nil {
		var cfg OpenCodeConfig
		if err := json.Unmarshal(data, &cfg); err == nil {
			if cfg.Agent != nil {
				if _, exists := cfg.Agent[agentName]; exists {
					return true, nil
				}
			}
			if cfg.Agents != nil {
				if _, exists := cfg.Agents[agentName]; exists {
					return true, nil
				}
			}
		}
	}

	// 3. Check global markdown definition file (~/.config/opencode/agents/<agentName>.md)
	home, err := os.UserHomeDir()
	if err == nil {
		globalAgentPath := filepath.Join(home, ".config", "opencode", "agents", agentName+".md")
		if info, err := os.Stat(globalAgentPath); err == nil && !info.IsDir() {
			return true, nil
		}

		// 4. Check global config (opencode.json)
		globalPath := filepath.Join(home, ".config", "opencode", "opencode.json")
		if data, err := ioutil.ReadFile(globalPath); err == nil {
			var cfg OpenCodeConfig
			if err := json.Unmarshal(data, &cfg); err == nil {
				if cfg.Agent != nil {
					if _, exists := cfg.Agent[agentName]; exists {
						return true, nil
					}
				}
				if cfg.Agents != nil {
					if _, exists := cfg.Agents[agentName]; exists {
						return true, nil
					}
				}
			}
		}
	}

	return false, nil
}
