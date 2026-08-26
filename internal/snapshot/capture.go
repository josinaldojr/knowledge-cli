package snapshot

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"kv/internal/diagnostic"
	"kv/internal/discovery"
	"kv/internal/hook"
	"kv/internal/openspec"
	"kv/internal/store"
	"kv/internal/transform"
)

type Capturer struct {
	snapshots  *Store
	repository *store.Repository
}

// AnalyzeAfter parses immutable after snapshots rather than mutable workspace
// files, preserving the evidence boundary for advisory diagnostics.
func (c *Capturer) AnalyzeAfter(ctx context.Context, operationID string, summary hook.ProviderSummary) ([]hook.Diagnostic, error) {
	revisions, err := c.repository.OperationRevisions(ctx, operationID, "after")
	if err != nil {
		return nil, err
	}
	artifacts := make([]diagnostic.Artifact, 0, len(revisions))
	for _, revision := range revisions {
		contents, err := c.snapshots.Get(revision.ContentHash)
		if err != nil {
			return nil, err
		}
		artifacts = append(artifacts, diagnostic.Artifact{LogicalType: revision.Metadata.LogicalType, Contents: string(contents)})
	}
	return diagnostic.Analyze(artifacts, summary), nil
}

func NewCapturer(snapshots *Store, repository *store.Repository) *Capturer {
	return &Capturer{snapshots: snapshots, repository: repository}
}

func (c *Capturer) CaptureBefore(ctx context.Context, envelope hook.HookEnvelope, changeID, operationID string) ([]hook.ArtifactRevision, error) {
	return c.captureEnvelope(ctx, envelope, changeID, operationID, "before")
}

func (c *Capturer) CaptureAfter(ctx context.Context, envelope hook.HookEnvelope, changeID, operationID string) ([]hook.ArtifactRevision, error) {
	return c.captureEnvelope(ctx, envelope, changeID, operationID, "after")
}

// RecordTransformation derives a verified transformation exclusively from the
// immutable before/after snapshot revisions already linked to an operation.
// Provider claims can improve the readable summary, but never add facts absent
// from the revision delta.
func (c *Capturer) RecordTransformation(ctx context.Context, operationID, workspaceID string, providerSummary hook.ProviderSummary) (*hook.Transformation, error) {
	before, err := c.repository.OperationRevisions(ctx, operationID, "before")
	if err != nil {
		return nil, err
	}
	after, err := c.repository.OperationRevisions(ctx, operationID, "after")
	if err != nil {
		return nil, err
	}
	beforeConcepts, err := c.concepts(before)
	if err != nil {
		return nil, err
	}
	afterConcepts, err := c.concepts(after)
	if err != nil {
		return nil, err
	}
	deltas := transform.CalculateDelta(beforeConcepts, afterConcepts)
	normalized := transform.NormalizeProviderSummary(providerSummary, deltas)
	items := transformationItems(deltas, normalized)
	summary := transformationSummary(normalized, items)
	id, err := c.repository.CreateTransformation(ctx, operationID, summary, items)
	if errors.Is(err, store.ErrNoOperationChanges) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if err := c.createMemories(ctx, workspaceID, deltas, after); err != nil {
		return nil, err
	}
	return &hook.Transformation{ID: id, OperationID: operationID, Summary: summary}, nil
}

func (c *Capturer) concepts(revisions []store.StoredArtifactRevision) ([]transform.Concept, error) {
	var concepts []transform.Concept
	for _, revision := range revisions {
		contents, err := c.snapshots.Get(revision.ContentHash)
		if err != nil {
			return nil, err
		}
		concepts = append(concepts, transform.ParseArtifact(revision.Metadata.LogicalType, string(contents))...)
	}
	return concepts, nil
}

func transformationItems(deltas []transform.Delta, normalized transform.NormalizedSummary) []store.TransformationItem {
	statements := map[string]string{}
	for _, claim := range normalized.Supported {
		statements[claim.Kind+"\x00"+claim.ID+"\x00"+claim.Status] = claim.Statement
	}
	items := make([]store.TransformationItem, 0, len(deltas))
	for _, delta := range deltas {
		if delta.Kind == transform.DeltaUnchanged {
			continue
		}
		concept := delta.After
		if concept == nil {
			concept = delta.Before
		}
		if concept == nil {
			continue
		}
		summary := statements[string(concept.Kind)+"\x00"+concept.ID+"\x00"+string(delta.Kind)]
		if summary == "" {
			summary = concept.Title
		}
		items = append(items, store.TransformationItem{Kind: string(concept.Kind), Status: string(delta.Kind), Summary: summary})
	}
	return items
}

func transformationSummary(normalized transform.NormalizedSummary, items []store.TransformationItem) string {
	var statements []string
	for _, claim := range normalized.Supported {
		if claim.Statement != "" {
			statements = append(statements, claim.Statement)
		}
	}
	if len(statements) > 0 {
		return strings.Join(statements, "; ")
	}
	if len(items) > 0 {
		return "Verified OpenSpec changes: " + strings.Join(itemSummaries(items), "; ")
	}
	return "Verified OpenSpec transformation"
}

func itemSummaries(items []store.TransformationItem) []string {
	result := make([]string, 0, len(items))
	for _, item := range items {
		result = append(result, item.Status+" "+item.Kind+" "+item.Summary)
	}
	return result
}

func (c *Capturer) createMemories(ctx context.Context, workspaceID string, deltas []transform.Delta, after []store.StoredArtifactRevision) error {
	afterByType := map[string][]string{}
	for _, revision := range after {
		afterByType[revision.Metadata.LogicalType] = append(afterByType[revision.Metadata.LogicalType], revision.ID)
	}
	for _, delta := range deltas {
		if delta.Kind == transform.DeltaUnchanged || delta.Kind == transform.DeltaRemoved || delta.After == nil {
			continue
		}
		kind := memoryKind(delta.After.Kind)
		if kind == "" {
			continue
		}
		logicalType := logicalTypeForConcept(delta.After.Kind)
		if _, err := c.repository.CreateTemporalMemory(ctx, workspaceID, store.MemoryRecord{Kind: kind, Status: "current", Summary: delta.After.Title, RevisionIDs: afterByType[logicalType]}); err != nil {
			return err
		}
	}
	return nil
}

func memoryKind(kind transform.ConceptKind) string {
	switch kind {
	case transform.ConceptSection:
		return "scope"
	case transform.ConceptDecision:
		return "decision"
	case transform.ConceptRequirement:
		return "requirement"
	case transform.ConceptTask:
		return "task"
	default:
		return ""
	}
}

func logicalTypeForConcept(kind transform.ConceptKind) string {
	switch kind {
	case transform.ConceptSection:
		return "proposal"
	case transform.ConceptDecision:
		return "design"
	case transform.ConceptRequirement, transform.ConceptScenario:
		return "specs"
	case transform.ConceptTask:
		return "tasks"
	default:
		return ""
	}
}

func (c *Capturer) captureEnvelope(ctx context.Context, envelope hook.HookEnvelope, changeID, operationID, phase string) ([]hook.ArtifactRevision, error) {
	artifacts, err := openspec.Enumerate(ctx, envelope.Workspace.CWD, envelope.Subject.ChangeID)
	if err != nil {
		return nil, err
	}
	git, err := discovery.DiscoverGit(envelope.Workspace.CWD)
	if err != nil {
		return nil, err
	}
	return c.capture(ctx, artifacts, changeID, operationID, phase, git.Head, time.Now())
}

func (c *Capturer) capture(ctx context.Context, artifacts []openspec.Artifact, changeID, operationID, phase, gitRevision string, capturedAt time.Time) ([]hook.ArtifactRevision, error) {
	revisions := make([]hook.ArtifactRevision, 0, len(artifacts))
	for _, artifact := range artifacts {
		manifest, contents, err := Build(artifact, gitRevision, capturedAt)
		if err != nil {
			return nil, fmt.Errorf("build artifact manifest: %w", err)
		}
		if _, err := c.snapshots.Put(manifest, contents); err != nil {
			return nil, err
		}
		id, err := c.repository.CreateArtifactRevisionWithMetadata(ctx, changeID, manifest.CanonicalPath, manifest.ContentHash, store.ArtifactRevisionMetadata{LogicalType: manifest.LogicalType, SizeBytes: manifest.Size, Status: manifest.Status, ParserVersion: manifest.ParserVersion, GitRevision: manifest.GitRevision})
		if err != nil {
			return nil, err
		}
		if err := c.repository.LinkOperationRevision(ctx, operationID, id, phase); err != nil {
			return nil, err
		}
		revisions = append(revisions, hook.ArtifactRevision{ID: id, ArtifactID: manifest.LogicalType, CanonicalPath: manifest.CanonicalPath, ContentHash: manifest.ContentHash, CapturedAt: manifest.CapturedAt})
	}
	return revisions, nil
}
