package opencode

import (
	"kv/internal/hook"
	"testing"
)

func TestAdapterUsesNativeSessionOrStableLazyCorrelation(t *testing.T) {
	adapter := New()
	native, err := adapter.Envelope(Event{CWD: t.TempDir(), NativeSessionID: "native", Name: hook.EventSessionObserved})
	if err != nil || native.Provider.NativeSessionID != "native" || native.Provider.CorrelationID != "" {
		t.Fatalf("native = %#v, err = %v", native, err)
	}
	cwd := t.TempDir()
	first, err := adapter.Envelope(Event{CWD: cwd, Name: hook.EventSessionObserved})
	if err != nil {
		t.Fatal(err)
	}
	second, err := adapter.Envelope(Event{CWD: cwd, Name: hook.EventBeforeApply})
	if err != nil || first.Provider.CorrelationID == "" || first.Provider.CorrelationID != second.Provider.CorrelationID {
		t.Fatalf("correlations = %q, %q, err = %v", first.Provider.CorrelationID, second.Provider.CorrelationID, err)
	}
}
