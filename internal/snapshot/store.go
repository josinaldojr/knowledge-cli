package snapshot

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Store struct {
	root     string
	maxBytes int64
	redact   func([]byte) bool
}

// RetentionPolicy controls garbage collection of snapshot *content*. Immutable
// manifests and all SQLite genealogy remain intact after content is removed.
// ProtectedHashes is useful for callers that must retain pending-operation
// snapshots regardless of age.
type RetentionPolicy struct {
	ContentMaxAge   time.Duration
	ProtectedHashes map[string]struct{}
}

func Open(root string, maxBytes int64, redact func([]byte) bool) (*Store, error) {
	if maxBytes <= 0 {
		return nil, fmt.Errorf("snapshot size limit must be positive")
	}
	if err := os.MkdirAll(filepath.Join(root, "contents"), 0700); err != nil {
		return nil, err
	}
	return &Store{root: root, maxBytes: maxBytes, redact: redact}, nil
}

func (s *Store) Put(manifest Manifest, contents []byte) (string, error) {
	if int64(len(contents)) > s.maxBytes {
		return "", fmt.Errorf("snapshot exceeds %d-byte limit", s.maxBytes)
	}
	if s.redact != nil && s.redact(contents) {
		return "", fmt.Errorf("snapshot rejected by redaction policy")
	}
	if strings.Contains(manifest.ContentHash, "/") || strings.Contains(manifest.ContentHash, "\\") || manifest.ContentHash == "" {
		return "", fmt.Errorf("invalid content hash")
	}
	path := filepath.Join(s.root, "contents", manifest.ContentHash)
	if filepath.Dir(path) != filepath.Join(s.root, "contents") {
		return "", fmt.Errorf("snapshot path escapes store")
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if os.IsExist(err) {
		return path, nil
	}
	if err != nil {
		return "", err
	}
	if _, err := file.Write(contents); err != nil {
		_ = file.Close()
		return "", err
	}
	if err := file.Close(); err != nil {
		return "", err
	}
	return path, nil
}

// Get returns a content-addressed snapshot after validating the caller's hash.
// It never follows a user-controlled path outside the private content directory.
func (s *Store) Get(contentHash string) ([]byte, error) {
	if strings.Contains(contentHash, "/") || strings.Contains(contentHash, "\\") || contentHash == "" {
		return nil, fmt.Errorf("invalid content hash")
	}
	path := filepath.Join(s.root, "contents", contentHash)
	if filepath.Dir(path) != filepath.Join(s.root, "contents") {
		return nil, fmt.Errorf("snapshot path escapes store")
	}
	return os.ReadFile(path)
}

// CollectGarbage removes content-addressed snapshot files older than the
// configured age, except hashes explicitly protected by the caller. It never
// alters manifests or relational records, preserving transformation evidence.
func (s *Store) CollectGarbage(now time.Time, policy RetentionPolicy) (int, error) {
	if policy.ContentMaxAge <= 0 {
		return 0, fmt.Errorf("snapshot retention age must be positive")
	}
	entries, err := os.ReadDir(filepath.Join(s.root, "contents"))
	if err != nil {
		return 0, err
	}
	cutoff := now.Add(-policy.ContentMaxAge)
	removed := 0
	for _, entry := range entries {
		if entry.IsDir() || strings.Contains(entry.Name(), "/") || strings.Contains(entry.Name(), "\\") {
			continue
		}
		if _, protected := policy.ProtectedHashes[entry.Name()]; protected {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			return removed, err
		}
		if !info.ModTime().Before(cutoff) {
			continue
		}
		if err := os.Remove(filepath.Join(s.root, "contents", entry.Name())); err != nil {
			return removed, err
		}
		removed++
	}
	return removed, nil
}
