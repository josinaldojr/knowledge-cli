package store

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type MigrationError struct {
	Version int
	Err     error
}

func (e *MigrationError) Error() string { return fmt.Sprintf("migration %d: %v", e.Version, e.Err) }
func (e *MigrationError) Unwrap() error { return e.Err }

type migration struct {
	version    int
	statements []string
}

var migrations = []migration{{version: 1, statements: []string{
	`CREATE TABLE IF NOT EXISTS workspaces (id TEXT PRIMARY KEY, identity TEXT NOT NULL UNIQUE, created_at TEXT NOT NULL)`,
	`CREATE TABLE IF NOT EXISTS workspace_instances (id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL REFERENCES workspaces(id), canonical_path TEXT NOT NULL UNIQUE, created_at TEXT NOT NULL)`,
	`CREATE TABLE IF NOT EXISTS provider_sessions (id TEXT PRIMARY KEY, workspace_instance_id TEXT NOT NULL REFERENCES workspace_instances(id), provider_kind TEXT NOT NULL, provider_session_key TEXT NOT NULL, status TEXT NOT NULL, started_at TEXT NOT NULL, last_seen_at TEXT NOT NULL, UNIQUE(workspace_instance_id, provider_kind, provider_session_key))`,
	`CREATE TABLE IF NOT EXISTS hook_events (event_id TEXT PRIMARY KEY, idempotency_key TEXT NOT NULL UNIQUE, provider_session_id TEXT REFERENCES provider_sessions(id), envelope_json BLOB NOT NULL, result_json BLOB, processed_at TEXT NOT NULL)`,
	`CREATE TABLE IF NOT EXISTS openspec_changes (id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL REFERENCES workspaces(id), change_key TEXT NOT NULL, status TEXT NOT NULL, UNIQUE(workspace_id, change_key))`,
	`CREATE TABLE IF NOT EXISTS operations (id TEXT PRIMARY KEY, change_id TEXT REFERENCES openspec_changes(id), session_id TEXT REFERENCES provider_sessions(id), kind TEXT NOT NULL, status TEXT NOT NULL, created_at TEXT NOT NULL)`,
	`CREATE TABLE IF NOT EXISTS artifact_revisions (id TEXT PRIMARY KEY, change_id TEXT REFERENCES openspec_changes(id), canonical_path TEXT NOT NULL, content_hash TEXT NOT NULL, captured_at TEXT NOT NULL, UNIQUE(change_id, canonical_path, content_hash))`,
	`CREATE TABLE IF NOT EXISTS transformations (id TEXT PRIMARY KEY, operation_id TEXT NOT NULL REFERENCES operations(id), before_revision_id TEXT REFERENCES artifact_revisions(id), after_revision_id TEXT REFERENCES artifact_revisions(id), created_at TEXT NOT NULL)`,
	`CREATE TABLE IF NOT EXISTS transformation_items (id TEXT PRIMARY KEY, transformation_id TEXT NOT NULL REFERENCES transformations(id), kind TEXT NOT NULL, status TEXT NOT NULL, summary TEXT NOT NULL)`,
	`CREATE TABLE IF NOT EXISTS memories (id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL REFERENCES workspaces(id), kind TEXT NOT NULL, status TEXT NOT NULL, summary TEXT NOT NULL, created_at TEXT NOT NULL)`,
	`CREATE TABLE IF NOT EXISTS memory_sources (memory_id TEXT NOT NULL REFERENCES memories(id), artifact_revision_id TEXT NOT NULL REFERENCES artifact_revisions(id), PRIMARY KEY(memory_id, artifact_revision_id))`,
	`CREATE TABLE IF NOT EXISTS diagnostics (id TEXT PRIMARY KEY, operation_id TEXT REFERENCES operations(id), code TEXT NOT NULL, severity TEXT NOT NULL, message TEXT NOT NULL, created_at TEXT NOT NULL)`,
}}, {version: 2, statements: []string{
	`CREATE INDEX IF NOT EXISTS idx_hook_events_idempotency_key ON hook_events(idempotency_key)`,
	`CREATE INDEX IF NOT EXISTS idx_provider_sessions_last_seen_at ON provider_sessions(last_seen_at)`,
	`CREATE INDEX IF NOT EXISTS idx_memories_workspace_status ON memories(workspace_id, status)`,
}}, {version: 3, statements: []string{
	`ALTER TABLE operations ADD COLUMN correlation_id TEXT`,
	`CREATE UNIQUE INDEX IF NOT EXISTS idx_operations_correlation_id ON operations(correlation_id) WHERE correlation_id IS NOT NULL`,
}}, {version: 4, statements: []string{
	`CREATE TABLE IF NOT EXISTS operation_revisions (operation_id TEXT NOT NULL REFERENCES operations(id), artifact_revision_id TEXT NOT NULL REFERENCES artifact_revisions(id), phase TEXT NOT NULL CHECK(phase IN ('before', 'after')), PRIMARY KEY(operation_id, artifact_revision_id, phase))`,
	`CREATE INDEX IF NOT EXISTS idx_operation_revisions_operation_phase ON operation_revisions(operation_id, phase)`,
}}, {version: 5, statements: []string{
	`ALTER TABLE transformations ADD COLUMN summary TEXT NOT NULL DEFAULT ''`,
}}, {version: 6, statements: []string{
	`ALTER TABLE memories ADD COLUMN valid_from TEXT`,
	`ALTER TABLE memories ADD COLUMN valid_to TEXT`,
	`ALTER TABLE memories ADD COLUMN supersedes_memory_id TEXT REFERENCES memories(id)`,
}}, {version: 7, statements: []string{
	`CREATE INDEX IF NOT EXISTS idx_memories_workspace_created ON memories(workspace_id, created_at DESC, id)`,
	`CREATE INDEX IF NOT EXISTS idx_memory_sources_revision ON memory_sources(artifact_revision_id, memory_id)`,
	`CREATE INDEX IF NOT EXISTS idx_artifact_revisions_change ON artifact_revisions(change_id, id)`,
}}, {version: 8, statements: []string{
	`ALTER TABLE artifact_revisions ADD COLUMN logical_type TEXT NOT NULL DEFAULT ''`,
	`ALTER TABLE artifact_revisions ADD COLUMN size_bytes INTEGER NOT NULL DEFAULT 0`,
	`ALTER TABLE artifact_revisions ADD COLUMN artifact_status TEXT NOT NULL DEFAULT ''`,
	`ALTER TABLE artifact_revisions ADD COLUMN parser_version TEXT NOT NULL DEFAULT ''`,
	`ALTER TABLE artifact_revisions ADD COLUMN git_revision TEXT NOT NULL DEFAULT ''`,
}}, {version: 9, statements: []string{
	`CREATE TABLE IF NOT EXISTS diagnostic_sources (diagnostic_id TEXT NOT NULL REFERENCES diagnostics(id), artifact_revision_id TEXT NOT NULL REFERENCES artifact_revisions(id), PRIMARY KEY(diagnostic_id, artifact_revision_id))`,
	`CREATE INDEX IF NOT EXISTS idx_diagnostics_operation ON diagnostics(operation_id, created_at DESC)`,
}}}

func (d *Database) Migrate(ctx context.Context, databasePath string) error {
	if _, err := d.DB.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (version INTEGER PRIMARY KEY, applied_at TEXT NOT NULL)`); err != nil {
		return fmt.Errorf("create migration metadata: %w", err)
	}
	for _, migration := range migrations {
		var applied bool
		if err := d.DB.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version = ?)`, migration.version).Scan(&applied); err != nil {
			return err
		}
		if applied {
			continue
		}
		if err := backup(databasePath); err != nil {
			return &MigrationError{migration.version, err}
		}
		if err := d.WithinTx(ctx, func(tx *sql.Tx) error {
			for _, statement := range migration.statements {
				if _, err := tx.ExecContext(ctx, statement); err != nil {
					return err
				}
			}
			_, err := tx.ExecContext(ctx, `INSERT INTO schema_migrations(version, applied_at) VALUES(?, ?)`, migration.version, time.Now().UTC().Format(time.RFC3339Nano))
			return err
		}); err != nil {
			return &MigrationError{migration.version, err}
		}
	}
	return nil
}

func backup(databasePath string) error {
	info, err := os.Stat(databasePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("stat database for backup: %w", err)
	}
	if info.Size() == 0 {
		return nil
	}
	backupDir := filepath.Join(filepath.Dir(databasePath), "backups")
	if err := os.MkdirAll(backupDir, 0700); err != nil {
		return err
	}
	contents, err := os.ReadFile(databasePath)
	if err != nil {
		return fmt.Errorf("read database for backup: %w", err)
	}
	name := fmt.Sprintf("%s-%s.bak", filepath.Base(databasePath), time.Now().UTC().Format("20060102T150405.000000000Z"))
	return os.WriteFile(filepath.Join(backupDir, name), contents, 0600)
}
