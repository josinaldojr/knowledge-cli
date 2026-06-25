package policy

import (
	"testing"

	"kv/internal/session"
)

func TestValidateCommand(t *testing.T) {
	pol := session.PolicyContract{
		AllowEnvRead:           false,
		AllowDependencyInstall: "false",
		AllowMigrations:        "false",
		AllowDocker:            false,
	}

	// 1. Check blocking `.env` reading
	_, err := ValidateCommand("cat .env", pol)
	if err == nil {
		t.Error("expected error for reading .env, got nil")
	}

	// 2. Check blocking dangerous command
	_, err = ValidateCommand("sudo rm -rf /", pol)
	if err == nil {
		t.Error("expected error for sudo rm -rf, got nil")
	}

	_, err = ValidateCommand("chmod -R 777 .", pol)
	if err == nil {
		t.Error("expected error for chmod, got nil")
	}

	// 3. Check blocking dependency install
	_, err = ValidateCommand("npm install express", pol)
	if err == nil {
		t.Error("expected error for npm install, got nil")
	}

	// 4. Check blocking docker
	_, err = ValidateCommand("docker run redis", pol)
	if err == nil {
		t.Error("expected error for docker run, got nil")
	}

	// 5. Allowed command
	allowed, err := ValidateCommand("echo hello", pol)
	if err != nil || !allowed {
		t.Errorf("expected command 'echo hello' to be allowed, got error: %v", err)
	}
}
