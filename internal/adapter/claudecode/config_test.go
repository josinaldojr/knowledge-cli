package claudecode

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"kv/internal/hook"
	"kv/internal/spool"
)

type unavailableClient struct{ fail bool }

func (c *unavailableClient) Handle(context.Context, hook.HookEnvelope) (hook.HookResult, error) {
	if c.fail {
		return hook.HookResult{}, errors.New("unavailable")
	}
	return hook.HookResult{Status: "processed"}, nil
}

func TestInstallUninstallAndSpoolDeliveryPreserveUserConfiguration(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	userConfig := []byte(`{"permissions":{"allow":["Read"]},"mcpServers":{"other":{"command":"other"}}}`)
	if err := os.WriteFile(path, userConfig, 0600); err != nil {
		t.Fatal(err)
	}
	installed, err := InstallMCP(path, "kv-test")
	if err != nil || !installed {
		t.Fatalf("install = %t, %v", installed, err)
	}
	if installed, err := InstallMCP(path, "kv-test"); err != nil || installed {
		t.Fatalf("second install = %t, %v", installed, err)
	}
	contents, err := os.ReadFile(path)
	if err != nil || !hasUserConfiguration(contents) {
		t.Fatalf("user configuration changed: %s, %v", contents, err)
	}
	queue, err := spool.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	client := &unavailableClient{fail: true}
	result, err := NewDispatcher(client, queue).Deliver(context.Background(), hook.HookEnvelope{EventID: "event"})
	if err != nil || !result.Degraded {
		t.Fatalf("delivery = %#v, %v", result, err)
	}
	client.fail = false
	if err := NewDispatcher(client, queue).RetryDue(context.Background(), time.Now().Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	if removed, err := UninstallMCP(path); err != nil || !removed {
		t.Fatalf("uninstall = %t, %v", removed, err)
	}
	contents, err = os.ReadFile(path)
	if err != nil || !hasUserConfiguration(contents) || hasKVEntry(contents) {
		t.Fatalf("uninstall changed user configuration: %s, %v", contents, err)
	}
}

func TestUninstallDoesNotRemoveUserManagedKVEntry(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	contents := []byte(`{"mcpServers":{"kv":{"command":"custom-kv","args":["mcp"]}}}`)
	if err := os.WriteFile(path, contents, 0600); err != nil {
		t.Fatal(err)
	}
	if installed, err := InstallMCP(path, "kv"); err != nil || installed {
		t.Fatalf("install = %t, %v", installed, err)
	}
	if removed, err := UninstallMCP(path); err != nil || removed {
		t.Fatalf("uninstall = %t, %v", removed, err)
	}
	after, err := os.ReadFile(path)
	if err != nil || string(after) != string(contents) {
		t.Fatalf("user entry changed: %s, %v", after, err)
	}
}

func TestUninstallDoesNotRemoveKVEntryCustomizedAfterInstall(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	if installed, err := InstallMCP(path, "kv"); err != nil || !installed {
		t.Fatalf("install = %t, %v", installed, err)
	}
	contents := []byte(`{"mcpServers":{"kv":{"command":"custom-kv","args":["mcp"]}}}`)
	if err := os.WriteFile(path, contents, 0600); err != nil {
		t.Fatal(err)
	}
	if removed, err := UninstallMCP(path); err != nil || removed {
		t.Fatalf("uninstall = %t, %v", removed, err)
	}
	after, err := os.ReadFile(path)
	if err != nil || string(after) != string(contents) {
		t.Fatalf("customized entry changed: %s, %v", after, err)
	}
}

func TestCheckMCPValidatesManualStdioEntry(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	if err := os.WriteFile(path, []byte(`{"mcpServers":{"kv":{"command":"kv","args":["mcp"]}}}`), 0600); err != nil {
		t.Fatal(err)
	}
	report := CheckMCP(path, "go")
	if !report.Binary || !report.MCPConfigured || !report.LifecycleAdapter || !report.SpoolRecovery {
		t.Fatalf("report = %#v", report)
	}
}

func hasUserConfiguration(contents []byte) bool {
	var config map[string]json.RawMessage
	if json.Unmarshal(contents, &config) != nil {
		return false
	}
	var permissions struct {
		Allow []string `json:"allow"`
	}
	var servers map[string]struct {
		Command string `json:"command"`
	}
	return json.Unmarshal(config["permissions"], &permissions) == nil && len(permissions.Allow) == 1 && permissions.Allow[0] == "Read" && json.Unmarshal(config["mcpServers"], &servers) == nil && servers["other"].Command == "other"
}

func hasKVEntry(contents []byte) bool {
	var config struct {
		MCPServers map[string]json.RawMessage `json:"mcpServers"`
	}
	return json.Unmarshal(contents, &config) == nil && config.MCPServers["kv"] != nil
}
