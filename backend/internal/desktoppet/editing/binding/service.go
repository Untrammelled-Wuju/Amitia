package binding

import (
	"context"
	"fmt"
	"time"

	"github.com/u-ai/backend/internal/desktoppet/editing"
	"github.com/u-ai/backend/internal/desktoppet/editing/baseline"
	"gorm.io/gorm"
)

const RevisionStatusPublishing = "publishing"

type BindActiveRevisionRequest struct {
	UserID                  string
	CharacterID             string
	ActionKey               string
	TargetRevisionID        string
	ExpectedBindingRevision int64
	Reason                  string
	Actor                   string
}

type ActiveRevisionBindingService interface {
	Bind(ctx context.Context, req BindActiveRevisionRequest) (*editing.ActiveActionRevisionBinding, error)
	GetActiveBinding(ctx context.Context, userID, characterID, actionKey string) (*editing.ActiveActionRevisionBinding, error)
	AllocateRevisionNumber(ctx context.Context, userID, characterID, actionKey string) (int64, error)
	ValidateRevisionForBinding(ctx context.Context, userID, characterID, actionKey, revisionID string) error
}

type service struct {
	repo BindingRepository
}

func NewService(repo BindingRepository) ActiveRevisionBindingService {
	return &service{repo: repo}
}

func (s *service) ValidateRevisionForBinding(ctx context.Context, userID, characterID, actionKey, revisionID string) error {
	rev, err := s.repo.GetActionRevisionForUser(userID, revisionID)
	if err != nil {
		return err
	}
	if rev == nil {
		return editing.ErrRevisionNotFound
	}
	if rev.CharacterID != characterID {
		return editing.ErrOwnershipDenied
	}
	if rev.ActionKey != actionKey {
		return editing.ErrOwnershipDenied
	}
	if rev.ArchivedAt != "" {
		return editing.ErrRevisionNotReady
	}
	switch rev.Status {
	case baseline.RevisionStatusCandidate,
		baseline.RevisionStatusFailed,
		baseline.RevisionStatusLegacyUnresolved,
		RevisionStatusPublishing:
		return editing.ErrRevisionNotReady
	}
	if rev.Status != baseline.RevisionStatusReady {
		return editing.ErrRevisionNotReady
	}
	return nil
}

func (s *service) Bind(ctx context.Context, req BindActiveRevisionRequest) (*editing.ActiveActionRevisionBinding, error) {
	if err := s.ValidateRevisionForBinding(ctx, req.UserID, req.CharacterID, req.ActionKey, req.TargetRevisionID); err != nil {
		return nil, err
	}
	existing, err := s.repo.GetActiveBinding(req.UserID, req.CharacterID, req.ActionKey)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		now := time.Now().UTC().Format(time.RFC3339)
		binding := &editing.ActiveActionRevisionBinding{
			ID:                     fmt.Sprintf("ab-%d", time.Now().UnixNano()),
			UserID:                 req.UserID,
			CharacterID:            req.CharacterID,
			ActionKey:              req.ActionKey,
			ActiveActionRevisionID: req.TargetRevisionID,
			BindingRevision:        1,
			BoundReason:            req.Reason,
			BoundBy:                req.Actor,
			BoundAt:                now,
			CreatedAt:              now,
			UpdatedAt:              now,
		}
		if err := s.repo.CreateActiveBinding(binding); err != nil {
			return nil, err
		}
		return binding, nil
	}
	rowsAffected, err := s.repo.CASUpdateActiveBinding(req.UserID, req.CharacterID, req.ActionKey, req.TargetRevisionID, req.ExpectedBindingRevision, req.Reason, req.Actor)
	if err != nil {
		return nil, err
	}
	if rowsAffected == 0 {
		return nil, editing.ErrActiveBindingConflict
	}
	updated, err := s.repo.GetActiveBinding(req.UserID, req.CharacterID, req.ActionKey)
	if err != nil {
		return nil, err
	}
	if updated == nil {
		return nil, editing.ErrActiveBindingConflict
	}
	return updated, nil
}

func (s *service) GetActiveBinding(ctx context.Context, userID, characterID, actionKey string) (*editing.ActiveActionRevisionBinding, error) {
	return s.repo.GetActiveBinding(userID, characterID, actionKey)
}

func (s *service) AllocateRevisionNumber(ctx context.Context, userID, characterID, actionKey string) (int64, error) {
	var allocated int64
	err := s.repo.DB().Transaction(func(tx *gorm.DB) error {
		n, txErr := s.repo.AllocateRevisionNumber(tx, userID, characterID, actionKey)
		if txErr != nil {
			return txErr
		}
		allocated = n
		return nil
	})
	if err != nil {
		return 0, err
	}
	return allocated, nil
}
