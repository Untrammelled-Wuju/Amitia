// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package worldbook

import (
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/u-ai/backend/pkg/app"
	"gorm.io/gorm"
)

type Repository interface {
	List(q WorldBookListQuery, spaceID string) ([]WorldBookEntry, int64, error)
	FindByID(id, spaceID string) (*WorldBookEntry, error)
	Create(e *WorldBookEntry) error
	Update(id, spaceID string, updates map[string]interface{}) error
	Delete(id, spaceID string) error
	GetAll(spaceID string) ([]WorldBookEntry, error)
	GetByCharacterID(spaceID, characterID string) ([]WorldBookEntry, error)
	GetByMatchType(spaceID, matchType string) ([]WorldBookEntry, error)
	IncrementHitCount(id, spaceID string) error
	DeleteAll(spaceID string) error
}

type repository struct {
	db *gorm.DB
}

func NewRepository(ctx *app.AppContext) Repository {
	return &repository{db: ctx.DB}
}

func (r *repository) List(q WorldBookListQuery, spaceID string) ([]WorldBookEntry, int64, error) {
	query := worldbookOwnerScope(r.db.Model(&WorldBookEntry{}), "space_id", spaceID)
	if q.MatchType != "" {
		query = query.Where("match_type = ?", q.MatchType)
	}
	if q.CharacterID != "" {
		query = query.Where("character_id = ? OR character_id = ''", q.CharacterID)
	}
	if keyword := strings.TrimSpace(q.Keyword); keyword != "" {
		like := "%" + strings.ToLower(keyword) + "%"
		query = query.Where(
			"LOWER(match_pattern) LIKE ? OR LOWER(inject_content) LIKE ? OR LOWER(match_type) LIKE ? OR LOWER(match_scope) LIKE ?",
			like, like, like, like,
		)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
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

func (r *repository) FindByID(id, spaceID string) (*WorldBookEntry, error) {
	var e WorldBookEntry
	query := worldbookOwnerScope(r.db.Where("id = ?", id), "space_id", spaceID)
	err := query.First(&e).Error
	return &e, err
}

func (r *repository) Create(e *WorldBookEntry) error {
	if e.ID == "" {
		e.ID = uuid.New().String()
	}
	e.SpaceID = normalizeWorldbookOwner(e.SpaceID)
	return r.db.Create(e).Error
}

func (r *repository) Update(id, spaceID string, updates map[string]interface{}) error {
	if len(updates) == 0 {
		return nil
	}
	updates["updated_at"] = time.Now().Format("2006-01-02 15:04:05")
	query := worldbookOwnerScope(r.db.Model(&WorldBookEntry{}).Where("id = ?", id), "space_id", spaceID)
	result := query.Updates(updates)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *repository) Delete(id, spaceID string) error {
	query := worldbookOwnerScope(r.db.Where("id = ?", id), "space_id", spaceID)
	result := query.Delete(&WorldBookEntry{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *repository) GetAll(spaceID string) ([]WorldBookEntry, error) {
	var items []WorldBookEntry
	query := worldbookOwnerScope(r.db.Order("priority DESC, created_at DESC"), "space_id", spaceID)
	err := query.Find(&items).Error
	if items == nil {
		items = []WorldBookEntry{}
	}
	return items, err
}

func (r *repository) GetByCharacterID(spaceID, characterID string) ([]WorldBookEntry, error) {
	var items []WorldBookEntry
	query := worldbookOwnerScope(r.db.Order("priority DESC, created_at DESC"), "space_id", spaceID)
	if characterID != "" {
		query = query.Where("character_id = ? OR character_id = ''", characterID)
	} else {
		query = query.Where("character_id = ''")
	}
	err := query.Find(&items).Error
	if items == nil {
		items = []WorldBookEntry{}
	}
	return items, err
}

func (r *repository) GetByMatchType(spaceID, matchType string) ([]WorldBookEntry, error) {
	var items []WorldBookEntry
	query := worldbookOwnerScope(r.db.Order("priority DESC, created_at DESC"), "space_id", spaceID)
	if matchType != "" {
		query = query.Where("match_type = ?", matchType)
	}
	err := query.Find(&items).Error
	if items == nil {
		items = []WorldBookEntry{}
	}
	return items, err
}

func (r *repository) IncrementHitCount(id, spaceID string) error {
	query := worldbookOwnerScope(r.db.Model(&WorldBookEntry{}).Where("id = ?", id), "space_id", spaceID)
	return query.UpdateColumn("hit_count", gorm.Expr("hit_count + 1")).Error
}

func (r *repository) DeleteAll(spaceID string) error {
	query := worldbookOwnerScope(r.db.Where("1 = 1"), "space_id", spaceID)
	return query.Delete(&WorldBookEntry{}).Error
}
