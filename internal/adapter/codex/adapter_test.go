package codex

import (
	"testing"

	"kv/internal/hook"
)

func TestAdapterUsesStableFallbackIdentity(t *testing.T) {
	adapter := New()
	cwd := t.TempDir()
	first, err := adapter.Envelope(Event{CWD: cwd, Name: hook.EventSessionObserved})
	if err != nil {
		t.Fatal(err)
	}
	second, err := adapter.Envelope(Event{CWD: cwd, Name: hook.EventBeforeApply})
	if err != nil || first.Provider.NativeSessionID != "" || first.Provider.CorrelationID == "" || first.Provider.CorrelationID != second.Provider.CorrelationID {
		t.Fatalf("envelopes = %#v, %#v, err = %v", first, second, err)
	}
}
