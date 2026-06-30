// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package character

import (
	"github.com/google/uuid"
	"github.com/u-ai/backend/pkg/app"
	"gorm.io/gorm"
)

type Repository interface {
	List(includeDisabled bool) ([]Character, error)
	FindByID(id string) (*Character, error)
	Create(c *Character) error
	Update(id string, updates map[string]interface{}) error
	Delete(id string) error
	SetActive(id string) error
	ListTemplates() ([]CharacterTemplate, error)
	FindTemplateByID(id string) (*CharacterTemplate, error)
	GetActive() (*Character, error)
}

type repository struct {
	db *gorm.DB
}

func NewRepository(ctx *app.AppContext) Repository {
	return &repository{db: ctx.DB}
}

func (r *repository) List(includeDisabled bool) ([]Character, error) {
	var chars []Character
	q := r.db.Order("sort_order, created_at")
	if !includeDisabled {
		q = q.Where("status = ?", "enabled")
	}
	err := q.Find(&chars).Error
	if chars == nil {
		chars = []Character{}
	}
	return chars, err
}

func (r *repository) FindByID(id string) (*Character, error) {
	var c Character
	err := r.db.Where("id = ?", id).First(&c).Error
	return &c, err
}

func (r *repository) Create(c *Character) error {
	if c.ID == "" {
		c.ID = uuid.New().String()
	}
	return r.db.Create(c).Error
}

func (r *repository) Update(id string, updates map[string]interface{}) error {
	return r.db.Model(&Character{}).Where("id = ?", id).Updates(updates).Error
}

func (r *repository) Delete(id string) error {
	return r.db.Where("id = ?", id).Delete(&Character{}).Error
}

func (r *repository) SetActive(id string) error {
	r.db.Model(&Character{}).Where("is_active = 1").Update("is_active", 0)
	return r.db.Model(&Character{}).Where("id = ?", id).Update("is_active", 1).Error
}

func (r *repository) FindTemplateByID(id string) (*CharacterTemplate, error) {
	var t CharacterTemplate
	err := r.db.Where("id = ?", id).First(&t).Error
	return &t, err
}

func (r *repository) ListTemplates() ([]CharacterTemplate, error) {
	var templates []CharacterTemplate
	err := r.db.Find(&templates).Error
	if templates == nil {
		templates = []CharacterTemplate{}
	}
	return templates, err
}

func (r *repository) GetActive() (*Character, error) {
	var c Character
	err := r.db.Where("is_active = 1").First(&c).Error
	return &c, err
}
