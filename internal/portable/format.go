// Package portable defines the versioned, human-readable global KV exchange format.
package portable

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

const SchemaVersion = 1

type Document struct {
	SchemaVersion int       `json:"schema_version"`
	ExportedAt    time.Time `json:"exported_at"`
	Workspace     Workspace `json:"workspace"`
	Sessions      []Session `json:"sessions"`
	Memories      []Memory  `json:"memories"`
}

type Workspace struct {
	Identity string `json:"identity"`
}

type Session struct {
	ID         string    `json:"id"`
	Provider   string    `json:"provider"`
	Status     string    `json:"status"`
	StartedAt  time.Time `json:"started_at"`
	LastSeenAt time.Time `json:"last_seen_at"`
}

type Memory struct {
	ID           string     `json:"id"`
	Kind         string     `json:"kind"`
	Status       string     `json:"status"`
	Summary      string     `json:"summary"`
	CreatedAt    time.Time  `json:"created_at"`
	ValidFrom    time.Time  `json:"valid_from,omitempty"`
	ValidTo      *time.Time `json:"valid_to,omitempty"`
	SupersedesID string     `json:"supersedes_id,omitempty"`
	Sources      []Source   `json:"sources"`
}

type Source struct {
	ChangeKey      string `json:"change_key"`
	ArtifactPath   string `json:"artifact_path"`
	RevisionHash   string `json:"revision_hash"`
	LogicalType    string `json:"logical_type,omitempty"`
	SizeBytes      int64  `json:"size_bytes,omitempty"`
	ArtifactStatus string `json:"artifact_status,omitempty"`
	ParserVersion  string `json:"parser_version,omitempty"`
	GitRevision    string `json:"git_revision,omitempty"`
}

// Validate rejects unknown format versions and unsafe logical evidence paths
// before an import can modify the global store.
func (d Document) Validate() error {
	if d.SchemaVersion != SchemaVersion {
		return fmt.Errorf("unsupported export schema version %d", d.SchemaVersion)
	}
	if d.ExportedAt.IsZero() || strings.TrimSpace(d.Workspace.Identity) == "" {
		return fmt.Errorf("exported_at and workspace.identity are required")
	}
	if err := uniqueSessions(d.Sessions); err != nil {
		return err
	}
	memoryIDs := make(map[string]struct{}, len(d.Memories))
	for _, memory := range d.Memories {
		if strings.TrimSpace(memory.ID) == "" || strings.TrimSpace(memory.Kind) == "" || strings.TrimSpace(memory.Status) == "" || strings.TrimSpace(memory.Summary) == "" || memory.CreatedAt.IsZero() {
			return fmt.Errorf("memory id, kind, status, summary, and created_at are required")
		}
		if _, exists := memoryIDs[memory.ID]; exists {
			return fmt.Errorf("duplicate memory id %q", memory.ID)
		}
		memoryIDs[memory.ID] = struct{}{}
		if len(memory.Sources) == 0 {
			return fmt.Errorf("memory %q has no evidence sources", memory.ID)
		}
		for _, source := range memory.Sources {
			if strings.TrimSpace(source.ChangeKey) == "" || strings.TrimSpace(source.RevisionHash) == "" || !validLogicalPath(source.ArtifactPath) || source.SizeBytes < 0 {
				return fmt.Errorf("memory %q has invalid evidence source", memory.ID)
			}
		}
	}
	for _, memory := range d.Memories {
		if memory.SupersedesID != "" {
			if _, exists := memoryIDs[memory.SupersedesID]; !exists {
				return fmt.Errorf("memory %q supersedes unknown memory %q", memory.ID, memory.SupersedesID)
			}
		}
	}
	return nil
}

func uniqueSessions(sessions []Session) error {
	ids := make(map[string]struct{}, len(sessions))
	for _, session := range sessions {
		if strings.TrimSpace(session.ID) == "" || strings.TrimSpace(session.Provider) == "" || strings.TrimSpace(session.Status) == "" || session.StartedAt.IsZero() || session.LastSeenAt.IsZero() {
			return fmt.Errorf("session id, provider, status, started_at, and last_seen_at are required")
		}
		if _, exists := ids[session.ID]; exists {
			return fmt.Errorf("duplicate session id %q", session.ID)
		}
		ids[session.ID] = struct{}{}
	}
	return nil
}

func validLogicalPath(value string) bool {
	if strings.TrimSpace(value) == "" || filepath.IsAbs(value) {
		return false
	}
	for _, component := range strings.FieldsFunc(filepath.Clean(value), func(r rune) bool { return r == '/' || r == '\\' }) {
		if component == ".." {
			return false
		}
	}
	return true
}
