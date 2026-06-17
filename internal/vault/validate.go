package vault

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"kv/internal/fsutil"
)

// Doctor performs diagnostics on the discovered Knowledge Vault.
// Returns true if required checks pass, false if they fail.
func Doctor() (bool, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return false, fmt.Errorf("failed to get current working directory: %v", err)
	}

	fmt.Println("Knowledge Vault Doctor")
	fmt.Println()

	// 1. Discover the vault
	result, err := FindVault(cwd)
	if err != nil {
		fmt.Printf("Workspace: %s\n", cwd)
		fmt.Printf("Marker:    None\n")
		fmt.Printf("Vault:     Not Found\n\n")
		fmt.Println("Required checks:")
		fmt.Println("[FAIL] Knowledge Vault exists")
		fmt.Println("\nStatus: unhealthy (Vault not found)")
		return false, nil
	}

	markerDisplay := "None"
	if result.Source == "marker" {
		markerDisplay = result.MarkerPath
	} else if result.Source == "env" {
		markerDisplay = "KNOWLEDGE_VAULT_PATH env var"
	}

	fmt.Printf("Workspace: %s\n", cwd)
	fmt.Printf("Marker:    %s\n", markerDisplay)
	fmt.Printf("Vault:     %s\n\n", result.Path)

	requiredPassed := true
	hasWarnings := false

	fmt.Println("Required checks:")

	// Check 1: .kv-vault exists
	markerPath := filepath.Join(result.Path, ".kv-vault")
	if fsutil.IsFile(markerPath) {
		fmt.Println("[OK] .kv-vault exists")
	} else {
		fmt.Println("[FAIL] .kv-vault exists")
		requiredPassed = false
	}

	// Check 2 & 3: valid JSON and type is knowledge-vault
	if requiredPassed {
		data, err := ioutil.ReadFile(markerPath)
		if err != nil {
			fmt.Printf("[FAIL] .kv-vault read failed: %v\n", err)
			requiredPassed = false
		} else {
			var meta VaultMetadata
			if err := json.Unmarshal(data, &meta); err != nil {
				fmt.Println("[FAIL] .kv-vault has valid JSON")
				fmt.Println("[FAIL] .kv-vault type is knowledge-vault")
				requiredPassed = false
			} else {
				fmt.Println("[OK] .kv-vault has valid JSON")
				if meta.Type == "knowledge-vault" {
					fmt.Println("[OK] .kv-vault type is knowledge-vault")
				} else {
					fmt.Printf("[FAIL] .kv-vault type is knowledge-vault (found: %s)\n", meta.Type)
					requiredPassed = false
				}
			}
		}
	} else {
		fmt.Println("[FAIL] .kv-vault has valid JSON")
		fmt.Println("[FAIL] .kv-vault type is knowledge-vault")
	}

	if !requiredPassed {
		fmt.Println("\nStatus: unhealthy")
		return false, nil
	}

	// Recommended checks
	fmt.Println("\nRecommended checks:")

	recommendedFiles := []string{
		"README.md",
		"AGENTS.md",
	}

	recommendedDirs := []string{
		"00-inbox",
		"01-global",
		"02-domains",
		"03-projects",
		"04-systems",
		"05-decisions",
		"06-agents",
		"07-runbooks",
		"08-prompts",
		"09-templates",
		"10-references",
	}

	for _, filename := range recommendedFiles {
		filePath := filepath.Join(result.Path, filename)
		if fsutil.IsFile(filePath) {
			fmt.Printf("[OK] %s exists\n", filename)
		} else {
			fmt.Printf("[WARN] %s missing\n", filename)
			hasWarnings = true
		}
	}

	for _, dirname := range recommendedDirs {
		dirPath := filepath.Join(result.Path, dirname)
		if fsutil.IsDir(dirPath) {
			fmt.Printf("[OK] %s exists\n", dirname)
		} else {
			fmt.Printf("[WARN] %s missing\n", dirname)
			hasWarnings = true
		}
	}

	fmt.Println()
	if hasWarnings {
		fmt.Println("Status: usable with warnings")
	} else {
		fmt.Println("Status: healthy")
	}

	return true, nil
}
