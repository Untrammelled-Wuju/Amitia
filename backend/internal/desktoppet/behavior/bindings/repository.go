package bindings

import (
	"context"
	"encoding/json"
	"time"

	"gorm.io/gorm"

	"github.com/u-ai/backend/internal/desktoppet/behavior"
	"github.com/u-ai/backend/internal/desktoppet/behavior/persistence"
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Create(ctx context.Context, binding behavior.BehaviorBinding) error {
	model := bindingToModel(binding)
	return r.db.WithContext(ctx).Create(model).Error
}

func (r *Repository) Update(ctx context.Context, id string, updates map[string]interface{}) error {
	if updates == nil {
		updates = make(map[string]interface{})
	}
	updates["updated_at"] = time.Now().Format(time.RFC3339)
	result := r.db.WithContext(ctx).
		Model(&persistence.BehaviorBindingModel{}).
		Where("id = ?", id).
		Updates(updates)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *Repository) Delete(ctx context.Context, id string) error {
	result := r.db.WithContext(ctx).
		Where("id = ?", id).
		Delete(&persistence.BehaviorBindingModel{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *Repository) GetByID(ctx context.Context, id string) (*behavior.BehaviorBinding, error) {
	var model persistence.BehaviorBindingModel
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&model).Error; err != nil {
		return nil, err
	}
	b := modelToBinding(model)
	return &b, nil
}

func (r *Repository) ListByUserCharacter(ctx context.Context, userID, characterID string) ([]behavior.BehaviorBinding, error) {
	var models []persistence.BehaviorBindingModel
	query := r.db.WithContext(ctx).Where("user_id = ?", userID)
	if characterID != "" {
		query = query.Where("character_id = ?", characterID)
	}
	if err := query.Order("created_at DESC").Find(&models).Error; err != nil {
		return nil, err
	}
	result := make([]behavior.BehaviorBinding, 0, len(models))
	for _, m := range models {
		result = append(result, modelToBinding(m))
	}
	return result, nil
}

func (r *Repository) ListByEventType(ctx context.Context, userID, characterID, eventType string) ([]behavior.BehaviorBinding, error) {
	var models []persistence.BehaviorBindingModel
	query := r.db.WithContext(ctx).Where("user_id = ? AND event_type = ?", userID, eventType)
	if characterID != "" {
		query = query.Where("character_id = ?", characterID)
	}
	if err := query.Order("priority_offset DESC, created_at DESC").Find(&models).Error; err != nil {
		return nil, err
	}
	result := make([]behavior.BehaviorBinding, 0, len(models))
	for _, m := range models {
		result = append(result, modelToBinding(m))
	}
	return result, nil
}

func bindingToModel(b behavior.BehaviorBinding) *persistence.BehaviorBindingModel {
	return &persistence.BehaviorBindingModel{
		ID:              b.ID,
		UserID:          b.UserID,
		CharacterID:     b.CharacterID,
		InstallationID:  b.InstallationID,
		EventType:       b.EventType,
		ConditionsJSON:  string(b.ConditionsJSON),
		Semantic:        b.Semantic,
		PreferredAction: b.PreferredAction,
		PriorityOffset:  b.PriorityOffset,
		CooldownMS:      b.CooldownMS,
		Enabled:         b.Enabled,
		Version:         b.Version,
		CreatedAt:       b.CreatedAt.Format(time.RFC3339),
		UpdatedAt:       b.UpdatedAt.Format(time.RFC3339),
	}
}

func modelToBinding(m persistence.BehaviorBindingModel) behavior.BehaviorBinding {
	var conditions json.RawMessage
	if m.ConditionsJSON != "" {
		conditions = json.RawMessage(m.ConditionsJSON)
	}
	b := behavior.BehaviorBinding{
		ID:              m.ID,
		UserID:          m.UserID,
		CharacterID:     m.CharacterID,
		InstallationID:  m.InstallationID,
		EventType:       m.EventType,
		ConditionsJSON:  conditions,
		Semantic:        m.Semantic,
		PreferredAction: m.PreferredAction,
		PriorityOffset:  m.PriorityOffset,
		CooldownMS:      m.CooldownMS,
		Enabled:         m.Enabled,
		Version:         m.Version,
	}
	if m.CreatedAt != "" {
		if t, err := time.Parse(time.RFC3339, m.CreatedAt); err == nil {
			b.CreatedAt = t
		}
	}
	if m.UpdatedAt != "" {
		if t, err := time.Parse(time.RFC3339, m.UpdatedAt); err == nil {
			b.UpdatedAt = t
		}
	}
	return b
}
