package opencode

import (
	"fmt"
	"os"
	"path/filepath"

	"kv/internal/fsutil"
)

// Doctor checks the status of the OpenCode commands and script installation.
// Returns true if healthy, false otherwise.
func Doctor() (bool, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return false, fmt.Errorf("could not resolve user home directory: %v", err)
	}

	configDir := filepath.Join(home, ".config", "opencode")
	fmt.Printf("OpenCode Doctor\n\n")
	fmt.Printf("Config dir: %s\n\n", configDir)
	fmt.Printf("Checks:\n")

	healthy := true

	// Helper to print check results
	check := func(label string, condition bool) {
		if condition {
			fmt.Printf("[OK] %s\n", label)
		} else {
			fmt.Printf("[FAIL] %s\n", label)
			healthy = false
		}
	}

	// 1. Config dir
	check("opencode config dir exists", fsutil.IsDir(configDir))

	// 2. opencode.json
	jsonPath := filepath.Join(configDir, "opencode.json")
	check("opencode.json exists", fsutil.IsFile(jsonPath))

	// 3. Scripts
	scriptsPath := filepath.Join(configDir, "scripts", "kv-find.ps1")
	check("scripts/kv-find.ps1 exists", fsutil.IsFile(scriptsPath))

	// 4. Commands
	cmdFiles := []string{
		"knowledge-init.md",
		"knowledge-start.md",
		"knowledge-plan.md",
		"knowledge-migrate.md",
		"knowledge-validate.md",
		"knowledge-update.md",
	}

	for _, filename := range cmdFiles {
		cmdPath := filepath.Join(configDir, "commands", filename)
		check(fmt.Sprintf("commands/%s exists", filename), fsutil.IsFile(cmdPath))
	}

	fmt.Println()
	if healthy {
		fmt.Println("Status: healthy")
	} else {
		fmt.Println("Status: unhealthy")
	}

	return healthy, nil
}
