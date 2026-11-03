// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package imagegen

import (
	"github.com/u-ai/backend/internal/configwrite"
	"gorm.io/gorm"
)

type Repository interface {
	List() ([]ImageGenConfig, error)
	GetByID(id int) (*ImageGenConfig, error)
	Create(cfg *ImageGenConfig) error
	Update(id int, updates map[string]interface{}) error
	Delete(id int) error
	Activate(id int) error
	GetActive() (*ImageGenConfig, error)
}

type repository struct{ db *gorm.DB }

func NewRepository(db *gorm.DB) Repository { return &repository{db: db} }

func (r *repository) List() ([]ImageGenConfig, error) {
	var configs []ImageGenConfig
	err := r.db.Order("is_active DESC, created_at DESC").Find(&configs).Error
	if configs == nil {
		configs = []ImageGenConfig{}
	}
	return configs, err
}

func (r *repository) GetByID(id int) (*ImageGenConfig, error) {
	var cfg ImageGenConfig
	err := r.db.Where("id = ?", id).First(&cfg).Error
	return &cfg, err
}

func (r *repository) Create(cfg *ImageGenConfig) error {
	return configwrite.Create(r.db, cfg, cfg.IsActive == 1)
}

func (r *repository) Update(id int, updates map[string]interface{}) error {
	return configwrite.Update[ImageGenConfig](r.db, id, updates)
}

func (r *repository) Delete(id int) error {
	return configwrite.Delete[ImageGenConfig](r.db, id)
}

func (r *repository) Activate(id int) error {
	return configwrite.Activate[ImageGenConfig](r.db, id)
}

func (r *repository) GetActive() (*ImageGenConfig, error) {
	var cfg ImageGenConfig
	err := r.db.Where("is_active = 1").First(&cfg).Error
	return &cfg, err
}
