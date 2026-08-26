// Package spool durably buffers lifecycle envelopes when KV is unavailable.
package spool

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"kv/internal/hook"
)

type Entry struct {
	Envelope hook.HookEnvelope `json:"envelope"`
	Attempts int               `json:"attempts"`
	RetryAt  time.Time         `json:"retry_at"`
}

type Spool struct{ dir string }

func Open(dir string) (*Spool, error) {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	return &Spool{dir: dir}, nil
}

func (s *Spool) Enqueue(entry Entry) error {
	if entry.Envelope.EventID == "" {
		return fmt.Errorf("spool entry requires event_id")
	}
	data, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	temporary := filepath.Join(s.dir, entry.Envelope.EventID+".tmp")
	final := filepath.Join(s.dir, entry.Envelope.EventID+".json")
	if err := os.WriteFile(temporary, data, 0600); err != nil {
		return err
	}
	return os.Rename(temporary, final)
}

func (s *Spool) Due(now time.Time) ([]Entry, error) {
	files, err := filepath.Glob(filepath.Join(s.dir, "*.json"))
	if err != nil {
		return nil, err
	}
	var entries []Entry
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			return nil, err
		}
		var entry Entry
		if err := json.Unmarshal(data, &entry); err != nil {
			if err := os.Rename(file, file+".corrupt"); err != nil {
				return nil, err
			}
			continue
		}
		if !entry.RetryAt.After(now) {
			entries = append(entries, entry)
		}
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].RetryAt.Before(entries[j].RetryAt) })
	return entries, nil
}

// Entries returns every valid queued event, including entries delayed by
// backoff. Corrupt entries are quarantined just as they are during delivery.
func (s *Spool) Entries() ([]Entry, error) {
	files, err := filepath.Glob(filepath.Join(s.dir, "*.json"))
	if err != nil {
		return nil, err
	}
	var entries []Entry
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			return nil, err
		}
		var entry Entry
		if err := json.Unmarshal(data, &entry); err != nil {
			if err := os.Rename(file, file+".corrupt"); err != nil {
				return nil, err
			}
			continue
		}
		entries = append(entries, entry)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].RetryAt.Before(entries[j].RetryAt) })
	return entries, nil
}

func (s *Spool) Acknowledge(eventID string) error {
	return removeIfExists(filepath.Join(s.dir, eventID+".json"))
}

func (s *Spool) Retry(entry Entry, now time.Time) error {
	entry.Attempts++
	entry.RetryAt = now.Add(time.Second * time.Duration(1<<min(entry.Attempts, 8)))
	return s.Enqueue(entry)
}

func removeIfExists(path string) error {
	err := os.Remove(path)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
