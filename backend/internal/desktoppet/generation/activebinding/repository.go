package activebinding

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Repository interface {
	GetByActionID(actionID string) (*ActiveBinding, error)
	Create(tx *gorm.DB, binding *ActiveBinding) error
	CASUpdate(tx *gorm.DB, actionID string, expectedRevision int, newBinding *ActiveBinding) (bool, error)
	Delete(tx *gorm.DB, actionID string) error
}

type repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) Repository {
	return &repository{db: db}
}

func generateUUID() string {
	return uuid.New().String()
}

func nowRFC3339() string {
	return time.Now().UTC().Format(time.RFC3339)
}

func (r *repository) GetByActionID(actionID string) (*ActiveBinding, error) {
	var binding ActiveBinding
	err := r.db.Where("generation_action_id = ?", actionID).First(&binding).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get active binding by action id: %w", err)
	}
	return &binding, nil
}

func (r *repository) Create(tx *gorm.DB, binding *ActiveBinding) error {
	if tx == nil {
		tx = r.db
	}
	if binding.GenerationActionID == "" {
		return fmt.Errorf("generation action id is required")
	}
	if binding.BindingRevision == 0 {
		binding.BindingRevision = 1
	}
	if binding.BoundAt == "" {
		binding.BoundAt = nowRFC3339()
	}
	if binding.CreatedAt == "" {
		binding.CreatedAt = nowRFC3339()
	}
	if binding.UpdatedAt == "" {
		binding.UpdatedAt = nowRFC3339()
	}
	if err := tx.Create(binding).Error; err != nil {
		return fmt.Errorf("create active binding: %w", err)
	}
	return nil
}

func (r *repository) CASUpdate(tx *gorm.DB, actionID string, expectedRevision int, newBinding *ActiveBinding) (bool, error) {
	if tx == nil {
		tx = r.db
	}
	if actionID == "" {
		return false, fmt.Errorf("action id is required")
	}
	if newBinding.UpdatedAt == "" {
		newBinding.UpdatedAt = nowRFC3339()
	}
	if newBinding.BoundAt == "" {
		newBinding.BoundAt = nowRFC3339()
	}
	updates := map[string]interface{}{
		"active_attempt_id":          newBinding.ActiveAttemptID,
		"active_primary_artifact_id": newBinding.ActivePrimaryArtifactID,
		"artifact_content_hash":      newBinding.ArtifactContentHash,
		"binding_revision":           expectedRevision + 1,
		"bound_at":                   newBinding.BoundAt,
		"bound_reason":               newBinding.BoundReason,
		"updated_at":                 newBinding.UpdatedAt,
	}
	result := tx.Model(&ActiveBinding{}).
		Where("generation_action_id = ? AND binding_revision = ?", actionID, expectedRevision).
		Updates(updates)
	if result.Error != nil {
		return false, fmt.Errorf("cas update active binding: %w", result.Error)
	}
	return result.RowsAffected > 0, nil
}

func (r *repository) Delete(tx *gorm.DB, actionID string) error {
	if tx == nil {
		tx = r.db
	}
	result := tx.Where("generation_action_id = ?", actionID).Delete(&ActiveBinding{})
	if result.Error != nil {
		return fmt.Errorf("delete active binding: %w", result.Error)
	}
	return nil
}
