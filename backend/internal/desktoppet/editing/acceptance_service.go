package editing

import (
	"context"

	"github.com/u-ai/backend/log"
)

type CandidateAcceptanceService struct {
	repo        Repository
	qualAdapter QualityAdapter
	audit       *AuditOutbox
}

func NewCandidateAcceptanceService(repo Repository, qualAdapter QualityAdapter, audit *AuditOutbox) *CandidateAcceptanceService {
	return &CandidateAcceptanceService{
		repo:        repo,
		qualAdapter: qualAdapter,
		audit:       audit,
	}
}

func (s *CandidateAcceptanceService) AcceptCandidate(ctx context.Context, candidateID, userID string) error {
	candidate, err := s.repo.GetCandidate(candidateID)
	if err != nil {
		return err
	}
	if candidate.Status != CandidateStatusReadyForReview {
		return ErrCandidateAlreadyDecided
	}

	if candidate.CandidateRevisionID != "" {
		passed, gateReason, err := s.qualAdapter.IsGatePassed(ctx, candidate.CandidateRevisionID)
		if err != nil {
			return err
		}
		if !passed {
			log.Logger.Errorf("candidate accept blocked by quality gate: candidate=%s revision=%s reason=%s",
				candidateID, candidate.CandidateRevisionID, gateReason)
			return ErrQualityGateBlocked
		}

		meta, err := s.repo.GetCandidateRevisionMetadata(candidate.CandidateRevisionID)
		if err != nil {
			return err
		}

		session, err := s.repo.GetEditSession(candidate.SessionID)
		if err != nil {
			return err
		}

		binding, err := s.repo.GetActiveRevisionBinding(session.ProcessingTaskID, session.ActionKey)
		if err != nil {
			return err
		}

		if binding == nil || candidate.BaseBindingRevision != binding.BindingVersion {
			s.repo.UpdateCandidateFields(candidate.ID, map[string]any{
				"status": CandidateStatusStaleCandidate,
			})
			log.Logger.Errorf("candidate accept conflict: candidate=%s baseBindingRevision=%d",
				candidateID, candidate.BaseBindingRevision)
			return ErrCandidateAcceptConflict
		}

		ok, err := s.repo.CASUpdateActiveBinding(
			session.ProcessingTaskID,
			session.ActionKey,
			candidate.BaseBindingRevision,
			candidate.CandidateRevisionID,
			userID,
			"candidate.accepted",
		)
		if err != nil {
			return err
		}
		if !ok {
			return ErrCandidateAcceptConflict
		}

		now := nowUTC()
		if err := s.repo.UpdateCandidateFields(candidate.ID, map[string]any{
			"status":      CandidateStatusAccepted,
			"accepted_at": now,
			"decided_by":  userID,
			"decided_at":  now,
		}); err != nil {
			return err
		}

		if err := s.repo.UpdateJobFields(candidate.JobID, map[string]any{
			"status": JobStatusAccepted,
		}); err != nil {
			return err
		}

		baseRevID := session.BaseRevisionID
		if meta != nil && meta.ParentRevisionID != "" {
			baseRevID = meta.ParentRevisionID
		}

		s.audit.Log(ctx, "candidate.accepted", userID, binding.CharacterID, session.ActionKey,
			candidate.SessionID, candidate.JobID, baseRevID, candidate.CandidateRevisionID,
			binding.RevisionID, candidate.CandidateRevisionID, "candidate.accepted")

		return nil
	}

	now := nowUTC()
	if err := s.repo.UpdateCandidateFields(candidate.ID, map[string]any{
		"status":      CandidateStatusAccepted,
		"accepted_at": now,
		"decided_by":  userID,
		"decided_at":  now,
	}); err != nil {
		return err
	}

	if err := s.repo.UpdateJobFields(candidate.JobID, map[string]any{
		"status": JobStatusAccepted,
	}); err != nil {
		return err
	}

	var actionKey, baseRevisionID string
	session, err := s.repo.GetEditSession(candidate.SessionID)
	if err == nil && session != nil {
		actionKey = session.ActionKey
		baseRevisionID = session.BaseRevisionID
	}

	s.audit.Log(ctx, "candidate.accepted", userID, "", actionKey,
		candidate.SessionID, candidate.JobID, baseRevisionID, "",
		"", "", "candidate.accepted")

	return nil
}

func (s *CandidateAcceptanceService) RejectCandidate(ctx context.Context, candidateID, userID, reason string) error {
	candidate, err := s.repo.GetCandidate(candidateID)
	if err != nil {
		return err
	}
	if candidate.Status != CandidateStatusReadyForReview && candidate.Status != CandidateStatusPending {
		return ErrCandidateAlreadyDecided
	}

	now := nowUTC()
	if err := s.repo.UpdateCandidateFields(candidate.ID, map[string]any{
		"status":        CandidateStatusRejected,
		"rejected_at":   now,
		"reject_reason": reason,
		"decided_by":    userID,
		"decided_at":    now,
	}); err != nil {
		return err
	}

	if err := s.repo.UpdateJobFields(candidate.JobID, map[string]any{
		"status":        JobStatusRejected,
		"rejected_by":   userID,
		"rejected_at":   now,
		"reject_reason": reason,
	}); err != nil {
		return err
	}

	var actionKey, baseRevisionID string
	session, err := s.repo.GetEditSession(candidate.SessionID)
	if err == nil && session != nil {
		actionKey = session.ActionKey
		baseRevisionID = session.BaseRevisionID
	}

	s.audit.Log(ctx, "candidate.rejected", userID, "", actionKey,
		candidate.SessionID, candidate.JobID, baseRevisionID,
		candidate.CandidateRevisionID, "", "", reason)

	return nil
}
