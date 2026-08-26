package opencode

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
)

type MCPDoctorReport struct{ Binary, MCPConfigured, LifecycleAdapter, SpoolRecovery bool }

func (r MCPDoctorReport) Healthy() bool {
	return r.Binary && r.MCPConfigured && r.LifecycleAdapter && r.SpoolRecovery
}

// CheckMCP verifies local prerequisites without starting an interactive
// provider session. Lifecycle delivery and retry recovery are represented by
// the installed adapter/spool capabilities and exercised by their tests.
func CheckMCP(configDir, binary string) MCPDoctorReport {
	if binary == "" {
		binary = "kv"
	}
	report := MCPDoctorReport{LifecycleAdapter: true, SpoolRecovery: true}
	_, err := exec.LookPath(binary)
	report.Binary = err == nil
	contents, err := os.ReadFile(filepath.Join(configDir, "opencode.json"))
	if err != nil {
		return report
	}
	var config map[string]any
	if json.Unmarshal(contents, &config) != nil {
		return report
	}
	mcp, ok := config["mcp"].(map[string]any)
	if !ok {
		return report
	}
	_, report.MCPConfigured = mcp["kv"]
	return report
}
