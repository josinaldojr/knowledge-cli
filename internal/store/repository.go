package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"

	"kv/internal/hook"
	"kv/internal/portable"
)

// Repository owns IDs for rows created in the global KV store. Callers provide
// stable natural identities; repository-generated UUIDs never cross provider
// or MCP boundaries as required inputs.
type Repository struct{ db *Database }

func NewRepository(db *Database) *Repository { return &Repository{db: db} }

func (r *Repository) EnsureWorkspace(ctx context.Context, identity string) (string, error) {
	if _, err := r.db.DB.ExecContext(ctx, `INSERT OR IGNORE INTO workspaces(id, identity, created_at) VALUES(?, ?, ?)`, uuid.NewString(), identity, now()); err != nil {
		return "", err
	}
	var id string
	err := r.db.DB.QueryRowContext(ctx, `SELECT id FROM workspaces WHERE identity = ?`, identity).Scan(&id)
	return id, err
}

// WorkspaceID resolves an existing global workspace without creating state.
func (r *Repository) WorkspaceID(ctx context.Context, identity string) (string, error) {
	var id string
	err := r.db.DB.QueryRowContext(ctx, `SELECT id FROM workspaces WHERE identity = ?`, identity).Scan(&id)
	return id, err
}

func (r *Repository) EnsureWorkspaceInstance(ctx context.Context, workspaceID, canonicalPath string) (string, error) {
	if _, err := r.db.DB.ExecContext(ctx, `INSERT OR IGNORE INTO workspace_instances(id, workspace_id, canonical_path, created_at) VALUES(?, ?, ?, ?)`, uuid.NewString(), workspaceID, canonicalPath, now()); err != nil {
		return "", err
	}
	var id string
	err := r.db.DB.QueryRowContext(ctx, `SELECT id FROM workspace_instances WHERE canonical_path = ?`, canonicalPath).Scan(&id)
	return id, err
}

func (r *Repository) EnsureProviderSession(ctx context.Context, instanceID string, provider hook.ProviderIdentity) (hook.ProviderSession, error) {
	key := provider.NativeSessionID
	if key == "" {
		key = provider.CorrelationID
	}
	createdAt := now()
	if _, err := r.db.DB.ExecContext(ctx, `INSERT OR IGNORE INTO provider_sessions(id, workspace_instance_id, provider_kind, provider_session_key, status, started_at, last_seen_at) VALUES(?, ?, ?, ?, ?, ?, ?)`, uuid.NewString(), instanceID, provider.Kind, key, hook.SessionActive, createdAt, createdAt); err != nil {
		return hook.ProviderSession{}, err
	}
	var session hook.ProviderSession
	var startedAt, lastSeenAt string
	err := r.db.DB.QueryRowContext(ctx, `SELECT id, status, started_at, last_seen_at FROM provider_sessions WHERE workspace_instance_id = ? AND provider_kind = ? AND provider_session_key = ?`, instanceID, provider.Kind, key).Scan(&session.ID, &session.Status, &startedAt, &lastSeenAt)
	if err != nil {
		return hook.ProviderSession{}, err
	}
	session.StartedAt, err = time.Parse(time.RFC3339Nano, startedAt)
	if err != nil {
		return hook.ProviderSession{}, fmt.Errorf("parse session start time: %w", err)
	}
	session.LastSeenAt, err = time.Parse(time.RFC3339Nano, lastSeenAt)
	session.Provider = provider
	session.Workspace.InstanceID = instanceID
	return session, err
}

func (r *Repository) UpdateSession(ctx context.Context, sessionID string, status hook.SessionStatus) error {
	_, err := r.db.DB.ExecContext(ctx, `UPDATE provider_sessions SET status = ?, last_seen_at = ? WHERE id = ?`, status, now(), sessionID)
	return err
}

func (r *Repository) SessionStatus(ctx context.Context, sessionID string) (hook.SessionStatus, error) {
	var status hook.SessionStatus
	err := r.db.DB.QueryRowContext(ctx, `SELECT status FROM provider_sessions WHERE id = ?`, sessionID).Scan(&status)
	return status, err
}

func (r *Repository) CreateOperation(ctx context.Context, changeID, sessionID, kind, status string) (string, error) {
	id := uuid.NewString()
	_, err := r.db.DB.ExecContext(ctx, `INSERT INTO operations(id, change_id, session_id, kind, status, created_at) VALUES(?, ?, ?, ?, ?, ?)`, id, nullable(changeID), nullable(sessionID), kind, status, now())
	return id, err
}

func (r *Repository) EnsureOpenSpecChange(ctx context.Context, workspaceID, changeKey string) (string, error) {
	if _, err := r.db.DB.ExecContext(ctx, `INSERT OR IGNORE INTO openspec_changes(id, workspace_id, change_key, status) VALUES(?, ?, ?, ?)`, uuid.NewString(), workspaceID, changeKey, "active"); err != nil {
		return "", err
	}
	var id string
	err := r.db.DB.QueryRowContext(ctx, `SELECT id FROM openspec_changes WHERE workspace_id = ? AND change_key = ?`, workspaceID, changeKey).Scan(&id)
	return id, err
}

func (r *Repository) EnsureOperation(ctx context.Context, changeID, sessionID string, operation hook.Operation, status string) (string, string, error) {
	if _, err := r.db.DB.ExecContext(ctx, `INSERT OR IGNORE INTO operations(id, correlation_id, change_id, session_id, kind, status, created_at) VALUES(?, ?, ?, ?, ?, ?, ?)`, uuid.NewString(), operation.ID, nullable(changeID), nullable(sessionID), operation.Kind, status, now()); err != nil {
		return "", "", err
	}
	var id, current string
	err := r.db.DB.QueryRowContext(ctx, `SELECT id, status FROM operations WHERE correlation_id = ?`, operation.ID).Scan(&id, &current)
	return id, current, err
}

func (r *Repository) UpdateOperationStatus(ctx context.Context, operationID, status string) error {
	_, err := r.db.DB.ExecContext(ctx, `UPDATE operations SET status = ? WHERE id = ?`, status, operationID)
	return err
}

// PendingOperations returns unfinished operations for a change, excluding the
// correlation currently being delivered. Callers use it to reconcile a missing
// after hook when a later hook observes the same OpenSpec change.
func (r *Repository) PendingOperations(ctx context.Context, changeID, excludeCorrelationID string) ([]string, error) {
	rows, err := r.db.DB.QueryContext(ctx, `SELECT id FROM operations WHERE change_id = ? AND status = 'pending' AND (correlation_id IS NULL OR correlation_id <> ?) ORDER BY created_at, id`, changeID, excludeCorrelationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var operationIDs []string
	for rows.Next() {
		var operationID string
		if err := rows.Scan(&operationID); err != nil {
			return nil, err
		}
		operationIDs = append(operationIDs, operationID)
	}
	return operationIDs, rows.Err()
}

func (r *Repository) LinkOperationRevision(ctx context.Context, operationID, revisionID, phase string) error {
	_, err := r.db.DB.ExecContext(ctx, `INSERT OR IGNORE INTO operation_revisions(operation_id, artifact_revision_id, phase) VALUES(?, ?, ?)`, operationID, revisionID, phase)
	return err
}

func (r *Repository) OperationHasChanges(ctx context.Context, operationID string) (bool, error) {
	load := func(phase string) (map[string]string, error) {
		rows, err := r.db.DB.QueryContext(ctx, `SELECT canonical_path, content_hash FROM operation_revisions r JOIN artifact_revisions a ON a.id = r.artifact_revision_id WHERE r.operation_id = ? AND r.phase = ?`, operationID, phase)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		result := map[string]string{}
		for rows.Next() {
			var path, hash string
			if err := rows.Scan(&path, &hash); err != nil {
				return nil, err
			}
			result[path] = hash
		}
		return result, rows.Err()
	}
	before, err := load("before")
	if err != nil {
		return false, err
	}
	after, err := load("after")
	if err != nil {
		return false, err
	}
	if len(before) != len(after) {
		return true, nil
	}
	for path, hash := range before {
		if after[path] != hash {
			return true, nil
		}
	}
	return false, nil
}

func (r *Repository) ReconcileStale(ctx context.Context, before time.Time) (sessions, operations int64, err error) {
	sessionResult, err := r.db.DB.ExecContext(ctx, `UPDATE provider_sessions SET status = ? WHERE status = ? AND last_seen_at < ?`, hook.SessionStale, hook.SessionActive, before.UTC().Format(time.RFC3339Nano))
	if err != nil {
		return 0, 0, err
	}
	sessions, err = sessionResult.RowsAffected()
	if err != nil {
		return 0, 0, err
	}
	operationResult, err := r.db.DB.ExecContext(ctx, `UPDATE operations SET status = 'abandoned' WHERE status = 'pending' AND created_at < ?`, before.UTC().Format(time.RFC3339Nano))
	if err != nil {
		return sessions, 0, err
	}
	operations, err = operationResult.RowsAffected()
	return sessions, operations, err
}

func (r *Repository) CreateArtifactRevision(ctx context.Context, changeID, path, hash string) (string, error) {
	return r.CreateArtifactRevisionWithMetadata(ctx, changeID, path, hash, ArtifactRevisionMetadata{})
}

// ArtifactRevisionMetadata preserves immutable manifest provenance without
// persisting artifact content in SQLite.
type ArtifactRevisionMetadata struct {
	LogicalType   string
	SizeBytes     int64
	Status        string
	ParserVersion string
	GitRevision   string
}

type StoredArtifactRevision struct {
	ID            string
	CanonicalPath string
	ContentHash   string
	Metadata      ArtifactRevisionMetadata
}

func (r *Repository) OperationRevisions(ctx context.Context, operationID, phase string) ([]StoredArtifactRevision, error) {
	rows, err := r.db.DB.QueryContext(ctx, `SELECT a.id, a.canonical_path, a.content_hash, a.logical_type, a.size_bytes, a.artifact_status, a.parser_version, a.git_revision FROM operation_revisions o JOIN artifact_revisions a ON a.id = o.artifact_revision_id WHERE o.operation_id = ? AND o.phase = ? ORDER BY a.canonical_path, a.id`, operationID, phase)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var revisions []StoredArtifactRevision
	for rows.Next() {
		var revision StoredArtifactRevision
		if err := rows.Scan(&revision.ID, &revision.CanonicalPath, &revision.ContentHash, &revision.Metadata.LogicalType, &revision.Metadata.SizeBytes, &revision.Metadata.Status, &revision.Metadata.ParserVersion, &revision.Metadata.GitRevision); err != nil {
			return nil, err
		}
		revisions = append(revisions, revision)
	}
	return revisions, rows.Err()
}

func (r *Repository) CreateArtifactRevisionWithMetadata(ctx context.Context, changeID, path, hash string, metadata ArtifactRevisionMetadata) (string, error) {
	var id string
	err := r.db.DB.QueryRowContext(ctx, `SELECT id FROM artifact_revisions WHERE change_id = ? AND canonical_path = ? AND content_hash = ?`, nullable(changeID), path, hash).Scan(&id)
	if err == nil {
		return id, nil
	}
	if err != sql.ErrNoRows {
		return "", err
	}
	id = uuid.NewString()
	_, err = r.db.DB.ExecContext(ctx, `INSERT INTO artifact_revisions(id, change_id, canonical_path, content_hash, captured_at, logical_type, size_bytes, artifact_status, parser_version, git_revision) VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, id, nullable(changeID), path, hash, now(), metadata.LogicalType, metadata.SizeBytes, metadata.Status, metadata.ParserVersion, metadata.GitRevision)
	return id, err
}

type TransformationItem struct {
	Kind    string
	Status  string
	Summary string
}

type MemoryRecord struct {
	Kind         string
	Status       string
	Summary      string
	ValidFrom    time.Time
	SupersedesID string
	RevisionIDs  []string
}

// MemoryCandidate is the evidence-backed projection used by the memory domain.
// It deliberately contains no snapshot content.
type MemoryCandidate struct {
	ID           string
	Kind         string
	Status       string
	Summary      string
	CreatedAt    time.Time
	ValidFrom    time.Time
	ValidTo      *time.Time
	SupersedesID string
	ChangeKey    string
	ArtifactPath string
	RevisionID   string
	RevisionHash string
}

// ListMemoryCandidates returns only records belonging to workspaceID. Sources
// are joined so every result remains traceable to its OpenSpec artifact.
func (r *Repository) ListMemoryCandidates(ctx context.Context, workspaceID string) ([]MemoryCandidate, error) {
	return r.listMemoryCandidates(ctx, `m.workspace_id = ?`, workspaceID)
}

// ListMemoryCandidatesForChange limits consolidation to one OpenSpec change
// while preserving its complete evidence-linked history.
func (r *Repository) ListMemoryCandidatesForChange(ctx context.Context, workspaceID, changeKey string) ([]MemoryCandidate, error) {
	return r.listMemoryCandidates(ctx, `m.workspace_id = ? AND c.change_key = ?`, workspaceID, changeKey)
}

func (r *Repository) listMemoryCandidates(ctx context.Context, condition string, args ...any) ([]MemoryCandidate, error) {
	query := `SELECT m.id, m.kind, m.status, m.summary, m.created_at, m.valid_from, m.valid_to, COALESCE(m.supersedes_memory_id, ''), c.change_key, a.canonical_path, a.id, a.content_hash FROM memories m JOIN memory_sources s ON s.memory_id = m.id JOIN artifact_revisions a ON a.id = s.artifact_revision_id LEFT JOIN openspec_changes c ON c.id = a.change_id WHERE ` + condition + ` ORDER BY m.created_at DESC, m.id, a.canonical_path, a.id`
	rows, err := r.db.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var candidates []MemoryCandidate
	for rows.Next() {
		var item MemoryCandidate
		var createdAt, validFrom string
		var validTo, changeKey sql.NullString
		if err := rows.Scan(&item.ID, &item.Kind, &item.Status, &item.Summary, &createdAt, &validFrom, &validTo, &item.SupersedesID, &changeKey, &item.ArtifactPath, &item.RevisionID, &item.RevisionHash); err != nil {
			return nil, err
		}
		var err error
		if item.CreatedAt, err = time.Parse(time.RFC3339Nano, createdAt); err != nil {
			return nil, fmt.Errorf("parse memory creation time: %w", err)
		}
		if validFrom != "" {
			if item.ValidFrom, err = time.Parse(time.RFC3339Nano, validFrom); err != nil {
				return nil, fmt.Errorf("parse memory validity start: %w", err)
			}
		}
		if validTo.Valid {
			value, err := time.Parse(time.RFC3339Nano, validTo.String)
			if err != nil {
				return nil, fmt.Errorf("parse memory validity end: %w", err)
			}
			item.ValidTo = &value
		}
		item.ChangeKey = changeKey.String
		candidates = append(candidates, item)
	}
	return candidates, rows.Err()
}

// CreateTemporalMemory requires artifact evidence and preserves the validity
// interval and optional supersession relationship for engineering memory.
func (r *Repository) CreateTemporalMemory(ctx context.Context, workspaceID string, record MemoryRecord) (string, error) {
	if len(record.RevisionIDs) == 0 {
		return "", errors.New("memory requires at least one evidence revision")
	}
	id := uuid.NewString()
	validFrom := record.ValidFrom
	if validFrom.IsZero() {
		validFrom = time.Now().UTC()
	}
	err := r.db.WithinTx(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, `INSERT INTO memories(id, workspace_id, kind, status, summary, created_at, valid_from, supersedes_memory_id) VALUES(?, ?, ?, ?, ?, ?, ?, ?)`, id, workspaceID, record.Kind, record.Status, record.Summary, now(), validFrom.Format(time.RFC3339Nano), nullable(record.SupersedesID)); err != nil {
			return err
		}
		for _, revisionID := range record.RevisionIDs {
			if _, err := tx.ExecContext(ctx, `INSERT INTO memory_sources(memory_id, artifact_revision_id) VALUES(?, ?)`, id, revisionID); err != nil {
				return err
			}
		}
		if record.SupersedesID != "" {
			_, err := tx.ExecContext(ctx, `UPDATE memories SET valid_to = ? WHERE id = ?`, validFrom.Format(time.RFC3339Nano), record.SupersedesID)
			return err
		}
		return nil
	})
	return id, err
}

type DiagnosticRecord struct {
	ID           string
	Code         string
	Severity     string
	Message      string
	CreatedAt    time.Time
	ChangeKey    string
	ArtifactPath string
	RevisionID   string
	Status       string
	Derivation   string
}

type WorkspaceStatus struct {
	ID, Identity            string
	ActiveSessions, Changes int
}
type SessionRecord struct {
	ID, Provider, Status  string
	StartedAt, LastSeenAt time.Time
}

// ExportKnowledge produces a portable projection that intentionally omits
// provider-native session keys and physical workspace or snapshot locations.
func (r *Repository) ExportKnowledge(ctx context.Context, workspaceID string) (portable.Document, error) {
	var document portable.Document
	document.SchemaVersion = portable.SchemaVersion
	document.ExportedAt = time.Now().UTC()
	if err := r.db.DB.QueryRowContext(ctx, `SELECT identity FROM workspaces WHERE id = ?`, workspaceID).Scan(&document.Workspace.Identity); err != nil {
		return portable.Document{}, err
	}
	sessions, err := r.CurrentSessions(ctx, workspaceID)
	if err != nil {
		return portable.Document{}, err
	}
	for _, session := range sessions {
		document.Sessions = append(document.Sessions, portable.Session{ID: session.ID, Provider: session.Provider, Status: session.Status, StartedAt: session.StartedAt, LastSeenAt: session.LastSeenAt})
	}
	rows, err := r.db.DB.QueryContext(ctx, `SELECT m.id, m.kind, m.status, m.summary, m.created_at, m.valid_from, m.valid_to, COALESCE(m.supersedes_memory_id, ''), c.change_key, a.canonical_path, a.content_hash, a.logical_type, a.size_bytes, a.artifact_status, a.parser_version, a.git_revision FROM memories m JOIN memory_sources s ON s.memory_id = m.id JOIN artifact_revisions a ON a.id = s.artifact_revision_id JOIN openspec_changes c ON c.id = a.change_id WHERE m.workspace_id = ? ORDER BY m.created_at, m.id, c.change_key, a.canonical_path, a.id`, workspaceID)
	if err != nil {
		return portable.Document{}, err
	}
	defer rows.Close()
	memories := map[string]*portable.Memory{}
	for rows.Next() {
		var id, createdAt, validFrom, supersedesID, changeKey, artifactPath, revisionHash string
		var validTo sql.NullString
		var source portable.Source
		memory := portable.Memory{}
		if err := rows.Scan(&id, &memory.Kind, &memory.Status, &memory.Summary, &createdAt, &validFrom, &validTo, &supersedesID, &changeKey, &artifactPath, &revisionHash, &source.LogicalType, &source.SizeBytes, &source.ArtifactStatus, &source.ParserVersion, &source.GitRevision); err != nil {
			return portable.Document{}, err
		}
		item := memories[id]
		if item == nil {
			created, err := time.Parse(time.RFC3339Nano, createdAt)
			if err != nil {
				return portable.Document{}, fmt.Errorf("parse memory creation time: %w", err)
			}
			memory.ID, memory.CreatedAt, memory.SupersedesID = id, created, supersedesID
			if validFrom != "" {
				memory.ValidFrom, err = time.Parse(time.RFC3339Nano, validFrom)
				if err != nil {
					return portable.Document{}, fmt.Errorf("parse memory validity start: %w", err)
				}
			}
			if validTo.Valid {
				value, err := time.Parse(time.RFC3339Nano, validTo.String)
				if err != nil {
					return portable.Document{}, fmt.Errorf("parse memory validity end: %w", err)
				}
				memory.ValidTo = &value
			}
			document.Memories = append(document.Memories, memory)
			item = &document.Memories[len(document.Memories)-1]
			memories[id] = item
		}
		source.ChangeKey, source.ArtifactPath, source.RevisionHash = changeKey, logicalArtifactPath(artifactPath), revisionHash
		item.Sources = append(item.Sources, source)
	}
	if err := rows.Err(); err != nil {
		return portable.Document{}, err
	}
	return document, document.Validate()
}

// ImportKnowledge validates the complete document before beginning a single
// transaction, so malformed files cannot leave partial global state behind.
func (r *Repository) ImportKnowledge(ctx context.Context, document portable.Document) error {
	if err := document.Validate(); err != nil {
		return err
	}
	return r.db.WithinTx(ctx, func(tx *sql.Tx) error {
		workspaceID := uuid.NewString()
		if _, err := tx.ExecContext(ctx, `INSERT OR IGNORE INTO workspaces(id, identity, created_at) VALUES(?, ?, ?)`, workspaceID, document.Workspace.Identity, now()); err != nil {
			return err
		}
		if err := tx.QueryRowContext(ctx, `SELECT id FROM workspaces WHERE identity = ?`, document.Workspace.Identity).Scan(&workspaceID); err != nil {
			return err
		}
		instanceID := uuid.NewString()
		instancePath := "import:" + document.Workspace.Identity
		if _, err := tx.ExecContext(ctx, `INSERT OR IGNORE INTO workspace_instances(id, workspace_id, canonical_path, created_at) VALUES(?, ?, ?, ?)`, instanceID, workspaceID, instancePath, now()); err != nil {
			return err
		}
		if err := tx.QueryRowContext(ctx, `SELECT id FROM workspace_instances WHERE canonical_path = ?`, instancePath).Scan(&instanceID); err != nil {
			return err
		}
		for _, session := range document.Sessions {
			if _, err := tx.ExecContext(ctx, `INSERT OR IGNORE INTO provider_sessions(id, workspace_instance_id, provider_kind, provider_session_key, status, started_at, last_seen_at) VALUES(?, ?, ?, ?, ?, ?, ?)`, uuid.NewString(), instanceID, session.Provider, "import:"+session.ID, session.Status, session.StartedAt.UTC().Format(time.RFC3339Nano), session.LastSeenAt.UTC().Format(time.RFC3339Nano)); err != nil {
				return err
			}
		}
		memoryIDs := make(map[string]string, len(document.Memories))
		for _, memory := range document.Memories {
			id := uuid.NewString()
			memoryIDs[memory.ID] = id
			if _, err := tx.ExecContext(ctx, `INSERT INTO memories(id, workspace_id, kind, status, summary, created_at, valid_from, valid_to) VALUES(?, ?, ?, ?, ?, ?, ?, ?)`, id, workspaceID, memory.Kind, memory.Status, memory.Summary, memory.CreatedAt.UTC().Format(time.RFC3339Nano), nullableTime(memory.ValidFrom), nullableTimePtr(memory.ValidTo)); err != nil {
				return err
			}
			for _, source := range memory.Sources {
				changeID := uuid.NewString()
				if _, err := tx.ExecContext(ctx, `INSERT OR IGNORE INTO openspec_changes(id, workspace_id, change_key, status) VALUES(?, ?, ?, 'imported')`, changeID, workspaceID, source.ChangeKey); err != nil {
					return err
				}
				if err := tx.QueryRowContext(ctx, `SELECT id FROM openspec_changes WHERE workspace_id = ? AND change_key = ?`, workspaceID, source.ChangeKey).Scan(&changeID); err != nil {
					return err
				}
				revisionID := uuid.NewString()
				if _, err := tx.ExecContext(ctx, `INSERT OR IGNORE INTO artifact_revisions(id, change_id, canonical_path, content_hash, captured_at, logical_type, size_bytes, artifact_status, parser_version, git_revision) VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, revisionID, changeID, source.ArtifactPath, source.RevisionHash, memory.CreatedAt.UTC().Format(time.RFC3339Nano), source.LogicalType, source.SizeBytes, source.ArtifactStatus, source.ParserVersion, source.GitRevision); err != nil {
					return err
				}
				if err := tx.QueryRowContext(ctx, `SELECT id FROM artifact_revisions WHERE change_id = ? AND canonical_path = ? AND content_hash = ?`, changeID, source.ArtifactPath, source.RevisionHash).Scan(&revisionID); err != nil {
					return err
				}
				if _, err := tx.ExecContext(ctx, `INSERT INTO memory_sources(memory_id, artifact_revision_id) VALUES(?, ?)`, id, revisionID); err != nil {
					return err
				}
			}
		}
		for _, memory := range document.Memories {
			if memory.SupersedesID != "" {
				if _, err := tx.ExecContext(ctx, `UPDATE memories SET supersedes_memory_id = ? WHERE id = ?`, memoryIDs[memory.SupersedesID], memoryIDs[memory.ID]); err != nil {
					return err
				}
			}
		}
		return nil
	})
}

func logicalArtifactPath(value string) string {
	if !filepath.IsAbs(value) {
		return value
	}
	parts := strings.Split(filepath.ToSlash(value), "/")
	for index, part := range parts {
		if part == "openspec" {
			return strings.Join(parts[index:], "/")
		}
	}
	return filepath.ToSlash(filepath.Base(value))
}

func nullableTime(value time.Time) any {
	if value.IsZero() {
		return nil
	}
	return value.UTC().Format(time.RFC3339Nano)
}

func nullableTimePtr(value *time.Time) any {
	if value == nil {
		return nil
	}
	return value.UTC().Format(time.RFC3339Nano)
}

type ChangeRecord struct{ Key, Status string }
type TransformationRecord struct {
	ID, ChangeKey, OperationID, OperationKind, OperationStatus, Summary string
	CreatedAt                                                           time.Time
	ArtifactPaths                                                       []string
}

func (r *Repository) GetWorkspaceStatus(ctx context.Context, workspaceID string) (WorkspaceStatus, error) {
	var status WorkspaceStatus
	err := r.db.DB.QueryRowContext(ctx, `SELECT w.id, w.identity, (SELECT COUNT(*) FROM provider_sessions s JOIN workspace_instances i ON i.id = s.workspace_instance_id WHERE i.workspace_id = w.id AND s.status = 'active'), (SELECT COUNT(*) FROM openspec_changes c WHERE c.workspace_id = w.id) FROM workspaces w WHERE w.id = ?`, workspaceID).Scan(&status.ID, &status.Identity, &status.ActiveSessions, &status.Changes)
	return status, err
}
func (r *Repository) CurrentSessions(ctx context.Context, workspaceID string) ([]SessionRecord, error) {
	rows, err := r.db.DB.QueryContext(ctx, `SELECT s.id, s.provider_kind, s.status, s.started_at, s.last_seen_at FROM provider_sessions s JOIN workspace_instances i ON i.id = s.workspace_instance_id WHERE i.workspace_id = ? ORDER BY s.last_seen_at DESC, s.id`, workspaceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var records []SessionRecord
	for rows.Next() {
		var record SessionRecord
		var started, seen string
		if err := rows.Scan(&record.ID, &record.Provider, &record.Status, &started, &seen); err != nil {
			return nil, err
		}
		var err error
		record.StartedAt, err = time.Parse(time.RFC3339Nano, started)
		if err != nil {
			return nil, err
		}
		record.LastSeenAt, err = time.Parse(time.RFC3339Nano, seen)
		if err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	return records, rows.Err()
}
func (r *Repository) ChangeHistory(ctx context.Context, workspaceID string) ([]ChangeRecord, error) {
	rows, err := r.db.DB.QueryContext(ctx, `SELECT change_key, status FROM openspec_changes WHERE workspace_id = ? ORDER BY change_key`, workspaceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var records []ChangeRecord
	for rows.Next() {
		var record ChangeRecord
		if err := rows.Scan(&record.Key, &record.Status); err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	return records, rows.Err()
}
func (r *Repository) TransformationGenealogy(ctx context.Context, workspaceID string) ([]TransformationRecord, error) {
	rows, err := r.db.DB.QueryContext(ctx, `SELECT t.id, c.change_key, o.id, o.kind, o.status, t.summary, t.created_at FROM transformations t JOIN operations o ON o.id = t.operation_id JOIN openspec_changes c ON c.id = o.change_id WHERE c.workspace_id = ? ORDER BY t.created_at, t.id`, workspaceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var records []TransformationRecord
	for rows.Next() {
		var record TransformationRecord
		var created string
		if err := rows.Scan(&record.ID, &record.ChangeKey, &record.OperationID, &record.OperationKind, &record.OperationStatus, &record.Summary, &created); err != nil {
			return nil, err
		}
		var err error
		record.CreatedAt, err = time.Parse(time.RFC3339Nano, created)
		if err != nil {
			return nil, err
		}
		paths, err := r.db.DB.QueryContext(ctx, `SELECT DISTINCT a.canonical_path FROM operation_revisions r JOIN artifact_revisions a ON a.id = r.artifact_revision_id WHERE r.operation_id = ? ORDER BY a.canonical_path`, record.OperationID)
		if err != nil {
			return nil, err
		}
		for paths.Next() {
			var path string
			if err := paths.Scan(&path); err != nil {
				paths.Close()
				return nil, err
			}
			record.ArtifactPaths = append(record.ArtifactPaths, path)
		}
		if err := paths.Close(); err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	return records, rows.Err()
}

// StoreDiagnostics atomically links each derived finding to every after
// snapshot observed for its operation. It stores no artifact content.
func (r *Repository) StoreDiagnostics(ctx context.Context, operationID string, diagnostics []hook.Diagnostic) error {
	if len(diagnostics) == 0 {
		return nil
	}
	return r.db.WithinTx(ctx, func(tx *sql.Tx) error {
		for _, diagnostic := range diagnostics {
			id := uuid.NewString()
			if _, err := tx.ExecContext(ctx, `INSERT INTO diagnostics(id, operation_id, code, severity, message, created_at) VALUES(?, ?, ?, ?, ?, ?)`, id, operationID, diagnostic.Code, diagnostic.Severity, diagnostic.Message, now()); err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, `INSERT INTO diagnostic_sources(diagnostic_id, artifact_revision_id) SELECT ?, artifact_revision_id FROM operation_revisions WHERE operation_id = ? AND phase = 'after'`, id, operationID); err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *Repository) ListDiagnostics(ctx context.Context, workspaceID string) ([]DiagnosticRecord, error) {
	rows, err := r.db.DB.QueryContext(ctx, `SELECT d.id, d.code, d.severity, d.message, d.created_at, c.change_key, a.canonical_path, a.id, c.status FROM diagnostics d JOIN operations o ON o.id = d.operation_id JOIN openspec_changes c ON c.id = o.change_id JOIN diagnostic_sources s ON s.diagnostic_id = d.id JOIN artifact_revisions a ON a.id = s.artifact_revision_id WHERE c.workspace_id = ? ORDER BY d.created_at DESC, d.id, a.canonical_path`, workspaceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var records []DiagnosticRecord
	for rows.Next() {
		var record DiagnosticRecord
		var createdAt string
		if err := rows.Scan(&record.ID, &record.Code, &record.Severity, &record.Message, &createdAt, &record.ChangeKey, &record.ArtifactPath, &record.RevisionID, &record.Status); err != nil {
			return nil, err
		}
		parsed, err := time.Parse(time.RFC3339Nano, createdAt)
		if err != nil {
			return nil, fmt.Errorf("parse diagnostic creation time: %w", err)
		}
		record.CreatedAt = parsed
		record.Derivation = "openspec_snapshot_analysis"
		records = append(records, record)
	}
	return records, rows.Err()
}

var ErrNoOperationChanges = errors.New("operation has no materialized OpenSpec changes")

// CreateTransformation records the verified outcome and all of its concept
// items atomically. Its operation retains the workspace, change, session, and
// before/after revision genealogy.
func (r *Repository) CreateTransformation(ctx context.Context, operationID, summary string, items []TransformationItem) (string, error) {
	changed, err := r.OperationHasChanges(ctx, operationID)
	if err != nil {
		return "", err
	}
	if !changed {
		if err := r.UpdateOperationStatus(ctx, operationID, "no_change"); err != nil {
			return "", err
		}
		return "", ErrNoOperationChanges
	}
	id := uuid.NewString()
	err = r.db.WithinTx(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, `INSERT INTO transformations(id, operation_id, summary, created_at) VALUES(?, ?, ?, ?)`, id, operationID, summary, now()); err != nil {
			return err
		}
		for _, item := range items {
			if _, err := tx.ExecContext(ctx, `INSERT INTO transformation_items(id, transformation_id, kind, status, summary) VALUES(?, ?, ?, ?, ?)`, uuid.NewString(), id, item.Kind, item.Status, item.Summary); err != nil {
				return err
			}
		}
		return nil
	})
	return id, err
}

func (r *Repository) CreateMemory(ctx context.Context, workspaceID, kind, status, summary string, revisionIDs []string) (string, error) {
	id := uuid.NewString()
	err := r.db.WithinTx(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, `INSERT INTO memories(id, workspace_id, kind, status, summary, created_at) VALUES(?, ?, ?, ?, ?, ?)`, id, workspaceID, kind, status, summary, now()); err != nil {
			return err
		}
		for _, revisionID := range revisionIDs {
			if _, err := tx.ExecContext(ctx, `INSERT INTO memory_sources(memory_id, artifact_revision_id) VALUES(?, ?)`, id, revisionID); err != nil {
				return err
			}
		}
		return nil
	})
	return id, err
}

func (r *Repository) FindByIdempotencyKey(ctx context.Context, key string) (hook.HookResult, bool, error) {
	var raw []byte
	err := r.db.DB.QueryRowContext(ctx, `SELECT result_json FROM hook_events WHERE idempotency_key = ?`, key).Scan(&raw)
	if err == sql.ErrNoRows {
		return hook.HookResult{}, false, nil
	}
	if err != nil {
		return hook.HookResult{}, false, err
	}
	var result hook.HookResult
	if err := json.Unmarshal(raw, &result); err != nil {
		return hook.HookResult{}, false, fmt.Errorf("decode stored hook result: %w", err)
	}
	return result, true, nil
}

func (r *Repository) StoreProcessed(ctx context.Context, envelope hook.HookEnvelope, result hook.HookResult) error {
	_, _, err := r.StoreOrGet(ctx, envelope, result)
	return err
}

// StoreOrGet makes duplicate delivery a successful replay of the original
// processing result. It is safe to call before returning an adapter response.
func (r *Repository) StoreOrGet(ctx context.Context, envelope hook.HookEnvelope, result hook.HookResult) (hook.HookResult, bool, error) {
	envelopeJSON, err := json.Marshal(envelope)
	if err != nil {
		return hook.HookResult{}, false, err
	}
	resultJSON, err := json.Marshal(result)
	if err != nil {
		return hook.HookResult{}, false, err
	}
	insert, err := r.db.DB.ExecContext(ctx, `INSERT OR IGNORE INTO hook_events(event_id, idempotency_key, envelope_json, result_json, processed_at) VALUES(?, ?, ?, ?, ?)`, envelope.EventID, envelope.IdempotencyKey, envelopeJSON, resultJSON, now())
	if err != nil {
		return hook.HookResult{}, false, err
	}
	affected, err := insert.RowsAffected()
	if err != nil {
		return hook.HookResult{}, false, err
	}
	if affected == 1 {
		return result, false, nil
	}
	prior, found, err := r.FindByIdempotencyKey(ctx, envelope.IdempotencyKey)
	if err != nil || !found {
		return hook.HookResult{}, false, fmt.Errorf("find duplicate hook event: %w", err)
	}
	return prior, true, nil
}

func now() string { return time.Now().UTC().Format(time.RFC3339Nano) }
func nullable(value string) any {
	if value == "" {
		return nil
	}
	return value
}
