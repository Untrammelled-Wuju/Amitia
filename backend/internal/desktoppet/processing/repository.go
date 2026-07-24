// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package processing

import (
	"time"

	"github.com/u-ai/backend/internal/desktoppet"
	"github.com/u-ai/backend/pkg/app"
	"gorm.io/gorm"
)

const desktopPetTimeFormat = "2006-01-02 15:04:05"

type Repository interface {
	DB() *gorm.DB

	CreateProcessingTask(tx *gorm.DB, task *ProcessingTask) error
	GetProcessingTask(id string) (*ProcessingTask, error)
	ListProcessingTasksByGenerationTask(generationTaskID string) ([]ProcessingTask, error)
	UpdateProcessingTaskStatus(tx *gorm.DB, taskID string, updates map[string]interface{}) error
	UpdateProcessingTaskStatusNoTx(taskID string, updates map[string]interface{}) error
	ClaimProcessingTask(taskID string, workerID string, executionID string, leaseExpiresAt string) (bool, error)
	UpdateProcessingHeartbeat(taskID string, now string) error
	RefreshProcessingLease(taskID string, leaseExpiresAt string, now string) error
	ListRecoverableProcessingTasks() ([]ProcessingTask, error)
	ListQueuedProcessingTasks() ([]ProcessingTask, error)
	SetProcessingCancelRequested(taskID string, now string) error
	GetProcessingTaskForUpdate(tx *gorm.DB, taskID string) (*ProcessingTask, error)

	CreateProcessingActions(tx *gorm.DB, actions []ProcessingAction) error
	ListProcessingActions(processingTaskID string) ([]ProcessingAction, error)
	ListProcessingActionsOrdered(processingTaskID string) ([]ProcessingAction, error)
	GetProcessingActionByActionKey(processingTaskID, actionKey string) (*ProcessingAction, error)
	UpdateProcessingAction(tx *gorm.DB, actionID string, updates map[string]interface{}) error
	UpdateProcessingActionNoTx(actionID string, updates map[string]interface{}) error
	UpdateProcessingActionAttempt(tx *gorm.DB, actionID string, attemptNumber int) error
	SetActionExcluded(actionID string, excluded bool) error

	CreateProcessedFrames(tx *gorm.DB, frames []ProcessedFrame) error
	ListProcessedFramesByAction(processingActionID string) ([]ProcessedFrame, error)
	UpdateProcessedFrame(tx *gorm.DB, frameID string, updates map[string]interface{}) error
	UpdateProcessedFrameNoTx(frameID string, updates map[string]interface{}) error

	CreatePackage(pkg *Package) error
	GetPackage(id string) (*Package, error)
	UpdatePackageStatus(id string, updates map[string]interface{}) error
	ListPackagesByUser(userID string, page, pageSize int) ([]Package, int64, error)
	ListPackagesByGenerationTask(generationTaskID string) ([]Package, error)

	GetGenerationTask(generationTaskID string) (*desktoppet.GenerationTask, error)
	ListActionsByTaskID(taskID string) ([]desktoppet.GenerationTaskAction, error)
	ListFramesByAction(taskActionID string) ([]desktoppet.GenerationFrame, error)
	ListSucceededActions(taskID string) ([]desktoppet.GenerationTaskAction, error)
}

type repository struct {
	db  *gorm.DB
	ctx *app.AppContext
}

func NewRepository(db *gorm.DB, ctx *app.AppContext) Repository {
	return &repository{db: db, ctx: ctx}
}

func (r *repository) DB() *gorm.DB { return r.db }

func (r *repository) CreateProcessingTask(tx *gorm.DB, task *ProcessingTask) error {
	return tx.Create(task).Error
}

func (r *repository) GetProcessingTask(id string) (*ProcessingTask, error) {
	var task ProcessingTask
	err := r.db.Where("id = ?", id).First(&task).Error
	if err != nil {
		return nil, err
	}
	return &task, nil
}

func (r *repository) ListProcessingTasksByGenerationTask(generationTaskID string) ([]ProcessingTask, error) {
	var tasks []ProcessingTask
	err := r.db.Where("generation_task_id = ?", generationTaskID).
		Order("created_at DESC").
		Find(&tasks).Error
	if tasks == nil {
		tasks = []ProcessingTask{}
	}
	return tasks, err
}

func (r *repository) UpdateProcessingTaskStatus(tx *gorm.DB, taskID string, updates map[string]interface{}) error {
	return tx.Model(&ProcessingTask{}).Where("id = ?", taskID).Updates(updates).Error
}

func (r *repository) UpdateProcessingTaskStatusNoTx(taskID string, updates map[string]interface{}) error {
	return r.db.Model(&ProcessingTask{}).Where("id = ?", taskID).Updates(updates).Error
}

func (r *repository) ClaimProcessingTask(taskID string, workerID string, executionID string, leaseExpiresAt string) (bool, error) {
	now := time.Now().Format(desktopPetTimeFormat)
	updates := map[string]interface{}{
		"status":            "processing",
		"worker_id":         workerID,
		"execution_id":      executionID,
		"lease_expires_at":  leaseExpiresAt,
		"last_heartbeat_at": now,
		"current_stage":     "validating_sources",
	}
	result := r.db.Model(&ProcessingTask{}).
		Where("id = ? AND status = ?", taskID, "queued").
		Updates(updates)
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected > 0, nil
}

func (r *repository) UpdateProcessingHeartbeat(taskID string, now string) error {
	return r.db.Model(&ProcessingTask{}).
		Where("id = ?", taskID).
		Update("last_heartbeat_at", now).Error
}

func (r *repository) RefreshProcessingLease(taskID string, leaseExpiresAt string, now string) error {
	return r.db.Model(&ProcessingTask{}).
		Where("id = ?", taskID).
		Updates(map[string]interface{}{
			"lease_expires_at":  leaseExpiresAt,
			"last_heartbeat_at": now,
		}).Error
}

func (r *repository) ListRecoverableProcessingTasks() ([]ProcessingTask, error) {
	now := time.Now().Format(desktopPetTimeFormat)
	var tasks []ProcessingTask
	err := r.db.Where("status = ? AND lease_expires_at < ?", "processing", now).
		Order("lease_expires_at ASC").
		Find(&tasks).Error
	if tasks == nil {
		tasks = []ProcessingTask{}
	}
	return tasks, err
}

func (r *repository) ListQueuedProcessingTasks() ([]ProcessingTask, error) {
	var tasks []ProcessingTask
	err := r.db.Where("status = ?", "queued").
		Order("created_at ASC").
		Find(&tasks).Error
	if tasks == nil {
		tasks = []ProcessingTask{}
	}
	return tasks, err
}

func (r *repository) SetProcessingCancelRequested(taskID string, now string) error {
	return r.db.Model(&ProcessingTask{}).
		Where("id = ?", taskID).
		Update("cancel_requested_at", now).Error
}

func (r *repository) GetProcessingTaskForUpdate(tx *gorm.DB, taskID string) (*ProcessingTask, error) {
	var task ProcessingTask
	err := tx.Where("id = ?", taskID).First(&task).Error
	if err != nil {
		return nil, err
	}
	return &task, nil
}

func (r *repository) CreateProcessingActions(tx *gorm.DB, actions []ProcessingAction) error {
	if len(actions) == 0 {
		return nil
	}
	return tx.Create(&actions).Error
}

func (r *repository) ListProcessingActions(processingTaskID string) ([]ProcessingAction, error) {
	var actions []ProcessingAction
	err := r.db.Where("processing_task_id = ?", processingTaskID).
		Find(&actions).Error
	if actions == nil {
		actions = []ProcessingAction{}
	}
	return actions, err
}

func (r *repository) ListProcessingActionsOrdered(processingTaskID string) ([]ProcessingAction, error) {
	var actions []ProcessingAction
	err := r.db.Where("processing_task_id = ?", processingTaskID).
		Order("created_at ASC").
		Find(&actions).Error
	if actions == nil {
		actions = []ProcessingAction{}
	}
	return actions, err
}

func (r *repository) GetProcessingActionByActionKey(processingTaskID, actionKey string) (*ProcessingAction, error) {
	var action ProcessingAction
	err := r.db.Where("processing_task_id = ? AND action_key = ?", processingTaskID, actionKey).First(&action).Error
	if err != nil {
		return nil, err
	}
	return &action, nil
}

func (r *repository) UpdateProcessingAction(tx *gorm.DB, actionID string, updates map[string]interface{}) error {
	return tx.Model(&ProcessingAction{}).Where("id = ?", actionID).Updates(updates).Error
}

func (r *repository) UpdateProcessingActionNoTx(actionID string, updates map[string]interface{}) error {
	return r.db.Model(&ProcessingAction{}).Where("id = ?", actionID).Updates(updates).Error
}

func (r *repository) UpdateProcessingActionAttempt(tx *gorm.DB, actionID string, attemptNumber int) error {
	return tx.Model(&ProcessingAction{}).
		Where("id = ?", actionID).
		Update("source_attempt_number", attemptNumber).Error
}

func (r *repository) SetActionExcluded(actionID string, excluded bool) error {
	excludedVal := 0
	if excluded {
		excludedVal = 1
	}
	return r.db.Model(&ProcessingAction{}).
		Where("id = ?", actionID).
		Update("excluded", excludedVal).Error
}

func (r *repository) CreateProcessedFrames(tx *gorm.DB, frames []ProcessedFrame) error {
	if len(frames) == 0 {
		return nil
	}
	return tx.Create(&frames).Error
}

func (r *repository) ListProcessedFramesByAction(processingActionID string) ([]ProcessedFrame, error) {
	var frames []ProcessedFrame
	err := r.db.Where("processing_action_id = ?", processingActionID).
		Order("frame_index ASC").
		Find(&frames).Error
	if frames == nil {
		frames = []ProcessedFrame{}
	}
	return frames, err
}

func (r *repository) UpdateProcessedFrame(tx *gorm.DB, frameID string, updates map[string]interface{}) error {
	return tx.Model(&ProcessedFrame{}).Where("id = ?", frameID).Updates(updates).Error
}

func (r *repository) UpdateProcessedFrameNoTx(frameID string, updates map[string]interface{}) error {
	return r.db.Model(&ProcessedFrame{}).Where("id = ?", frameID).Updates(updates).Error
}

func (r *repository) CreatePackage(pkg *Package) error {
	return r.db.Create(pkg).Error
}

func (r *repository) GetPackage(id string) (*Package, error) {
	var pkg Package
	err := r.db.Where("id = ?", id).First(&pkg).Error
	if err != nil {
		return nil, err
	}
	return &pkg, nil
}

func (r *repository) UpdatePackageStatus(id string, updates map[string]interface{}) error {
	return r.db.Model(&Package{}).Where("id = ?", id).Updates(updates).Error
}

func (r *repository) ListPackagesByUser(userID string, page, pageSize int) ([]Package, int64, error) {
	var packages []Package
	var total int64
	q := r.db.Model(&Package{}).Where("user_id = ?", userID)
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize
	err := q.Order("created_at DESC").
		Offset(offset).
		Limit(pageSize).
		Find(&packages).Error
	if packages == nil {
		packages = []Package{}
	}
	return packages, total, err
}

func (r *repository) ListPackagesByGenerationTask(generationTaskID string) ([]Package, error) {
	var packages []Package
	err := r.db.Where("generation_task_id = ?", generationTaskID).
		Order("version DESC").
		Find(&packages).Error
	if packages == nil {
		packages = []Package{}
	}
	return packages, err
}

func (r *repository) GetGenerationTask(generationTaskID string) (*desktoppet.GenerationTask, error) {
	var task desktoppet.GenerationTask
	err := r.db.Where("id = ?", generationTaskID).First(&task).Error
	if err != nil {
		return nil, err
	}
	return &task, nil
}

func (r *repository) ListActionsByTaskID(taskID string) ([]desktoppet.GenerationTaskAction, error) {
	var actions []desktoppet.GenerationTaskAction
	err := r.db.Where("task_id = ?", taskID).
		Order("sort_order ASC").
		Find(&actions).Error
	if actions == nil {
		actions = []desktoppet.GenerationTaskAction{}
	}
	return actions, err
}

func (r *repository) ListFramesByAction(taskActionID string) ([]desktoppet.GenerationFrame, error) {
	var frames []desktoppet.GenerationFrame
	err := r.db.Where("task_action_id = ?", taskActionID).
		Order("frame_index ASC").
		Find(&frames).Error
	if frames == nil {
		frames = []desktoppet.GenerationFrame{}
	}
	return frames, err
}

func (r *repository) ListSucceededActions(taskID string) ([]desktoppet.GenerationTaskAction, error) {
	var actions []desktoppet.GenerationTaskAction
	err := r.db.Where("task_id = ? AND status = ?", taskID, "succeeded").
		Order("sort_order ASC").
		Find(&actions).Error
	if actions == nil {
		actions = []desktoppet.GenerationTaskAction{}
	}
	return actions, err
}
