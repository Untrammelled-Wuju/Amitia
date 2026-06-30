// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package worldbook

import (
	"github.com/google/uuid"
	"github.com/u-ai/backend/pkg/app"
	"gorm.io/gorm"
	"time"
)

type Repository interface {
	List(q WorldBookListQuery) ([]WorldBookEntry, int64, error)
	FindByID(id string) (*WorldBookEntry, error)
	Create(e *WorldBookEntry) error
	Update(id string, updates map[string]interface{}) error
	Delete(id string) error
	GetAll() ([]WorldBookEntry, error)
	GetByMatchType(matchType string) ([]WorldBookEntry, error)
	IncrementHitCount(id string) error
	DeleteAll() error
}

type repository struct {
	db *gorm.DB
}

func NewRepository(ctx *app.AppContext) Repository {
	return &repository{db: ctx.DB}
}

func (r *repository) List(q WorldBookListQuery) ([]WorldBookEntry, int64, error) {
	query := r.db.Model(&WorldBookEntry{})
	if q.MatchType != "" {
		query = query.Where("match_type = ?", q.MatchType)
	}
	var total int64
	query.Count(&total)
	if q.Page <= 0 {
		q.Page = 1
	}
	if q.PageSize <= 0 {
		q.PageSize = 20
	}
	var items []WorldBookEntry
	err := query.Order("priority DESC, created_at DESC").Offset((q.Page - 1) * q.PageSize).Limit(q.PageSize).Find(&items).Error
	if items == nil {
		items = []WorldBookEntry{}
	}
	return items, total, err
}

func (r *repository) FindByID(id string) (*WorldBookEntry, error) {
	var e WorldBookEntry
	err := r.db.Where("id = ?", id).First(&e).Error
	return &e, err
}

func (r *repository) Create(e *WorldBookEntry) error {
	if e.ID == "" {
		e.ID = uuid.New().String()
	}
	return r.db.Create(e).Error
}

func (r *repository) Update(id string, updates map[string]interface{}) error {
	if len(updates) == 0 {
		return nil
	}
	updates["updated_at"] = time.Now().Format("2006-01-02 15:04:05")
	return r.db.Model(&WorldBookEntry{}).Where("id = ?", id).Updates(updates).Error
}

func (r *repository) Delete(id string) error {
	return r.db.Where("id = ?", id).Delete(&WorldBookEntry{}).Error
}

func (r *repository) GetAll() ([]WorldBookEntry, error) {
	var items []WorldBookEntry
	err := r.db.Order("priority DESC, created_at DESC").Find(&items).Error
	if items == nil {
		items = []WorldBookEntry{}
	}
	return items, err
}

func (r *repository) GetByMatchType(matchType string) ([]WorldBookEntry, error) {
	var items []WorldBookEntry
	query := r.db.Order("priority DESC, created_at DESC")
	if matchType != "" {
		query = query.Where("match_type = ?", matchType)
	}
	err := query.Find(&items).Error
	if items == nil {
		items = []WorldBookEntry{}
	}
	return items, err
}

func (r *repository) IncrementHitCount(id string) error {
	return r.db.Model(&WorldBookEntry{}).Where("id = ?", id).UpdateColumn("hit_count", gorm.Expr("hit_count + 1")).Error
}

func (r *repository) DeleteAll() error {
	return r.db.Where("1 = 1").Delete(&WorldBookEntry{}).Error
}
