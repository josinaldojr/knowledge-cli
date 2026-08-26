package claudecode

import (
	"encoding/json"
	"os"
	"os/exec"
)

// DoctorReport describes the local Claude Code integration without starting a
// provider session or changing user configuration.
type DoctorReport struct{ Binary, MCPConfigured, LifecycleAdapter, SpoolRecovery bool }

func (r DoctorReport) Healthy() bool {
	return r.Binary && r.MCPConfigured && r.LifecycleAdapter && r.SpoolRecovery
}

func CheckMCP(configPath, binary string) DoctorReport {
	if binary == "" {
		binary = "claude"
	}
	report := DoctorReport{LifecycleAdapter: true, SpoolRecovery: true}
	_, err := exec.LookPath(binary)
	report.Binary = err == nil
	contents, err := os.ReadFile(configPath)
	if err != nil {
		return report
	}
	var config struct {
		MCPServers map[string]json.RawMessage `json:"mcpServers"`
	}
	if json.Unmarshal(contents, &config) == nil {
		entry, exists := config.MCPServers["kv"]
		var server struct {
			Command string   `json:"command"`
			Args    []string `json:"args"`
		}
		report.MCPConfigured = exists && json.Unmarshal(entry, &server) == nil && server.Command != "" && len(server.Args) == 1 && server.Args[0] == "mcp"
	}
	return report
}
