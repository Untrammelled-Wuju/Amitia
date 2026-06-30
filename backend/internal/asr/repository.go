// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package asr

import "gorm.io/gorm"

type Repository interface {
	List() ([]AsrConfig, error)
	GetByID(id int) (*AsrConfig, error)
	Create(cfg *AsrConfig) error
	Update(id int, updates map[string]interface{}) error
	Delete(id int) error
	Activate(id int) error
	GetActive() (*AsrConfig, error)
}

type repository struct{ db *gorm.DB }

func NewRepository(db *gorm.DB) Repository { return &repository{db: db} }

func (r *repository) List() ([]AsrConfig, error) {
	var configs []AsrConfig
	err := r.db.Order("is_active DESC, created_at DESC").Find(&configs).Error
	if configs == nil {
		configs = []AsrConfig{}
	}
	return configs, err
}

func (r *repository) GetByID(id int) (*AsrConfig, error) {
	var cfg AsrConfig
	err := r.db.Where("id = ?", id).First(&cfg).Error
	return &cfg, err
}

func (r *repository) Create(cfg *AsrConfig) error { return r.db.Create(cfg).Error }

func (r *repository) Update(id int, updates map[string]interface{}) error {
	return r.db.Model(&AsrConfig{}).Where("id = ?", id).Updates(updates).Error
}

func (r *repository) Delete(id int) error {
	return r.db.Where("id = ?", id).Delete(&AsrConfig{}).Error
}

func (r *repository) Activate(id int) error {
	r.db.Model(&AsrConfig{}).Where("is_active = 1").Update("is_active", 0)
	return r.db.Model(&AsrConfig{}).Where("id = ?", id).Update("is_active", 1).Error
}

func (r *repository) GetActive() (*AsrConfig, error) {
	var cfg AsrConfig
	err := r.db.Where("is_active = 1").First(&cfg).Error
	return &cfg, err
}
