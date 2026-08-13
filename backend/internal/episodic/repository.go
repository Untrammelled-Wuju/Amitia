// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package episodic

import (
	"github.com/google/uuid"
	"github.com/u-ai/backend/pkg/app"
	"gorm.io/gorm"
)

type EpisodicTimeFilter struct {
	TimeFrom string
	TimeTo   string
	Basis    string
}

type Repository interface {
	List(q EpisodicListQuery) ([]EpisodicMemory, int64, error)
	FindByID(id string) (*EpisodicMemory, error)
	Create(m *EpisodicMemory) error
	Delete(id string) error
	GetByUserID(userID string, limit int) ([]EpisodicMemory, error)
	GetRecent(userID string, limit int) ([]EpisodicMemory, error)
	GetRecentScoped(userID string, limit int) ([]EpisodicMemory, error)
	GetDetailWithMessages(id string, db *gorm.DB) (*EpisodicMemory, []map[string]interface{}, error)
	FindByTime(filter EpisodicTimeFilter, userID string, limit int) ([]EpisodicMemory, int64, error)
}

type repository struct {
	db *gorm.DB
}

func NewRepository(ctx *app.AppContext) Repository {
	return &repository{db: ctx.DB}
}

func (r *repository) List(q EpisodicListQuery) ([]EpisodicMemory, int64, error) {
	query := r.db.Model(&EpisodicMemory{})
	if q.UserID != "" {
		query = query.Where("user_id = ?", q.UserID)
	}
	if q.SceneType != "" {
		query = query.Where("scene_type = ?", q.SceneType)
	}
	var total int64
	query.Count(&total)
	if q.Page <= 0 {
		q.Page = 1
	}
	if q.PageSize <= 0 {
		q.PageSize = 20
	}
	var items []EpisodicMemory
	err := query.Order("created_at DESC").Offset((q.Page - 1) * q.PageSize).Limit(q.PageSize).Find(&items).Error
	if items == nil {
		items = []EpisodicMemory{}
	}
	return items, total, err
}

func (r *repository) FindByID(id string) (*EpisodicMemory, error) {
	var m EpisodicMemory
	err := r.db.Where("id = ?", id).First(&m).Error
	return &m, err
}

func (r *repository) Create(m *EpisodicMemory) error {
	if m.ID == "" {
		m.ID = uuid.New().String()
	}
	return r.db.Create(m).Error
}

func (r *repository) Delete(id string) error {
	return r.db.Where("id = ?", id).Delete(&EpisodicMemory{}).Error
}

func (r *repository) GetByUserID(userID string, limit int) ([]EpisodicMemory, error) {
	if limit <= 0 {
		limit = 20
	}
	var items []EpisodicMemory
	err := r.db.Where("user_id = ?", userID).Order("created_at DESC").Limit(limit).Find(&items).Error
	if items == nil {
		items = []EpisodicMemory{}
	}
	return items, err
}

func (r *repository) GetRecent(userID string, limit int) ([]EpisodicMemory, error) {
	if limit <= 0 {
		limit = 5
	}
	var items []EpisodicMemory
	err := r.db.Where("user_id = ?", userID).Order("created_at DESC").Limit(limit).Find(&items).Error
	if items == nil {
		items = []EpisodicMemory{}
	}
	return items, err
}

func (r *repository) GetRecentScoped(userID string, limit int) ([]EpisodicMemory, error) {
	if userID == "" || userID == "default" {
		return []EpisodicMemory{}, nil
	}
	return r.GetRecent(userID, limit)
}

func (r *repository) FindByTime(filter EpisodicTimeFilter, userID string, limit int) ([]EpisodicMemory, int64, error) {
	if limit <= 0 {
		limit = 20
	}
	query := r.db.Model(&EpisodicMemory{})
	if userID != "" && userID != "default" {
		query = query.Where("user_id = ?", userID)
	}
	switch filter.Basis {
	case "validity":
		if filter.TimeFrom != "" {
			query = query.Where("message_time_end >= ? OR message_time_end = ''", filter.TimeFrom)
		}
		if filter.TimeTo != "" {
			query = query.Where("message_time_start <= ? OR message_time_start = ''", filter.TimeTo)
		}
	default:
		if filter.TimeFrom != "" {
			query = query.Where("created_at >= ?", filter.TimeFrom)
		}
		if filter.TimeTo != "" {
			query = query.Where("created_at <= ?", filter.TimeTo)
		}
	}
	var total int64
	query.Count(&total)
	var items []EpisodicMemory
	err := query.Order("message_time_start ASC, created_at ASC").Limit(limit).Find(&items).Error
	if items == nil {
		items = []EpisodicMemory{}
	}
	return items, total, err
}

func (r *repository) GetDetailWithMessages(id string, db *gorm.DB) (*EpisodicMemory, []map[string]interface{}, error) {
	var m EpisodicMemory
	err := r.db.Where("id = ?", id).First(&m).Error
	if err != nil {
		return nil, nil, err
	}
	var messages []map[string]interface{}
	if m.MessageTimeStart != "" && m.MessageTimeEnd != "" {
		query := db.Table("messages").Where("created_at >= ? AND created_at <= ?", m.MessageTimeStart, m.MessageTimeEnd)
		if m.SourceConvID != "" {
			query = query.Where("conversation_id = ?", m.SourceConvID)
		}
		query.Order("created_at ASC").Find(&messages)
	}
	if messages == nil {
		messages = []map[string]interface{}{}
	}
	return &m, messages, nil
}
