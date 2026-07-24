// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package desktoppet

import (
	"errors"
	"time"

	"github.com/u-ai/backend/internal/character"
	"github.com/u-ai/backend/pkg/app"
	"gorm.io/gorm"
)

const desktopPetTimeFormat = "2006-01-02 15:04:05"

type Repository interface {
	DB() *gorm.DB
	ListEnabledActions() ([]ActionDefinition, error)
	GetEnabledActionsByKeys(keys []string) ([]ActionDefinition, error)
	CreateTask(tx *gorm.DB, task *GenerationTask) error
	CreateTaskActions(tx *gorm.DB, actions []GenerationTaskAction) error
	GetTaskByID(id string) (*GenerationTask, error)
	ListActionsByTaskID(taskID string) ([]GenerationTaskAction, error)
	ListTasks(characterID, status string, page, pageSize int) ([]GenerationTask, int64, error)
	DeleteTask(tx *gorm.DB, id string) error
	DeleteActionsByTaskID(tx *gorm.DB, taskID string) error
	GetImageGenConfigByID(id int) (*imageGenConfigView, error)
	FindCharacterByID(id string) (*character.Character, error)
	CreateFrames(tx *gorm.DB, frames []GenerationFrame) error
	ListFramesByAction(taskActionID string) ([]GenerationFrame, error)
	ListFramesByTask(taskID string) ([]GenerationFrame, error)
	UpdateFrame(tx *gorm.DB, frameID string, updates map[string]interface{}) error
	ListPollingFrames(taskID string) ([]GenerationFrame, error)
	ClaimTask(taskID string, workerID string, executionID string, leaseExpiresAt string) (bool, error)
	UpdateHeartbeat(taskID string, now string) error
	RefreshLease(taskID string, leaseExpiresAt string, now string) error
	ListRecoverableTasks() ([]GenerationTask, error)
	ListQueuedTasks() ([]GenerationTask, error)
	UpdateTaskStatus(tx *gorm.DB, taskID string, updates map[string]interface{}) error
	UpdateTaskStatusNoTx(taskID string, updates map[string]interface{}) error
	SetCancelRequested(taskID string, now string) error
	GetTaskForUpdate(tx *gorm.DB, taskID string) (*GenerationTask, error)
	ListActionsByTaskIDOrdered(taskID string) ([]GenerationTaskAction, error)
	UpdateActionStatus(tx *gorm.DB, actionID string, updates map[string]interface{}) error
	UpdateActionStatusNoTx(actionID string, updates map[string]interface{}) error
	IncrementActionAttempt(tx *gorm.DB, actionID string) error
	CreateCallLog(log *GenerationCallLog) error
}

type imageGenConfigView struct {
	ID        int    `gorm:"column:id"`
	Name      string `gorm:"column:name"`
	ApiKey    string `gorm:"column:api_key"`
	ModelName string `gorm:"column:model_name"`
	BaseUrl   string `gorm:"column:base_url"`
	IsActive  int    `gorm:"column:is_active"`
	Enabled   int    `gorm:"column:enabled"`
}

func (imageGenConfigView) TableName() string { return "image_gen_configs" }

type repository struct {
	db  *gorm.DB
	ctx *app.AppContext
}

func NewRepository(db *gorm.DB, ctx *app.AppContext) Repository {
	return &repository{db: db, ctx: ctx}
}

func (r *repository) DB() *gorm.DB { return r.db }

func (r *repository) ListEnabledActions() ([]ActionDefinition, error) {
	var actions []ActionDefinition
	err := r.db.Where("enabled = ?", 1).
		Order("category_key ASC, sort_order ASC").
		Find(&actions).Error
	if actions == nil {
		actions = []ActionDefinition{}
	}
	return actions, err
}

func (r *repository) GetEnabledActionsByKeys(keys []string) ([]ActionDefinition, error) {
	var actions []ActionDefinition
	if len(keys) == 0 {
		return []ActionDefinition{}, nil
	}
	err := r.db.Where("enabled = ? AND action_key IN ?", 1, keys).
		Order("category_key ASC, sort_order ASC").
		Find(&actions).Error
	if actions == nil {
		actions = []ActionDefinition{}
	}
	return actions, err
}

func (r *repository) CreateTask(tx *gorm.DB, task *GenerationTask) error {
	return tx.Create(task).Error
}

func (r *repository) CreateTaskActions(tx *gorm.DB, actions []GenerationTaskAction) error {
	if len(actions) == 0 {
		return nil
	}
	return tx.Create(&actions).Error
}

func (r *repository) GetTaskByID(id string) (*GenerationTask, error) {
	var task GenerationTask
	err := r.db.Where("id = ?", id).First(&task).Error
	if err != nil {
		return nil, err
	}
	return &task, nil
}

func (r *repository) ListActionsByTaskID(taskID string) ([]GenerationTaskAction, error) {
	var actions []GenerationTaskAction
	err := r.db.Where("task_id = ?", taskID).
		Order("sort_order ASC").
		Find(&actions).Error
	if actions == nil {
		actions = []GenerationTaskAction{}
	}
	return actions, err
}

func (r *repository) ListTasks(characterID, status string, page, pageSize int) ([]GenerationTask, int64, error) {
	var tasks []GenerationTask
	var total int64
	q := r.db.Model(&GenerationTask{})
	if characterID != "" {
		q = q.Where("character_id = ?", characterID)
	}
	if status != "" {
		q = q.Where("status = ?", status)
	}
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
		Find(&tasks).Error
	if tasks == nil {
		tasks = []GenerationTask{}
	}
	return tasks, total, err
}

func (r *repository) DeleteTask(tx *gorm.DB, id string) error {
	return tx.Where("id = ?", id).Delete(&GenerationTask{}).Error
}

func (r *repository) DeleteActionsByTaskID(tx *gorm.DB, taskID string) error {
	return tx.Where("task_id = ?", taskID).Delete(&GenerationTaskAction{}).Error
}

func (r *repository) GetImageGenConfigByID(id int) (*imageGenConfigView, error) {
	var cfg imageGenConfigView
	err := r.db.Where("id = ?", id).First(&cfg).Error
	if err != nil {
		return nil, err
	}
	return &cfg, nil
}

func (r *repository) FindCharacterByID(id string) (*character.Character, error) {
	charRepo := character.NewRepository(r.ctx)
	c, err := charRepo.FindByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
		return nil, err
	}
	return c, nil
}

func (r *repository) CreateFrames(tx *gorm.DB, frames []GenerationFrame) error {
	if len(frames) == 0 {
		return nil
	}
	return tx.Create(&frames).Error
}

func (r *repository) ListFramesByAction(taskActionID string) ([]GenerationFrame, error) {
	var frames []GenerationFrame
	err := r.db.Where("task_action_id = ?", taskActionID).
		Order("frame_index ASC").
		Find(&frames).Error
	if frames == nil {
		frames = []GenerationFrame{}
	}
	return frames, err
}

func (r *repository) ListFramesByTask(taskID string) ([]GenerationFrame, error) {
	var frames []GenerationFrame
	err := r.db.Where("task_id = ?", taskID).
		Order("task_action_id ASC, frame_index ASC").
		Find(&frames).Error
	if frames == nil {
		frames = []GenerationFrame{}
	}
	return frames, err
}

func (r *repository) UpdateFrame(tx *gorm.DB, frameID string, updates map[string]interface{}) error {
	return tx.Model(&GenerationFrame{}).Where("id = ?", frameID).Updates(updates).Error
}

func (r *repository) ListPollingFrames(taskID string) ([]GenerationFrame, error) {
	var frames []GenerationFrame
	err := r.db.Where("task_id = ? AND status IN ? AND provider_operation_id IS NOT NULL AND provider_operation_id != ''", taskID, []string{"submitted", "polling"}).
		Order("frame_index ASC").
		Find(&frames).Error
	if frames == nil {
		frames = []GenerationFrame{}
	}
	return frames, err
}

func (r *repository) ClaimTask(taskID string, workerID string, executionID string, leaseExpiresAt string) (bool, error) {
	now := time.Now().Format(desktopPetTimeFormat)
	updates := map[string]interface{}{
		"status":            "processing",
		"worker_id":         workerID,
		"execution_id":      executionID,
		"lease_expires_at":  leaseExpiresAt,
		"last_heartbeat_at": now,
		"current_stage":     "preparing",
	}
	result := r.db.Model(&GenerationTask{}).
		Where("id = ? AND status = ?", taskID, "queued").
		Updates(updates)
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected > 0, nil
}

func (r *repository) UpdateHeartbeat(taskID string, now string) error {
	return r.db.Model(&GenerationTask{}).
		Where("id = ?", taskID).
		Update("last_heartbeat_at", now).Error
}

func (r *repository) RefreshLease(taskID string, leaseExpiresAt string, now string) error {
	return r.db.Model(&GenerationTask{}).
		Where("id = ?", taskID).
		Updates(map[string]interface{}{
			"lease_expires_at":  leaseExpiresAt,
			"last_heartbeat_at": now,
		}).Error
}

func (r *repository) ListRecoverableTasks() ([]GenerationTask, error) {
	now := time.Now().Format(desktopPetTimeFormat)
	var tasks []GenerationTask
	err := r.db.Where("status = ? AND lease_expires_at < ?", "processing", now).
		Order("lease_expires_at ASC").
		Find(&tasks).Error
	if tasks == nil {
		tasks = []GenerationTask{}
	}
	return tasks, err
}

func (r *repository) ListQueuedTasks() ([]GenerationTask, error) {
	var tasks []GenerationTask
	err := r.db.Where("status = ?", "queued").
		Order("created_at ASC").
		Find(&tasks).Error
	if tasks == nil {
		tasks = []GenerationTask{}
	}
	return tasks, err
}

func (r *repository) UpdateTaskStatus(tx *gorm.DB, taskID string, updates map[string]interface{}) error {
	return tx.Model(&GenerationTask{}).Where("id = ?", taskID).Updates(updates).Error
}

func (r *repository) UpdateTaskStatusNoTx(taskID string, updates map[string]interface{}) error {
	return r.db.Model(&GenerationTask{}).Where("id = ?", taskID).Updates(updates).Error
}

func (r *repository) SetCancelRequested(taskID string, now string) error {
	return r.db.Model(&GenerationTask{}).
		Where("id = ?", taskID).
		Update("cancel_requested_at", now).Error
}

func (r *repository) GetTaskForUpdate(tx *gorm.DB, taskID string) (*GenerationTask, error) {
	var task GenerationTask
	err := tx.Where("id = ?", taskID).First(&task).Error
	if err != nil {
		return nil, err
	}
	return &task, nil
}

func (r *repository) ListActionsByTaskIDOrdered(taskID string) ([]GenerationTaskAction, error) {
	var actions []GenerationTaskAction
	err := r.db.Where("task_id = ?", taskID).
		Order("sort_order ASC").
		Find(&actions).Error
	if actions == nil {
		actions = []GenerationTaskAction{}
	}
	return actions, err
}

func (r *repository) UpdateActionStatus(tx *gorm.DB, actionID string, updates map[string]interface{}) error {
	return tx.Model(&GenerationTaskAction{}).Where("id = ?", actionID).Updates(updates).Error
}

func (r *repository) UpdateActionStatusNoTx(actionID string, updates map[string]interface{}) error {
	return r.db.Model(&GenerationTaskAction{}).Where("id = ?", actionID).Updates(updates).Error
}

func (r *repository) IncrementActionAttempt(tx *gorm.DB, actionID string) error {
	return tx.Model(&GenerationTaskAction{}).
		Where("id = ?", actionID).
		Updates(map[string]interface{}{
			"attempt_number":  gorm.Expr("attempt_number + 1"),
			"current_attempt": gorm.Expr("current_attempt + 1"),
		}).Error
}

func (r *repository) CreateCallLog(log *GenerationCallLog) error {
	return r.db.Create(log).Error
}
