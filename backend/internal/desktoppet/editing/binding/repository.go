package binding

import (
	"errors"
	"fmt"
	"time"

	"github.com/u-ai/backend/internal/desktoppet/editing"
	"gorm.io/gorm"
)

type BindingRepository interface {
	GetActiveBinding(userID, characterID, actionKey string) (*editing.ActiveActionRevisionBinding, error)
	CreateActiveBinding(binding *editing.ActiveActionRevisionBinding) error
	CASUpdateActiveBinding(userID, characterID, actionKey string, targetRevisionID string, expectedBindingRevision int64, reason, actor string) (int64, error)
	GetActionStream(userID, characterID, actionKey string) (*editing.ActionStream, error)
	CreateActionStream(stream *editing.ActionStream) error
	AllocateRevisionNumber(tx *gorm.DB, userID, characterID, actionKey string) (int64, error)
	GetActionRevision(id string) (*editing.ActionRevision, error)
	GetActionRevisionForUser(userID, revisionID string) (*editing.ActionRevision, error)
	ListRevisionFrames(revisionID string) ([]editing.ActionRevisionFrame, error)
	DB() *gorm.DB
}

type bindingRepository struct {
	db *gorm.DB
}

func NewBindingRepository(db *gorm.DB) BindingRepository {
	return &bindingRepository{db: db}
}

func (r *bindingRepository) DB() *gorm.DB { return r.db }

func (r *bindingRepository) GetActiveBinding(userID, characterID, actionKey string) (*editing.ActiveActionRevisionBinding, error) {
	var binding editing.ActiveActionRevisionBinding
	err := r.db.Where("user_id = ? AND character_id = ? AND action_key = ?", userID, characterID, actionKey).First(&binding).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &binding, nil
}

func (r *bindingRepository) CreateActiveBinding(binding *editing.ActiveActionRevisionBinding) error {
	now := time.Now().UTC().Format(time.RFC3339)
	if binding.ID == "" {
		binding.ID = fmt.Sprintf("ab-%d", time.Now().UnixNano())
	}
	if binding.CreatedAt == "" {
		binding.CreatedAt = now
	}
	if binding.BoundAt == "" {
		binding.BoundAt = now
	}
	binding.UpdatedAt = now
	return r.db.Create(binding).Error
}

func (r *bindingRepository) CASUpdateActiveBinding(userID, characterID, actionKey string, targetRevisionID string, expectedBindingRevision int64, reason, actor string) (int64, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	result := r.db.Model(&editing.ActiveActionRevisionBinding{}).
		Where("user_id = ? AND character_id = ? AND action_key = ? AND binding_revision = ?",
			userID, characterID, actionKey, expectedBindingRevision).
		Updates(map[string]any{
			"active_action_revision_id": targetRevisionID,
			"binding_revision":          gorm.Expr("binding_revision + 1"),
			"bound_reason":              reason,
			"bound_by":                  actor,
			"bound_at":                  now,
			"updated_at":                now,
		})
	if result.Error != nil {
		return 0, result.Error
	}
	return result.RowsAffected, nil
}

func (r *bindingRepository) GetActionStream(userID, characterID, actionKey string) (*editing.ActionStream, error) {
	var stream editing.ActionStream
	err := r.db.Where("user_id = ? AND character_id = ? AND action_key = ?", userID, characterID, actionKey).First(&stream).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &stream, nil
}

func (r *bindingRepository) CreateActionStream(stream *editing.ActionStream) error {
	now := time.Now().UTC().Format(time.RFC3339)
	if stream.ID == "" {
		stream.ID = fmt.Sprintf("as-%d", time.Now().UnixNano())
	}
	if stream.CreatedAt == "" {
		stream.CreatedAt = now
	}
	stream.UpdatedAt = now
	return r.db.Create(stream).Error
}

func (r *bindingRepository) AllocateRevisionNumber(tx *gorm.DB, userID, characterID, actionKey string) (int64, error) {
	db := tx
	if db == nil {
		db = r.db
	}
	var stream editing.ActionStream
	err := db.Where("user_id = ? AND character_id = ? AND action_key = ?", userID, characterID, actionKey).First(&stream).Error
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return 0, err
		}
		now := time.Now().UTC().Format(time.RFC3339)
		stream = editing.ActionStream{
			ID:                 fmt.Sprintf("as-%d", time.Now().UnixNano()),
			UserID:             userID,
			CharacterID:        characterID,
			ActionKey:          actionKey,
			NextRevisionNumber: 1,
			CreatedAt:          now,
			UpdatedAt:          now,
		}
		if createErr := db.Create(&stream).Error; createErr != nil {
			return 0, createErr
		}
	}
	allocated := stream.NextRevisionNumber
	now := time.Now().UTC().Format(time.RFC3339)
	result := db.Model(&editing.ActionStream{}).
		Where("id = ? AND next_revision_number = ?", stream.ID, allocated).
		Updates(map[string]any{
			"next_revision_number": allocated + 1,
			"updated_at":           now,
		})
	if result.Error != nil {
		return 0, result.Error
	}
	if result.RowsAffected == 0 {
		return 0, editing.ErrActiveBindingConflict
	}
	return allocated, nil
}

func (r *bindingRepository) GetActionRevision(id string) (*editing.ActionRevision, error) {
	var rev editing.ActionRevision
	err := r.db.Where("id = ?", id).First(&rev).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, editing.ErrRevisionNotFound
		}
		return nil, err
	}
	return &rev, nil
}

func (r *bindingRepository) GetActionRevisionForUser(userID, revisionID string) (*editing.ActionRevision, error) {
	var rev editing.ActionRevision
	err := r.db.Where("id = ? AND user_id = ?", revisionID, userID).First(&rev).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &rev, nil
}

func (r *bindingRepository) ListRevisionFrames(revisionID string) ([]editing.ActionRevisionFrame, error) {
	var frames []editing.ActionRevisionFrame
	err := r.db.Where("revision_id = ?", revisionID).Order("logical_index ASC").Find(&frames).Error
	return frames, err
}
