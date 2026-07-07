package opencode

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
)

func TestIsAgentDefined(t *testing.T) {
	tmpDir, err := ioutil.TempDir("", "opencode-config-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Test with empty agent (defaults)
	defined, err := IsAgentDefined(tmpDir, "")
	if err != nil || !defined {
		t.Errorf("expected empty agent to be defined and no error, got defined=%v, err=%v", defined, err)
	}

	// Test missing config file
	defined, err = IsAgentDefined(tmpDir, "backend")
	if err != nil || defined {
		t.Errorf("expected undefined agent and no error, got defined=%v, err=%v", defined, err)
	}

	// Create workspace opencode.json
	opencodeDir := filepath.Join(tmpDir, ".opencode")
	if err := os.MkdirAll(opencodeDir, 0755); err != nil {
		t.Fatalf("failed to create dir: %v", err)
	}
	configContent := []byte(`{
		"agents": {
			"backend": {
				"file": ".opencode/agents/backend.md"
			}
		}
	}`)
	if err := ioutil.WriteFile(filepath.Join(opencodeDir, "opencode.json"), configContent, 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	// Test defined agent
	defined, err = IsAgentDefined(tmpDir, "backend")
	if err != nil || !defined {
		t.Errorf("expected backend agent to be defined, got defined=%v, err=%v", defined, err)
	}

	// Test undefined agent with config file present
	defined, err = IsAgentDefined(tmpDir, "frontend")
	if err != nil || defined {
		t.Errorf("expected frontend agent to be undefined, got defined=%v, err=%v", defined, err)
	}

	// Test defined agent via singular "agent" key in opencode.json
	configContentSingular := []byte(`{
		"agent": {
			"frontend": {
				"model": "gpt-4"
			}
		}
	}`)
	if err := ioutil.WriteFile(filepath.Join(opencodeDir, "opencode.json"), configContentSingular, 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	defined, err = IsAgentDefined(tmpDir, "frontend")
	if err != nil || !defined {
		t.Errorf("expected frontend agent to be defined via singular key, got defined=%v, err=%v", defined, err)
	}

	// Test defined agent via .opencode/agents/<agentName>.md file
	// Clear the opencode.json to ensure it's not matching from there
	if err := ioutil.WriteFile(filepath.Join(opencodeDir, "opencode.json"), []byte("{}"), 0644); err != nil {
		t.Fatalf("failed to clear config file: %v", err)
	}

	agentsDir := filepath.Join(opencodeDir, "agents")
	if err := os.MkdirAll(agentsDir, 0755); err != nil {
		t.Fatalf("failed to create agents dir: %v", err)
	}

	if err := ioutil.WriteFile(filepath.Join(agentsDir, "architect.md"), []byte("architect prompt"), 0644); err != nil {
		t.Fatalf("failed to write agent markdown: %v", err)
	}

	defined, err = IsAgentDefined(tmpDir, "architect")
	if err != nil || !defined {
		t.Errorf("expected architect agent to be defined via markdown file, got defined=%v, err=%v", defined, err)
	}
}
