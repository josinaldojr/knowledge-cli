package codex

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"context"
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
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte("model = \"gpt\"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	installed, err := InstallMCP(path, "kv-test")
	if err != nil || !installed {
		t.Fatalf("install = %t, %v", installed, err)
	}
	if installed, err := InstallMCP(path, "kv-test"); err != nil || installed {
		t.Fatalf("second install = %t, %v", installed, err)
	}
	contents, _ := os.ReadFile(path)
	if string(contents[:len("model = \"gpt\"\n")]) != "model = \"gpt\"\n" {
		t.Fatalf("user configuration changed: %s", contents)
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
	if removed, err := UninstallMCP(path); err != nil || !removed {
		t.Fatalf("uninstall = %t, %v", removed, err)
	}
	contents, _ = os.ReadFile(path)
	if string(contents) != "model = \"gpt\"\n\n" {
		t.Fatalf("uninstall changed user config: %q", contents)
	}
}
