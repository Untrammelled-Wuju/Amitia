package generation

import (
	"fmt"

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

type ActiveBindingFinalizerRepo interface {
	CASUpdate(tx *gorm.DB, actionID string, expectedRevision int, attemptID, artifactID, hash, reason string) (bool, error)
	GetByActionID(actionID string) (revision int, exists bool, err error)
}

type GenerationFinalizer struct {
	attemptRepo       AttemptRepository
	artifactRepo      ArtifactRepository
	activeBindingRepo ActiveBindingFinalizerRepo
}

func NewGenerationFinalizer(attemptRepo AttemptRepository, artifactRepo ArtifactRepository, activeBindingRepo ActiveBindingFinalizerRepo) *GenerationFinalizer {
	return &GenerationFinalizer{
		attemptRepo:       attemptRepo,
		artifactRepo:      artifactRepo,
		activeBindingRepo: activeBindingRepo,
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
	if err := f.markActionSucceeded(req.Tx, req.TaskActionID, now); err != nil {
		return NewGenerationError(ErrCodeFinalizeFailed, fmt.Sprintf("mark action succeeded: %v", err), err)
	}
	if req.PrimaryArtifactID != "" {
		if err := f.markArtifactPersisted(req.Tx, req.PrimaryArtifactID, now); err != nil {
			return NewGenerationError(ErrCodeFinalizeFailed, fmt.Sprintf("mark artifact persisted: %v", err), err)
		}
	}
	if req.AutoPromote {
		if err := f.promoteActiveBinding(req); err != nil {
			return err
		}
	}
	if err := f.applyActionCostStats(req.Tx, req.TaskActionID, req); err != nil {
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

func (f *GenerationFinalizer) markActionSucceeded(tx *gorm.DB, taskActionID, now string) error {
	result := tx.Table("desktop_pet_generation_task_actions").
		Where("id = ?", taskActionID).
		Updates(map[string]interface{}{
			"status":       "succeeded",
			"progress":     100,
			"completed_at": now,
			"updated_at":   now,
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return NewGenerationError(ErrCodeFinalizeFailed, fmt.Sprintf("task action not found: %s", taskActionID), nil)
	}
	return nil
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
	revision, exists, err := f.activeBindingRepo.GetByActionID(req.TaskActionID)
	if err != nil {
		return NewGenerationError(ErrCodeFinalizeFailed, fmt.Sprintf("get active binding: %v", err), err)
	}
	if !exists {
		revision = 0
	}
	reason := "finalize_promote"
	ok, err := f.activeBindingRepo.CASUpdate(req.Tx, req.TaskActionID, revision, req.AttemptID, req.PrimaryArtifactID, req.ArtifactHash, reason)
	if err != nil {
		return NewGenerationError(ErrCodeFinalizeFailed, fmt.Sprintf("cas update active binding: %v", err), err)
	}
	if !ok {
		return NewGenerationError(ErrCodeActiveBindingCASConflict, "active binding revision conflict", nil)
	}
	return nil
}

func (f *GenerationFinalizer) applyActionCostStats(tx *gorm.DB, taskActionID string, req FinalizeAttemptRequest) error {
	result := tx.Table("desktop_pet_generation_task_actions").
		Where("id = ?", taskActionID).
		Updates(map[string]interface{}{
			"actual_cost":          req.ActualCost,
			"actual_input_units":   req.ActualInputUnits,
			"actual_output_units":  req.ActualOutputUnits,
			"actual_success_count": gorm.Expr("actual_success_count + 1"),
			"updated_at":           nowRFC3339(),
		})
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
