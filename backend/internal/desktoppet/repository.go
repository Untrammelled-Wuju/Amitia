// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package desktoppet

import (
	"errors"

	"github.com/u-ai/backend/internal/character"
	"github.com/u-ai/backend/pkg/app"
	"gorm.io/gorm"
)

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
}

type imageGenConfigView struct {
	ID        int    `gorm:"column:id"`
	Name      string `gorm:"column:name"`
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
