package claudecode

import (
	"testing"

	"kv/internal/hook"
)

func TestAdapterForwardsClaudeSessionIDAndUsesStableFallback(t *testing.T) {
	adapter := New()
	native, err := adapter.Envelope(Event{ClaudeSessionID: "claude-native", CWD: t.TempDir(), Name: hook.EventSessionObserved})
	if err != nil || native.Provider.NativeSessionID != "claude-native" || native.Provider.CorrelationID != "" {
		t.Fatalf("native = %#v, err = %v", native, err)
	}
	cwd := t.TempDir()
	first, err := adapter.Envelope(Event{CWD: cwd, Name: hook.EventSessionObserved})
	if err != nil {
		t.Fatal(err)
	}
	second, err := adapter.Envelope(Event{CWD: cwd, Name: hook.EventSessionFinished})
	if err != nil || first.Provider.CorrelationID == "" || first.Provider.CorrelationID != second.Provider.CorrelationID {
		t.Fatalf("fallback correlation = %q, %q, err = %v", first.Provider.CorrelationID, second.Provider.CorrelationID, err)
	}
}
