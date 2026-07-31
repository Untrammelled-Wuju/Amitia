package binding

import (
	"context"
	"fmt"
	"time"

	"github.com/u-ai/backend/internal/desktoppet/editing"
	"gorm.io/gorm"
)

type ActionStreamService interface {
	EnsureStream(userID, characterID, actionKey string) (*editing.ActionStream, error)
	AllocateRevisionNumber(ctx context.Context, userID, characterID, actionKey string) (int64, error)
}

type actionStreamService struct {
	repo BindingRepository
}

func NewActionStreamService(repo BindingRepository) ActionStreamService {
	return &actionStreamService{repo: repo}
}

func (s *actionStreamService) EnsureStream(userID, characterID, actionKey string) (*editing.ActionStream, error) {
	stream, err := s.repo.GetActionStream(userID, characterID, actionKey)
	if err != nil {
		return nil, err
	}
	if stream != nil {
		return stream, nil
	}
	now := time.Now().UTC().Format(time.RFC3339)
	stream = &editing.ActionStream{
		ID:                 fmt.Sprintf("as-%d", time.Now().UnixNano()),
		UserID:             userID,
		CharacterID:        characterID,
		ActionKey:          actionKey,
		NextRevisionNumber: 1,
		CreatedAt:          now,
		UpdatedAt:          now,
	}
	if err := s.repo.CreateActionStream(stream); err != nil {
		return nil, err
	}
	return stream, nil
}

func (s *actionStreamService) AllocateRevisionNumber(ctx context.Context, userID, characterID, actionKey string) (int64, error) {
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
