package policy

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"kv/internal/session"
)

// ValidateCommand checks a command line string against policy rules.
// Returns true if allowed, or false with an error describing the policy violation.
func ValidateCommand(commandLine string, pol session.PolicyContract) (bool, error) {
	cmdClean := strings.TrimSpace(commandLine)

	// 1. Check for .env read
	if !pol.AllowEnvRead && (strings.Contains(cmdClean, ".env") || strings.Contains(cmdClean, "dotenv")) {
		return false, fmt.Errorf("policy violation: reading env files (.env) is blocked")
	}

	// 2. Check for dangerous commands
	dangerous := []string{"sudo", "chmod", "rm -rf", "rm -f", "rm -r"}
	for _, d := range dangerous {
		if strings.Contains(cmdClean, d) {
			return false, fmt.Errorf("policy violation: dangerous command '%s' is blocked", d)
		}
	}

	// 3. Check for dependency installation
	depCommands := []string{"go get", "go install", "npm install", "npm i ", "npm ci", "yarn install", "yarn add", "pip install", "pip3 install", "cargo install", "cargo add"}
	isDepInstall := false
	for _, dc := range depCommands {
		if strings.Contains(cmdClean, dc) {
			isDepInstall = true
			break
		}
	}

	if isDepInstall {
		if pol.AllowDependencyInstall == "false" || pol.AllowDependencyInstall == "nil" {
			return false, fmt.Errorf("policy violation: dependency installation is blocked")
		}
		if pol.AllowDependencyInstall == "ask" || pol.AllowDependencyInstall == "" {
			// Prompt the user if interactive
			fmt.Printf("Policy Alert: Command tries to install dependencies: '%s'\n", cmdClean)
			fmt.Print("Allow dependency installation? (y/N): ")
			reader := bufio.NewReader(os.Stdin)
			response, err := reader.ReadString('\n')
			if err != nil {
				return false, fmt.Errorf("policy warning: blocked dependency install in non-interactive environment")
			}
			response = strings.ToLower(strings.TrimSpace(response))
			if response == "y" || response == "yes" {
				fmt.Println("Dependency installation allowed by user.")
				return true, nil
			}
			return false, fmt.Errorf("policy warning: dependency installation rejected by user")
		}
	}

	// 4. Check for migrations
	migrationCommands := []string{"migrate", "db:migrate", "goose", "golang-migrate"}
	isMigration := false
	for _, mc := range migrationCommands {
		if strings.Contains(cmdClean, mc) {
			isMigration = true
			break
		}
	}

	if isMigration {
		if pol.AllowMigrations == "false" || pol.AllowMigrations == "nil" {
			return false, fmt.Errorf("policy violation: database migrations are blocked")
		}
		if pol.AllowMigrations == "ask" || pol.AllowMigrations == "" {
			fmt.Printf("Policy Alert: Command tries to run migrations: '%s'\n", cmdClean)
			fmt.Print("Allow running migrations? (y/N): ")
			reader := bufio.NewReader(os.Stdin)
			response, err := reader.ReadString('\n')
			if err != nil {
				return false, fmt.Errorf("policy warning: blocked migrations in non-interactive environment")
			}
			response = strings.ToLower(strings.TrimSpace(response))
			if response == "y" || response == "yes" {
				fmt.Println("Migrations allowed by user.")
				return true, nil
			}
			return false, fmt.Errorf("policy warning: migrations rejected by user")
		}
	}

	// 5. Check for docker commands
	if !pol.AllowDocker && (strings.Contains(cmdClean, "docker ") || strings.Contains(cmdClean, "docker-compose ")) {
		return false, fmt.Errorf("policy violation: docker commands are blocked")
	}

	return true, nil
}
