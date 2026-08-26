package opencode

import (
	"fmt"
	"os"
	"path/filepath"

	"kv/internal/fsutil"
)

// Install installs global scripts and commands for OpenCode.
func Install() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("could not resolve user home directory: %v", err)
	}

	configDir := filepath.Join(home, ".config", "opencode")
	fmt.Printf("Config directory: %s\n\n", configDir)

	if err := fsutil.EnsureDir(configDir); err != nil {
		return fmt.Errorf("failed to create config directory %s: %v", configDir, err)
	}
	installedMCP, err := InstallMCP(configDir, "kv")
	if err != nil {
		return fmt.Errorf("failed to install KV MCP server: %v", err)
	}
	if installedMCP {
		fmt.Println("[MERGE] KV MCP server")
	} else {
		fmt.Println("[SKIP] KV MCP server (already configured)")
	}

	// 1. opencode.json (does not overwrite or backup if it already exists)
	jsonPath := filepath.Join(configDir, "opencode.json")
	if !fsutil.Exists(jsonPath) {
		jsonContent := []byte("{\n  \"$schema\": \"https://opencode.ai/config.json\"\n}\n")
		if err := fsutil.WriteFile(jsonPath, jsonContent, 0644); err != nil {
			return fmt.Errorf("failed to write opencode.json: %v", err)
		}
		fmt.Println("[CREATE] opencode.json")
	} else {
		fmt.Println("[SKIP] opencode.json (already exists)")
	}

	// 2. scripts/kv-find.ps1
	scriptsDir := filepath.Join(configDir, "scripts")
	ps1Path := filepath.Join(scriptsDir, "kv-find.ps1")
	backedUp, err := fsutil.WriteWithBackup(ps1Path, []byte(FindScriptTemplate), 0755)
	if err != nil {
		return fmt.Errorf("failed to install kv-find.ps1: %v", err)
	}
	if backedUp {
		fmt.Println("[BACKUP & CREATE] scripts/kv-find.ps1 (backup created at scripts/kv-find.ps1.bak)")
	} else {
		fmt.Println("[CREATE] scripts/kv-find.ps1")
	}

	// 3. Command markdown templates
	commandsDir := filepath.Join(configDir, "commands")
	commands := map[string]string{
		"knowledge-init.md":     InitTemplate,
		"knowledge-start.md":    StartTemplate,
		"knowledge-plan.md":     PlanTemplate,
		"knowledge-migrate.md":  MigrateTemplate,
		"knowledge-validate.md": ValidateTemplate,
		"knowledge-update.md":   UpdateTemplate,
	}

	for filename, content := range commands {
		cmdPath := filepath.Join(commandsDir, filename)
		bup, err := fsutil.WriteWithBackup(cmdPath, []byte(content), 0644)
		if err != nil {
			return fmt.Errorf("failed to install %s: %v", filename, err)
		}
		if bup {
			fmt.Printf("[BACKUP & CREATE] commands/%s (backup created at commands/%s.bak)\n", filename, filename)
		} else {
			fmt.Printf("[CREATE] commands/%s\n", filename)
		}
	}

	fmt.Println("\nOpenCode installation completed successfully.")
	return nil
}

func Uninstall() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("could not resolve user home directory: %v", err)
	}
	removed, err := UninstallMCP(filepath.Join(home, ".config", "opencode"))
	if err != nil {
		return fmt.Errorf("failed to uninstall KV MCP server: %v", err)
	}
	if removed {
		fmt.Println("[REMOVE] KV MCP server")
	} else {
		fmt.Println("[SKIP] KV MCP server (not configured)")
	}
	return nil
}
