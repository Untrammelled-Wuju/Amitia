package generation

import (
	"errors"
	"fmt"

	"gorm.io/gorm"
)

var (
	ErrAttemptNotFound      = NewGenerationError(ErrCodeAttemptNotFound, "attempt not found", nil)
	ErrAttemptAlreadyActive = NewGenerationError(ErrCodeAttemptAlreadyActive, "active attempt already exists", nil)
	ErrAttemptLockFailed    = NewGenerationError(ErrCodeAttemptLockFailed, "failed to lock task action", nil)
)

var activeAttemptStatuses = []string{
	string(AttemptStatusPending),
	string(AttemptStatusPreparingReference),
	string(AttemptStatusBuildingPrompt),
	string(AttemptStatusWaitingRateLimit),
	string(AttemptStatusSubmitting),
	string(AttemptStatusPolling),
	string(AttemptStatusResultReceived),
	string(AttemptStatusPersisting),
	string(AttemptStatusUnknownSubmission),
}

type AttemptRepository interface {
	CreateAttempt(tx *gorm.DB, attempt *ActionGenerationAttempt) error
	GetAttemptByID(id string) (*ActionGenerationAttempt, error)
	ListAttemptsByActionID(taskActionID string) ([]ActionGenerationAttempt, error)
	GetActiveAttempt(taskActionID string) (*ActionGenerationAttempt, error)
	UpdateAttemptStatus(attemptID string, updates map[string]interface{}) error
	UpdateAttemptStatusOwned(tx *gorm.DB, attemptID, executionID string, updates map[string]interface{}) (bool, error)
	CountAttemptsByActionID(taskActionID string) (int, error)
	AtomicallyCreateAttempt(tx *gorm.DB, taskActionID string, attempt *ActionGenerationAttempt) (*ActionGenerationAttempt, error)
}

type attemptRepository struct {
	db *gorm.DB
}

func NewAttemptRepository(db *gorm.DB) AttemptRepository {
	return &attemptRepository{db: db}
}

func (r *attemptRepository) CreateAttempt(tx *gorm.DB, attempt *ActionGenerationAttempt) error {
	if tx == nil {
		tx = r.db
	}
	if attempt.ID == "" {
		attempt.ID = generateUUID()
	}
	if attempt.CreatedAt == "" {
		attempt.CreatedAt = nowRFC3339()
	}
	if attempt.UpdatedAt == "" {
		attempt.UpdatedAt = nowRFC3339()
	}
	if err := tx.Create(attempt).Error; err != nil {
		return fmt.Errorf("create attempt: %w", err)
	}
	return nil
}

func (r *attemptRepository) GetAttemptByID(id string) (*ActionGenerationAttempt, error) {
	var attempt ActionGenerationAttempt
	err := r.db.Where("id = ?", id).First(&attempt).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrAttemptNotFound
		}
		return nil, err
	}
	return &attempt, nil
}

func (r *attemptRepository) ListAttemptsByActionID(taskActionID string) ([]ActionGenerationAttempt, error) {
	var attempts []ActionGenerationAttempt
	err := r.db.Where("task_action_id = ?", taskActionID).Order("attempt_number ASC").Find(&attempts).Error
	if err != nil {
		return nil, err
	}
	return attempts, nil
}

func (r *attemptRepository) GetActiveAttempt(taskActionID string) (*ActionGenerationAttempt, error) {
	var attempt ActionGenerationAttempt
	err := r.db.Where("task_action_id = ? AND status IN ?", taskActionID, activeAttemptStatuses).
		Order("attempt_number DESC").First(&attempt).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &attempt, nil
}

func (r *attemptRepository) UpdateAttemptStatus(attemptID string, updates map[string]interface{}) error {
	if len(updates) == 0 {
		return nil
	}
	if _, ok := updates["updated_at"]; !ok {
		updates["updated_at"] = nowRFC3339()
	}
	result := r.db.Model(&ActionGenerationAttempt{}).Where("id = ?", attemptID).Updates(updates)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrAttemptNotFound
	}
	return nil
}

func (r *attemptRepository) UpdateAttemptStatusOwned(tx *gorm.DB, attemptID, executionID string, updates map[string]interface{}) (bool, error) {
	if tx == nil {
		tx = r.db
	}
	if len(updates) == 0 {
		return false, nil
	}
	if _, ok := updates["updated_at"]; !ok {
		updates["updated_at"] = nowRFC3339()
	}
	result := tx.Model(&ActionGenerationAttempt{}).
		Where("id = ? AND execution_id = ?", attemptID, executionID).
		Updates(updates)
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected > 0, nil
}

func (r *attemptRepository) CountAttemptsByActionID(taskActionID string) (int, error) {
	var count int64
	err := r.db.Model(&ActionGenerationAttempt{}).Where("task_action_id = ?", taskActionID).Count(&count).Error
	if err != nil {
		return 0, err
	}
	return int(count), nil
}

func (r *attemptRepository) AtomicallyCreateAttempt(tx *gorm.DB, taskActionID string, attempt *ActionGenerationAttempt) (*ActionGenerationAttempt, error) {
	if tx == nil {
		tx = r.db
	}

	var taskAction struct {
		AttemptNumber  int `gorm:"column:attempt_number"`
		CurrentAttempt int `gorm:"column:current_attempt"`
	}
	err := tx.Table("desktop_pet_generation_task_actions").
		Where("id = ?", taskActionID).
		First(&taskAction).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, NewGenerationError(ErrCodeAttemptNotFound, fmt.Sprintf("task action not found: %s", taskActionID), err)
		}
		return nil, err
	}

	var existingActive ActionGenerationAttempt
	err = tx.Where("task_action_id = ? AND status IN ?", taskActionID, activeAttemptStatuses).
		First(&existingActive).Error
	if err == nil {
		return nil, NewGenerationError(ErrCodeAttemptAlreadyActive,
			fmt.Sprintf("active attempt %s already exists for action %s", existingActive.ID, taskActionID), nil)
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	var count int64
	err = tx.Model(&ActionGenerationAttempt{}).Where("task_action_id = ?", taskActionID).Count(&count).Error
	if err != nil {
		return nil, err
	}

	newAttemptNumber := int(count) + 1
	if taskAction.AttemptNumber >= newAttemptNumber {
		newAttemptNumber = taskAction.AttemptNumber + 1
	}

	if attempt.ID == "" {
		attempt.ID = generateUUID()
	}
	attempt.TaskActionID = taskActionID
	attempt.AttemptNumber = newAttemptNumber
	if attempt.Status == "" {
		attempt.Status = string(AttemptStatusPending)
	}
	if attempt.CreatedAt == "" {
		attempt.CreatedAt = nowRFC3339()
	}
	if attempt.UpdatedAt == "" {
		attempt.UpdatedAt = nowRFC3339()
	}

	if err := tx.Create(attempt).Error; err != nil {
		return nil, fmt.Errorf("create attempt: %w", err)
	}

	err = tx.Table("desktop_pet_generation_task_actions").
		Where("id = ?", taskActionID).
		Updates(map[string]interface{}{
			"current_attempt": newAttemptNumber,
			"attempt_number":  newAttemptNumber,
			"updated_at":      nowRFC3339(),
		}).Error
	if err != nil {
		return nil, fmt.Errorf("update task action active attempt: %w", err)
	}

	return attempt, nil
}
