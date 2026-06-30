// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package embedding_config

import "gorm.io/gorm"

type Repository interface {
	List() ([]EmbeddingConfig, error)
	GetByID(id int) (*EmbeddingConfig, error)
	Create(cfg *EmbeddingConfig) error
	Update(id int, updates map[string]interface{}) error
	Delete(id int) error
	Activate(id int) error
	GetActive() (*EmbeddingConfig, error)
}

type repository struct{ db *gorm.DB }

func NewRepository(db *gorm.DB) Repository { return &repository{db: db} }

func (r *repository) List() ([]EmbeddingConfig, error) {
	var configs []EmbeddingConfig
	err := r.db.Order("is_active DESC, created_at DESC").Find(&configs).Error
	if configs == nil {
		configs = []EmbeddingConfig{}
	}
	return configs, err
}

func (r *repository) GetByID(id int) (*EmbeddingConfig, error) {
	var cfg EmbeddingConfig
	err := r.db.Where("id = ?", id).First(&cfg).Error
	return &cfg, err
}

func (r *repository) Create(cfg *EmbeddingConfig) error { return r.db.Create(cfg).Error }

func (r *repository) Update(id int, updates map[string]interface{}) error {
	return r.db.Model(&EmbeddingConfig{}).Where("id = ?", id).Updates(updates).Error
}

func (r *repository) Delete(id int) error {
	return r.db.Where("id = ?", id).Delete(&EmbeddingConfig{}).Error
}

func (r *repository) Activate(id int) error {
	r.db.Model(&EmbeddingConfig{}).Where("is_active = 1").Update("is_active", 0)
	return r.db.Model(&EmbeddingConfig{}).Where("id = ?", id).Update("is_active", 1).Error
}

func (r *repository) GetActive() (*EmbeddingConfig, error) {
	var cfg EmbeddingConfig
	err := r.db.Where("is_active = 1").First(&cfg).Error
	return &cfg, err
}
