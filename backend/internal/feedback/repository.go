// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package feedback

import (
	"github.com/u-ai/backend/pkg/app"
	"gorm.io/gorm"
)

type Repository interface {
	Create(fb *MessageFeedback) error
	GetByMessage(msgID string) ([]MessageFeedback, error)
	GetStats() (total int64, byType map[string]int64, recent []MessageFeedback, err error)
	GetRecent(limit int) ([]MessageFeedback, error)
	Delete(id int) error
	GetMessage(msgID string) (role, convID string, err error)
}

type repository struct {
	db *gorm.DB
}

func NewRepository(ctx *app.AppContext) Repository {
	return &repository{db: ctx.DB}
}

func (r *repository) Create(fb *MessageFeedback) error {
	return r.db.Create(fb).Error
}

func (r *repository) GetByMessage(msgID string) ([]MessageFeedback, error) {
	var items []MessageFeedback
	err := r.db.Where("message_id = ?", msgID).Order("created_at DESC").Find(&items).Error
	if items == nil {
		items = []MessageFeedback{}
	}
	return items, err
}

func (r *repository) GetStats() (int64, map[string]int64, []MessageFeedback, error) {
	var total int64
	r.db.Model(&MessageFeedback{}).Count(&total)

	rows, err := r.db.Model(&MessageFeedback{}).Select("feedback_type, COUNT(*) as cnt").Group("feedback_type").Rows()
	if err != nil || rows == nil {
		return total, map[string]int64{}, []MessageFeedback{}, nil
	}
	defer rows.Close()
	byType := map[string]int64{}
	for rows.Next() {
		var t string
		var c int64
		rows.Scan(&t, &c)
		byType[t] = c
	}

	var recent []MessageFeedback
	r.db.Order("created_at DESC").Limit(10).Find(&recent)
	if recent == nil {
		recent = []MessageFeedback{}
	}

	return total, byType, recent, nil
}

func (r *repository) GetRecent(limit int) ([]MessageFeedback, error) {
	var items []MessageFeedback
	err := r.db.Order("created_at DESC").Limit(limit).Find(&items).Error
	if items == nil {
		items = []MessageFeedback{}
	}
	return items, err
}

func (r *repository) Delete(id int) error {
	return r.db.Delete(&MessageFeedback{}, id).Error
}

func (r *repository) GetMessage(msgID string) (role, convID string, err error) {
	err = r.db.Table("messages").Select("role, conversation_id").Where("id = ?", msgID).Row().Scan(&role, &convID)
	return
}
