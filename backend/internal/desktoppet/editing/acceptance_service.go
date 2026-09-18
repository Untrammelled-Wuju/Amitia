package editing

import (
	"context"
	"fmt"
)

type applyCandidateOperationFunc func(ctx context.Context, sessionID, spaceID string, req ApplyOperationRequest) (*ApplyOperationResponse, error)

type CandidateAcceptanceService struct {
	repo           Repository
	qualAdapter    QualityAdapter
	audit          *AuditOutbox
	applyOperation applyCandidateOperationFunc
}

func NewCandidateAcceptanceService(repo Repository, qualAdapter QualityAdapter, audit *AuditOutbox, applyOperation applyCandidateOperationFunc) *CandidateAcceptanceService {
	return &CandidateAcceptanceService{repo: repo, qualAdapter: qualAdapter, audit: audit, applyOperation: applyOperation}
}

func (s *CandidateAcceptanceService) beginOperation(candidate *EditCandidate, spaceID, action, idempotencyKey string) (*CandidateAcceptanceOperation, bool, error) {
	if idempotencyKey == "" {
		idempotencyKey = fmt.Sprintf("candidate:%s:%s", action, candidate.ID)
	}
	existing, err := s.repo.GetCandidateAcceptanceOperation(candidate.ID, idempotencyKey)
	if err != nil {
		return nil, false, err
	}
	if existing != nil {
		if existing.SpaceID != spaceID || existing.Action != action {
			return nil, false, ErrPermissionDenied
		}
		return existing, existing.Status == "completed", nil
	}
	op := &CandidateAcceptanceOperation{
		ID:             generateID("accept-op"),
		CandidateID:    candidate.ID,
		SessionID:      candidate.SessionID,
		SpaceID:        spaceID,
		Action:         action,
		IdempotencyKey: idempotencyKey,
		Status:         "pending",
		CreatedAt:      nowUTC(),
		UpdatedAt:      nowUTC(),
	}
	if err := s.repo.CreateCandidateAcceptanceOperation(op); err != nil {
		existing, readErr := s.repo.GetCandidateAcceptanceOperation(candidate.ID, idempotencyKey)
		if readErr != nil || existing == nil {
			return nil, false, err
		}
		if existing.SpaceID != spaceID || existing.Action != action {
			return nil, false, ErrPermissionDenied
		}
		return existing, existing.Status == "completed", nil
	}
	return op, false, nil
}

func (s *CandidateAcceptanceService) failOperation(op *CandidateAcceptanceOperation, err error) error {
	if op != nil {
		_ = s.repo.UpdateCandidateAcceptanceOperation(op.ID, "failed", err.Error())
	}
	return err
}

func (s *CandidateAcceptanceService) AcceptCandidate(ctx context.Context, candidateID, spaceID, idempotencyKey string) error {
	candidate, err := s.repo.GetCandidate(candidateID)
	if err != nil {
		return err
	}
	if candidate.SpaceID != "" && candidate.SpaceID != spaceID {
		return ErrPermissionDenied
	}
	session, err := s.repo.GetEditSession(candidate.SessionID)
	if err != nil {
		return err
	}
	if session == nil || session.SpaceID != spaceID {
		return ErrPermissionDenied
	}
	if candidate.Status == CandidateStatusAccepted {
		return nil
	}
	if candidate.Status == CandidateStatusRejected || candidate.Status == CandidateStatusExpired || candidate.Status == CandidateStatusArchived || candidate.Status == CandidateStatusStaleCandidate {
		return ErrCandidateAlreadyDecided
	}

	op, completed, err := s.beginOperation(candidate, spaceID, "accept", idempotencyKey)
	if err != nil {
		return err
	}
	if completed {
		return nil
	}

	if candidate.CandidateRevisionID == "" {
		if candidate.CandidateVersion != 0 && session.SessionVersion != candidate.CandidateVersion {
			_ = s.repo.UpdateCandidateFields(candidate.ID, map[string]any{"status": CandidateStatusStaleCandidate})
			return s.failOperation(op, ErrCandidateAcceptConflict)
		}
		if candidate.AssetID == "" || candidate.TargetFrameID == "" {
			return s.failOperation(op, ErrCandidateNotReady)
		}
		if s.applyOperation == nil {
			return s.failOperation(op, fmt.Errorf("candidate apply operation service unavailable"))
		}
		_, err := s.applyOperation(ctx, candidate.SessionID, spaceID, ApplyOperationRequest{
			BaseSessionVersion: session.SessionVersion,
			IdempotencyKey:     "candidate-frame-accept:" + candidate.ID,
			Operation: OperationPayload{
				Type:          OpFrameReplaceAsset,
				SchemaVersion: OperationSchemaVersion,
				Payload: FrameReplaceAssetPayload{
					FrameID:    candidate.TargetFrameID,
					AssetID:    candidate.AssetID,
					KeepAnchor: true,
				},
			},
		})
		if err != nil {
			return s.failOperation(op, err)
		}
		return s.finalizeAccepted(ctx, op, candidate, session, spaceID, "", "")
	}

	if candidate.CandidateVersion != 0 && session.SessionVersion != candidate.CandidateVersion {
		_ = s.repo.UpdateCandidateFields(candidate.ID, map[string]any{"status": CandidateStatusStaleCandidate})
		return s.failOperation(op, ErrCandidateAcceptConflict)
	}

	passed, gateReason, err := s.qualAdapter.IsGatePassed(ctx, candidate.CandidateRevisionID)
	if err != nil {
		return s.failOperation(op, err)
	}
	if !passed {
		switch gateReason {
		case "needs_review":
			// This endpoint is the explicit human review decision, so a
			// needs_review quality verdict can be overridden by acceptance.
		case "quality_pending", "pending", "running":
			return s.failOperation(op, ErrCandidateQualityNotReady)
		default:
			return s.failOperation(op, ErrQualityGateBlocked)
		}
	}

	binding, err := resolveActiveRevisionBinding(s.repo, session.ProcessingTaskID, session.ActionKey)
	if err != nil {
		return s.failOperation(op, err)
	}
	if binding == nil {
		return s.failOperation(op, ErrCandidateAcceptConflict)
	}
	if binding.RevisionID == candidate.CandidateRevisionID {
		return s.finalizeAccepted(ctx, op, candidate, session, spaceID, binding.RevisionID, binding.RevisionID)
	}
	if binding.BindingRevision != candidate.BaseBindingRevision ||
		(candidate.ParentRevisionID != "" && binding.RevisionID != candidate.ParentRevisionID) {
		_ = s.repo.UpdateCandidateFields(candidate.ID, map[string]any{"status": CandidateStatusStaleCandidate})
		return s.failOperation(op, ErrCandidateAcceptConflict)
	}
	previousRevisionID := binding.RevisionID
	if _, _, err := bindActiveRevision(s.repo, session.ProcessingTaskID, session.ActionKey, candidate.CandidateRevisionID, candidate.BaseBindingRevision, spaceID, "candidate.accepted"); err != nil {
		return s.failOperation(op, err)
	}
	return s.finalizeAccepted(ctx, op, candidate, session, spaceID, previousRevisionID, candidate.CandidateRevisionID)
}

func (s *CandidateAcceptanceService) finalizeAccepted(ctx context.Context, op *CandidateAcceptanceOperation, candidate *EditCandidate, session *EditSession, spaceID, previousRevisionID, newRevisionID string) error {
	now := nowUTC()
	effectiveVerdict := candidate.EffectiveVerdict
	if candidate.CandidateRevisionID != "" {
		if rev, err := s.repo.GetActionRevision(candidate.CandidateRevisionID); err == nil {
			effectiveVerdict = rev.QualityVerdict
		}
	}
	if err := s.repo.UpdateCandidateFields(candidate.ID, map[string]any{
		"status":            CandidateStatusAccepted,
		"accepted_at":       now,
		"decided_by":        spaceID,
		"decided_at":        now,
		"effective_verdict": effectiveVerdict,
	}); err != nil {
		return s.failOperation(op, err)
	}
	if candidate.CandidateRevisionID != "" {
		_ = s.repo.UpdateCandidateRevisionMetadataStatus(candidate.CandidateRevisionID, CandidateStatusAccepted)
	}
	if candidate.JobID != "" {
		if err := s.repo.UpdateJobFields(candidate.JobID, map[string]any{"status": JobStatusAccepted, "completed_at": now}); err != nil {
			return s.failOperation(op, err)
		}
	}
	if err := s.repo.UpdateCandidateAcceptanceOperation(op.ID, "completed", ""); err != nil {
		return err
	}
	_ = s.audit.Log(ctx, "candidate.accepted", spaceID, session.ActionKey,
		candidate.SessionID, candidate.JobID, session.BaseRevisionID, candidate.CandidateRevisionID,
		previousRevisionID, newRevisionID, "candidate.accepted")
	return nil
}

func (s *CandidateAcceptanceService) RejectCandidate(ctx context.Context, candidateID, spaceID, reason, idempotencyKey string) error {
	candidate, err := s.repo.GetCandidate(candidateID)
	if err != nil {
		return err
	}
	if candidate.SpaceID != "" && candidate.SpaceID != spaceID {
		return ErrPermissionDenied
	}
	session, err := s.repo.GetEditSession(candidate.SessionID)
	if err != nil {
		return err
	}
	if session == nil || session.SpaceID != spaceID {
		return ErrPermissionDenied
	}
	if candidate.Status == CandidateStatusRejected {
		return nil
	}
	if candidate.Status == CandidateStatusAccepted || candidate.Status == CandidateStatusExpired || candidate.Status == CandidateStatusArchived {
		return ErrCandidateAlreadyDecided
	}
	op, completed, err := s.beginOperation(candidate, spaceID, "reject", idempotencyKey)
	if err != nil {
		return err
	}
	if completed {
		return nil
	}
	now := nowUTC()
	if err := s.repo.UpdateCandidateFields(candidate.ID, map[string]any{
		"status":        CandidateStatusRejected,
		"rejected_at":   now,
		"reject_reason": reason,
		"decided_by":    spaceID,
		"decided_at":    now,
	}); err != nil {
		return s.failOperation(op, err)
	}
	if candidate.CandidateRevisionID != "" {
		_ = s.repo.UpdateCandidateRevisionMetadataStatus(candidate.CandidateRevisionID, CandidateStatusRejected)
	}
	if candidate.JobID != "" {
		if err := s.repo.UpdateJobFields(candidate.JobID, map[string]any{
			"status":        JobStatusRejected,
			"rejected_by":   spaceID,
			"rejected_at":   now,
			"reject_reason": reason,
			"completed_at":  now,
		}); err != nil {
			return s.failOperation(op, err)
		}
	}
	if err := s.repo.UpdateCandidateAcceptanceOperation(op.ID, "completed", ""); err != nil {
		return err
	}
	_ = s.audit.Log(ctx, "candidate.rejected", spaceID, session.ActionKey,
		candidate.SessionID, candidate.JobID, session.BaseRevisionID, candidate.CandidateRevisionID,
		"", "", reason)
	return nil
}
