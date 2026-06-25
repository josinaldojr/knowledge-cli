package session

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"kv/internal/fsutil"
)

// LogEvent writes a structured audit event in JSONL format to the session's audit log file.
func LogEvent(workspaceDir, sessionID, eventType string, extra map[string]interface{}) error {
	sessionDir := filepath.Join(workspaceDir, ".kv", "sessions", sessionID)
	if err := fsutil.EnsureDir(sessionDir); err != nil {
		return fmt.Errorf("failed to ensure session directory: %w", err)
	}
	auditFile := filepath.Join(sessionDir, "audit.jsonl")

	event := map[string]interface{}{
		"type": eventType,
		"at":   time.Now().Format(time.RFC3339),
	}
	for k, v := range extra {
		event[k] = v
	}

	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal audit event: %w", err)
	}

	f, err := os.OpenFile(auditFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open audit file: %w", err)
	}
	defer f.Close()

	if _, err := f.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("failed to write to audit file: %w", err)
	}
	return nil
}
