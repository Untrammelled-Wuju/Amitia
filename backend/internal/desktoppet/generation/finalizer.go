package generation

import (
	"errors"
	"fmt"

	"github.com/u-ai/backend/internal/desktoppet/generation/activebinding"
	"gorm.io/gorm"
)

type FinalizeAttemptRequest struct {
	Tx                *gorm.DB
	AttemptID         string
	TaskActionID      string
	TaskID            string
	PrimaryArtifactID string
	ArtifactHash      string
	ExecutionID       string
	ActualCost        float64
	ActualInputUnits  int
	ActualOutputUnits int
	AutoPromote       bool
}

type GenerationFinalizer struct {
	attemptRepo    AttemptRepository
	artifactRepo   ArtifactRepository
	bindingService *activebinding.BindingService
}

func NewGenerationFinalizer(attemptRepo AttemptRepository, artifactRepo ArtifactRepository, bindingService *activebinding.BindingService) *GenerationFinalizer {
	return &GenerationFinalizer{
		attemptRepo:    attemptRepo,
		artifactRepo:   artifactRepo,
		bindingService: bindingService,
	}
}

func (f *GenerationFinalizer) FinalizeAttempt(req FinalizeAttemptRequest) error {
	if req.Tx == nil {
		return NewGenerationError(ErrCodeFinalizeFailed, "transaction is nil", nil)
	}
	now := nowRFC3339()
	if err := f.markAttemptSucceeded(req.Tx, req.AttemptID, now); err != nil {
		return NewGenerationError(ErrCodeFinalizeFailed, fmt.Sprintf("mark attempt succeeded: %v", err), err)
	}
	actionTransitioned, err := f.markActionSucceeded(req.Tx, req.TaskActionID, now)
	if err != nil {
		return NewGenerationError(ErrCodeFinalizeFailed, fmt.Sprintf("mark action succeeded: %v", err), err)
	}
	if req.PrimaryArtifactID != "" {
		if err := f.markArtifactPersisted(req.Tx, req.PrimaryArtifactID, now); err != nil {
			return NewGenerationError(ErrCodeFinalizeFailed, fmt.Sprintf("mark artifact persisted: %v", err), err)
		}
	}
	if req.AutoPromote && req.PrimaryArtifactID != "" {
		if err := f.promoteActiveBinding(req); err != nil {
			return err
		}
	}
	if err := f.applyActionCostStats(req.Tx, req.TaskActionID, req, actionTransitioned); err != nil {
		return NewGenerationError(ErrCodeFinalizeFailed, fmt.Sprintf("apply action cost stats: %v", err), err)
	}
	if err := f.applyAttemptCostStats(req.Tx, req.AttemptID, req); err != nil {
		return NewGenerationError(ErrCodeFinalizeFailed, fmt.Sprintf("apply attempt cost stats: %v", err), err)
	}
	return nil
}

func (f *GenerationFinalizer) markAttemptSucceeded(tx *gorm.DB, attemptID, now string) error {
	result := tx.Model(&ActionGenerationAttempt{}).
		Where("id = ?", attemptID).
		Updates(map[string]interface{}{
			"status":       string(AttemptStatusSucceeded),
			"completed_at": now,
			"updated_at":   now,
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrAttemptNotFound
	}
	return nil
}

func (f *GenerationFinalizer) markActionSucceeded(tx *gorm.DB, taskActionID, now string) (bool, error) {
	var current struct {
		Status string `gorm:"column:status"`
	}
	if err := tx.Table("desktop_pet_generation_task_actions").
		Select("status").
		Where("id = ?", taskActionID).
		First(&current).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, NewGenerationError(ErrCodeFinalizeFailed, fmt.Sprintf("task action not found: %s", taskActionID), err)
		}
		return false, err
	}
	if current.Status == "succeeded" {
		return false, nil
	}
	result := tx.Table("desktop_pet_generation_task_actions").
		Where("id = ?", taskActionID).
		Updates(map[string]interface{}{
			"status":        "succeeded",
			"progress":      100,
			"error_code":    "",
			"error_message": "",
			"completed_at":  now,
			"updated_at":    now,
		})
	if result.Error != nil {
		return false, result.Error
	}
	if result.RowsAffected == 0 {
		return false, NewGenerationError(ErrCodeFinalizeFailed, fmt.Sprintf("task action not found: %s", taskActionID), nil)
	}
	return true, nil
}

func (f *GenerationFinalizer) markArtifactPersisted(tx *gorm.DB, artifactID, now string) error {
	result := tx.Model(&GenerationArtifact{}).
		Where("id = ? AND status IN ?", artifactID, []string{string(ArtifactStatusStaging), string(ArtifactStatusSaved)}).
		Updates(map[string]interface{}{
			"status":     string(ArtifactStatusPersisted),
			"updated_at": now,
		})
	return result.Error
}

func (f *GenerationFinalizer) promoteActiveBinding(req FinalizeAttemptRequest) error {
	err := f.bindingService.BindActiveArtifact(req.Tx, activebinding.BindRequest{
		TaskActionID:      req.TaskActionID,
		AttemptID:         req.AttemptID,
		PrimaryArtifactID: req.PrimaryArtifactID,
		ArtifactHash:      req.ArtifactHash,
		Reason:            "finalize_promote",
	})
	if err != nil {
		if errors.Is(err, activebinding.ErrBindingCASConflict) {
			return NewGenerationError(ErrCodeActiveBindingCASConflict, "active binding revision conflict", err)
		}
		return NewGenerationError(ErrCodeFinalizeFailed, fmt.Sprintf("promote active binding: %v", err), err)
	}
	return nil
}

func (f *GenerationFinalizer) applyActionCostStats(tx *gorm.DB, taskActionID string, req FinalizeAttemptRequest, incrementSuccess bool) error {
	updates := map[string]interface{}{
		"actual_cost":         req.ActualCost,
		"actual_input_units":  req.ActualInputUnits,
		"actual_output_units": req.ActualOutputUnits,
		"updated_at":          nowRFC3339(),
	}
	if incrementSuccess {
		updates["actual_success_count"] = gorm.Expr("actual_success_count + 1")
	}
	result := tx.Table("desktop_pet_generation_task_actions").
		Where("id = ?", taskActionID).
		Updates(updates)
	return result.Error
}

func (f *GenerationFinalizer) applyAttemptCostStats(tx *gorm.DB, attemptID string, req FinalizeAttemptRequest) error {
	result := tx.Model(&ActionGenerationAttempt{}).
		Where("id = ?", attemptID).
		Updates(map[string]interface{}{
			"actual_cost":         req.ActualCost,
			"actual_input_units":  req.ActualInputUnits,
			"actual_output_units": req.ActualOutputUnits,
			"updated_at":          nowRFC3339(),
		})
	return result.Error
}
