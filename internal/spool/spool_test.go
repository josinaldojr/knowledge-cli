package spool

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"kv/internal/hook"
)

func TestSpoolRecoversRetriesAndAcknowledges(t *testing.T) {
	dir := t.TempDir()
	spool, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	entry := Entry{Envelope: hook.HookEnvelope{EventID: "event"}, RetryAt: now}
	if err := spool.Enqueue(entry); err != nil {
		t.Fatal(err)
	}
	spool, err = Open(dir) // recovery after process restart
	if err != nil {
		t.Fatal(err)
	}
	due, err := spool.Due(now)
	if err != nil || len(due) != 1 {
		t.Fatalf("due = %#v, %v", due, err)
	}
	if err := spool.Retry(due[0], now); err != nil {
		t.Fatal(err)
	}
	due, err = spool.Due(now)
	if err != nil || len(due) != 0 {
		t.Fatalf("retried entries are due too soon: %#v, %v", due, err)
	}
	if err := spool.Acknowledge("event"); err != nil {
		t.Fatal(err)
	}
}

func TestSpoolQuarantinesCorruptEntries(t *testing.T) {
	dir := t.TempDir()
	spool, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "bad.json"), []byte("not json"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := spool.Due(time.Now()); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "bad.json.corrupt")); err != nil {
		t.Fatal(err)
	}
}
