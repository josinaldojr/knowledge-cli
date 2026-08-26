package lifecycle

import (
	"context"
	"strings"
	"time"

	"kv/internal/hook"
	"kv/internal/memory"
	"kv/internal/store"
)

type HookApplicationService struct {
	sessions     *SessionService
	repository   *store.Repository
	capturer     BeforeCapturer
	retriever    MemoryRetriever
	consolidator ArchiveConsolidator
}

type MemoryRetriever interface {
	Retrieve(context.Context, memory.Query) ([]memory.Result, error)
}

// ArchiveConsolidator creates the evidence-backed current view of a completed
// OpenSpec change while retaining its historical transformations.
type ArchiveConsolidator interface {
	Consolidate(context.Context, string, string) (memory.ConsolidatedView, error)
}

type AfterDiagnosticsAnalyzer interface {
	AnalyzeAfter(context.Context, string, hook.ProviderSummary) ([]hook.Diagnostic, error)
}

// TransformationRecorder persists a verified delta after snapshots have been
// captured. It is optional to retain the core lifecycle service testable with
// lightweight capturers.
type TransformationRecorder interface {
	RecordTransformation(context.Context, string, string, hook.ProviderSummary) (*hook.Transformation, error)
}

type BeforeCapturer interface {
	CaptureBefore(context.Context, hook.HookEnvelope, string, string) ([]hook.ArtifactRevision, error)
	CaptureAfter(context.Context, hook.HookEnvelope, string, string) ([]hook.ArtifactRevision, error)
}

func NewHookApplicationService(sessions *SessionService, repository *store.Repository, capturers ...BeforeCapturer) *HookApplicationService {
	service := &HookApplicationService{sessions: sessions, repository: repository}
	if len(capturers) > 0 {
		service.capturer = capturers[0]
	}
	return service
}

// WithMemoryRetriever attaches the optional pre-operation assistance service.
// It is separate from construction to retain compatibility with existing callers.
func (s *HookApplicationService) WithMemoryRetriever(retriever MemoryRetriever) *HookApplicationService {
	s.retriever = retriever
	return s
}

// WithArchiveConsolidator enables advisory consolidation after an archive
// hook. Consolidation failures are reported as degraded assistance and never
// invalidate an already-materialized OpenSpec archive.
func (s *HookApplicationService) WithArchiveConsolidator(consolidator ArchiveConsolidator) *HookApplicationService {
	s.consolidator = consolidator
	return s
}

func (s *HookApplicationService) Handle(ctx context.Context, envelope hook.HookEnvelope) (hook.HookResult, error) {
	if err := envelope.Validate(time.Now().UTC()); err != nil {
		return hook.HookResult{}, err
	}
	if result, found, err := s.repository.FindByIdempotencyKey(ctx, envelope.IdempotencyKey); err != nil || found {
		return result, err
	}
	session, err := s.sessions.EnsureSession(ctx, envelope.Workspace.CWD, envelope.Provider)
	if err != nil {
		return hook.HookResult{}, err
	}
	result := hook.HookResult{SessionID: session.ID, Status: "processed"}
	if strings.HasPrefix(string(envelope.Event), "openspec.") {
		changeID, err := s.repository.EnsureOpenSpecChange(ctx, session.Workspace.LogicalID, envelope.Subject.ChangeID)
		if err != nil {
			return hook.HookResult{}, err
		}
		if s.capturer != nil {
			reconciled, err := s.reconcilePending(ctx, envelope, changeID)
			if err != nil {
				return hook.HookResult{}, err
			}
			if reconciled > 0 {
				result.Diagnostics = append(result.Diagnostics, hook.Diagnostic{Code: "missing_after_hook_reconciled", Severity: "info", Message: "reconciled pending OpenSpec operations from the current artifact state"})
			}
		}
		status := "pending"
		if strings.HasPrefix(string(envelope.Event), "openspec.after.") {
			status = "completed"
			if envelope.Subject.NoChange {
				status = "no_change"
			}
		}
		operationID, current, err := s.repository.EnsureOperation(ctx, changeID, session.ID, envelope.Subject.Operation, status)
		if err != nil {
			return hook.HookResult{}, err
		}
		if strings.HasPrefix(string(envelope.Event), "openspec.after.") && current == "pending" {
			if err := s.repository.UpdateOperationStatus(ctx, operationID, status); err != nil {
				return hook.HookResult{}, err
			}
		}
		result.OperationID = operationID
		if strings.HasPrefix(string(envelope.Event), "openspec.before.") && s.retriever != nil {
			memories, err := s.retriever.Retrieve(ctx, memory.Query{WorkspaceID: session.Workspace.LogicalID, ChangeKey: envelope.Subject.ChangeID, Text: strings.Join(append(envelope.Subject.Operation.ArtifactIDs, envelope.Subject.Operation.Kind), " "), Limit: 20})
			if err != nil {
				// Retrieval is advisory: allow the OpenSpec operation to proceed.
				result.Degraded = true
				result.Diagnostics = append(result.Diagnostics, hook.Diagnostic{Code: "memory_retrieval_degraded", Severity: "warning", Message: "engineering memory retrieval is unavailable"})
			} else {
				result.Memory = toMemoryContext(memories)
			}
		}
		if strings.HasPrefix(string(envelope.Event), "openspec.before.") && s.capturer != nil {
			revisions, err := s.capturer.CaptureBefore(ctx, envelope, changeID, operationID)
			if err != nil {
				return hook.HookResult{}, err
			}
			result.Revisions = revisions
		}
		if strings.HasPrefix(string(envelope.Event), "openspec.after.") && s.capturer != nil {
			revisions, err := s.capturer.CaptureAfter(ctx, envelope, changeID, operationID)
			if err != nil {
				return hook.HookResult{}, err
			}
			result.Revisions = revisions
			if recorder, ok := s.capturer.(TransformationRecorder); ok {
				transformation, err := recorder.RecordTransformation(ctx, operationID, session.Workspace.LogicalID, envelope.Subject.ProviderSummary)
				if err != nil {
					return hook.HookResult{}, err
				}
				result.Transformation = transformation
			}
			if analyzer, ok := s.capturer.(AfterDiagnosticsAnalyzer); ok {
				diagnostics, err := analyzer.AnalyzeAfter(ctx, operationID, envelope.Subject.ProviderSummary)
				if err != nil {
					return hook.HookResult{}, err
				}
				if err := s.repository.StoreDiagnostics(ctx, operationID, diagnostics); err != nil {
					return hook.HookResult{}, err
				}
				result.Diagnostics = append(result.Diagnostics, diagnostics...)
			}
			changed, err := s.repository.OperationHasChanges(ctx, operationID)
			if err != nil {
				return hook.HookResult{}, err
			}
			if !changed {
				if err := s.repository.UpdateOperationStatus(ctx, operationID, "no_change"); err != nil {
					return hook.HookResult{}, err
				}
				result.Status = "no_change"
			}
		}
		if envelope.Event == hook.EventAfterArchive && s.consolidator != nil {
			view, err := s.consolidator.Consolidate(ctx, session.Workspace.LogicalID, envelope.Subject.ChangeID)
			if err != nil {
				result.Degraded = true
				result.Diagnostics = append(result.Diagnostics, hook.Diagnostic{Code: "archive_consolidation_degraded", Severity: "warning", Message: "archive completed, but engineering-memory consolidation is incomplete"})
			} else {
				result.Memory = toMemoryContext(consolidatedResults(view))
			}
		}
	}
	stored, _, err := s.repository.StoreOrGet(ctx, envelope, result)
	return stored, err
}

func consolidatedResults(view memory.ConsolidatedView) []memory.Result {
	groups := [][]memory.Result{view.FinalScope, view.CurrentDecisions, view.FinalRequirements, view.AcceptedDeviations, view.SupersededHistory}
	var results []memory.Result
	for _, group := range groups {
		results = append(results, group...)
	}
	return results
}

func toMemoryContext(results []memory.Result) []hook.MemoryContext {
	contexts := make([]hook.MemoryContext, 0, len(results))
	for _, result := range results {
		contexts = append(contexts, hook.MemoryContext{ID: result.ID, Kind: result.Kind, Status: result.Status, Summary: result.Summary, Score: result.Score, Current: result.Current, Source: hook.MemorySource{ChangeID: result.ChangeKey, ArtifactID: result.ArtifactPath, RevisionID: result.RevisionID}})
	}
	return contexts
}

// reconcilePending closes operations whose after hooks were not delivered. The
// next hook for the same change supplies a current, authoritative OpenSpec
// state, so it can serve as the missing after snapshot without relying on a
// filesystem watcher or provider ordering guarantees.
func (s *HookApplicationService) reconcilePending(ctx context.Context, envelope hook.HookEnvelope, changeID string) (int, error) {
	pending, err := s.repository.PendingOperations(ctx, changeID, envelope.Subject.Operation.ID)
	if err != nil {
		return 0, err
	}
	for _, operationID := range pending {
		if _, err := s.capturer.CaptureAfter(ctx, envelope, changeID, operationID); err != nil {
			return 0, err
		}
		changed, err := s.repository.OperationHasChanges(ctx, operationID)
		if err != nil {
			return 0, err
		}
		status := "completed"
		if !changed {
			status = "no_change"
		}
		if err := s.repository.UpdateOperationStatus(ctx, operationID, status); err != nil {
			return 0, err
		}
	}
	return len(pending), nil
}
