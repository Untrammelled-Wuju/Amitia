package bindings

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"gorm.io/gorm"
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Create(ctx context.Context, binding BehaviorBinding) error {
	model := bindingToModel(binding)
	return r.db.WithContext(ctx).Create(model).Error
}

var repoBindingAllowedColumns = map[string]bool{
	"event_type":       true,
	"conditions_json":  true,
	"semantic":         true,
	"preferred_action": true,
	"priority_offset":  true,
	"cooldown_ms":      true,
	"enabled":          true,
	"updated_at":       true,
}

func (r *Repository) Update(ctx context.Context, id string, updates map[string]interface{}) error {
	if updates == nil {
		updates = make(map[string]interface{})
	}
	updates["updated_at"] = time.Now().Format(time.RFC3339)
	sanitized := make(map[string]interface{}, len(updates))
	for k, v := range updates {
		if repoBindingAllowedColumns[k] {
			sanitized[k] = v
		}
	}
	result := r.db.WithContext(ctx).
		Model(&BehaviorBindingModel{}).
		Where("id = ?", id).
		Updates(sanitized)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *Repository) UpdateTyped(ctx context.Context, req BindingUpdateRequest) (*BehaviorBinding, error) {
	tx := r.db.WithContext(ctx).Begin()
	if tx.Error != nil {
		return nil, tx.Error
	}
	defer func() {
		if rec := recover(); rec != nil {
			tx.Rollback()
		}
	}()

	var model BehaviorBindingModel
	if err := tx.Where("id = ? AND user_id = ?", req.ID, req.UserID).First(&model).Error; err != nil {
		tx.Rollback()
		if err == gorm.ErrRecordNotFound {
			return nil, errors.New("binding not found")
		}
		return nil, err
	}

	if model.Version != req.ExpectedVersion {
		tx.Rollback()
		return nil, errors.New("version conflict")
	}

	updates := map[string]interface{}{
		"version":    model.Version + 1,
		"updated_at": time.Now().Format(time.RFC3339),
	}
	if req.EventType != nil {
		updates["event_type"] = *req.EventType
	}
	if req.ConditionsJSON != nil {
		updates["conditions_json"] = *req.ConditionsJSON
	}
	if req.Semantic != nil {
		updates["semantic"] = *req.Semantic
	}
	if req.PreferredAction != nil {
		updates["preferred_action"] = *req.PreferredAction
	}
	if req.PriorityOffset != nil {
		updates["priority_offset"] = *req.PriorityOffset
	}
	if req.CooldownMS != nil {
		updates["cooldown_ms"] = *req.CooldownMS
	}
	if req.Enabled != nil {
		updates["enabled"] = *req.Enabled
	}

	result := tx.Model(&model).
		Where("id = ? AND user_id = ? AND version = ?", req.ID, req.UserID, req.ExpectedVersion).
		Updates(updates)
	if result.Error != nil {
		tx.Rollback()
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		tx.Rollback()
		return nil, errors.New("version conflict")
	}

	if err := tx.First(&model).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	if err := tx.Commit().Error; err != nil {
		return nil, err
	}

	b := modelToBinding(model)
	return &b, nil
}

func (r *Repository) Delete(ctx context.Context, id string) error {
	result := r.db.WithContext(ctx).
		Where("id = ?", id).
		Delete(&BehaviorBindingModel{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *Repository) GetByID(ctx context.Context, id string) (*BehaviorBinding, error) {
	var model BehaviorBindingModel
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&model).Error; err != nil {
		return nil, err
	}
	b := modelToBinding(model)
	return &b, nil
}

func (r *Repository) ListByUserCharacter(ctx context.Context, userID, characterID string) ([]BehaviorBinding, error) {
	var models []BehaviorBindingModel
	query := r.db.WithContext(ctx).Where("user_id = ?", userID)
	if characterID != "" {
		query = query.Where("character_id = ?", characterID)
	}
	if err := query.Order("created_at DESC").Find(&models).Error; err != nil {
		return nil, err
	}
	result := make([]BehaviorBinding, 0, len(models))
	for _, m := range models {
		result = append(result, modelToBinding(m))
	}
	return result, nil
}

// ListByScope returns bindings for exactly one persisted user/character scope.
// Unlike ListByUserCharacter, an empty characterID means the global scope only.
func (r *Repository) ListByScope(ctx context.Context, userID, characterID string) ([]BehaviorBinding, error) {
	var models []BehaviorBindingModel
	if err := r.db.WithContext(ctx).
		Where("user_id = ? AND character_id = ?", userID, characterID).
		Order("created_at ASC").
		Find(&models).Error; err != nil {
		return nil, err
	}
	result := make([]BehaviorBinding, 0, len(models))
	for _, m := range models {
		result = append(result, modelToBinding(m))
	}
	return result, nil
}

func (r *Repository) ListScopes(ctx context.Context) ([]EvaluatorScope, error) {
	type scopeRow struct {
		UserID      string `gorm:"column:user_id"`
		CharacterID string `gorm:"column:character_id"`
	}
	var rows []scopeRow
	if err := r.db.WithContext(ctx).
		Model(&BehaviorBindingModel{}).
		Select("user_id, character_id").
		Group("user_id, character_id").
		Order("user_id ASC, character_id ASC").
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	result := make([]EvaluatorScope, 0, len(rows))
	for _, row := range rows {
		result = append(result, EvaluatorScope{UserID: row.UserID, CharacterID: row.CharacterID})
	}
	return result, nil
}

func (r *Repository) ListByEventType(ctx context.Context, userID, characterID, eventType string) ([]BehaviorBinding, error) {
	var models []BehaviorBindingModel
	query := r.db.WithContext(ctx).Where("user_id = ? AND event_type = ?", userID, eventType)
	if characterID != "" {
		query = query.Where("character_id = ?", characterID)
	}
	if err := query.Order("priority_offset DESC, created_at DESC").Find(&models).Error; err != nil {
		return nil, err
	}
	result := make([]BehaviorBinding, 0, len(models))
	for _, m := range models {
		result = append(result, modelToBinding(m))
	}
	return result, nil
}

func (r *Repository) GetByIDTyped(ctx context.Context, id string) (*BehaviorBinding, error) {
	return r.GetByID(ctx, id)
}

func bindingToModel(b BehaviorBinding) *BehaviorBindingModel {
	return &BehaviorBindingModel{
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

func modelToBinding(m BehaviorBindingModel) BehaviorBinding {
	var conditions json.RawMessage
	if m.ConditionsJSON != "" {
		conditions = json.RawMessage(m.ConditionsJSON)
	}
	b := BehaviorBinding{
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
