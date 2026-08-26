package codex

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const managedConfig = "# Managed by KV; remove with kv codex uninstall\n[mcp_servers.kv]\ncommand = \"%s\"\nargs = [\"mcp\"]\n"

// InstallMCP appends only KV's dedicated TOML table to Codex's user config.
// Existing configuration is left byte-for-byte intact and an existing KV table
// is considered user-managed rather than overwritten.
func InstallMCP(configPath, binary string) (bool, error) {
	if binary == "" {
		binary = "kv"
	}
	if err := os.MkdirAll(filepath.Dir(configPath), 0700); err != nil {
		return false, err
	}
	contents, err := os.ReadFile(configPath)
	if err != nil && !os.IsNotExist(err) {
		return false, err
	}
	if strings.Contains(string(contents), "[mcp_servers.kv]") {
		return false, nil
	}
	addition := fmt.Sprintf(managedConfig, strings.ReplaceAll(binary, "\"", "\\\""))
	if len(contents) > 0 && contents[len(contents)-1] != '\n' {
		contents = append(contents, '\n')
	}
	if err := os.WriteFile(configPath, append(contents, []byte("\n"+addition)...), 0600); err != nil {
		return false, err
	}
	return true, nil
}

// UninstallMCP removes only the exact KV-owned table. A user-created KV table
// without the managed marker is never modified.
func UninstallMCP(configPath string) (bool, error) {
	contents, err := os.ReadFile(configPath)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	start := strings.Index(string(contents), "# Managed by KV; remove with kv codex uninstall\n[mcp_servers.kv]\n")
	if start < 0 {
		return false, nil
	}
	rest := string(contents[start:])
	args := strings.Index(rest, "args = [")
	if args < 0 {
		return false, fmt.Errorf("invalid KV-managed Codex configuration")
	}
	newline := strings.Index(rest[args:], "\n")
	if newline < 0 {
		return false, fmt.Errorf("invalid KV-managed Codex configuration")
	}
	end := start + args + newline + 1
	updated := append(append([]byte{}, contents[:start]...), contents[end:]...)
	if err := os.WriteFile(configPath, updated, 0600); err != nil {
		return false, err
	}
	return true, nil
}

type DoctorReport struct{ Binary, MCPConfigured, LifecycleAdapter, SpoolRecovery bool }

func (r DoctorReport) Healthy() bool {
	return r.Binary && r.MCPConfigured && r.LifecycleAdapter && r.SpoolRecovery
}

func CheckMCP(configPath, binary string) DoctorReport {
	if binary == "" {
		binary = "kv"
	}
	report := DoctorReport{LifecycleAdapter: true, SpoolRecovery: true}
	_, err := exec.LookPath(binary)
	report.Binary = err == nil
	contents, err := os.ReadFile(configPath)
	report.MCPConfigured = err == nil && strings.Contains(string(contents), "[mcp_servers.kv]")
	return report
}
