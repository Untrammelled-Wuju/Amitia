// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package feedback

import (
	"github.com/u-ai/backend/pkg/app"
	"gorm.io/gorm"
)

type Repository interface {
	Create(fb *MessageFeedback) error
	GetByMessageForUser(msgID, userID string) ([]MessageFeedback, error)
	GetStats() (total int64, byType map[string]int64, recent []MessageFeedback, err error)
	GetRecent(limit int) ([]MessageFeedback, error)
	DeleteForUser(id int, userID string) error
	GetMessageForUser(msgID, userID string) (role, convID string, err error)
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

func (r *repository) GetByMessageForUser(msgID, userID string) ([]MessageFeedback, error) {
	if _, _, err := r.GetMessageForUser(msgID, userID); err != nil {
		return nil, err
	}
	var items []MessageFeedback
	err := r.db.Where("message_id = ?", msgID).Order("created_at DESC").Find(&items).Error
	if items == nil {
		items = []MessageFeedback{}
	}
	return items, err
}

func (r *repository) GetStats() (int64, map[string]int64, []MessageFeedback, error) {
	var total int64
	if err := r.db.Model(&MessageFeedback{}).Count(&total).Error; err != nil {
		return 0, nil, nil, err
	}

	rows, err := r.db.Model(&MessageFeedback{}).Select("feedback_type, COUNT(*) as cnt").Group("feedback_type").Rows()
	if err != nil {
		return total, nil, nil, err
	}
	if rows == nil {
		return total, map[string]int64{}, []MessageFeedback{}, nil
	}
	defer rows.Close()
	byType := map[string]int64{}
	for rows.Next() {
		var t string
		var c int64
		if err := rows.Scan(&t, &c); err != nil {
			return total, nil, nil, err
		}
		byType[t] = c
	}

	var recent []MessageFeedback
	if err := r.db.Order("created_at DESC").Limit(10).Find(&recent).Error; err != nil {
		return total, byType, nil, err
	}
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

func (r *repository) DeleteForUser(id int, userID string) error {
	var fb MessageFeedback
	if err := r.db.Where("id = ?", id).Take(&fb).Error; err != nil {
		return err
	}
	if _, _, err := r.GetMessageForUser(fb.MessageID, userID); err != nil {
		return err
	}
	result := r.db.Where("id = ?", id).Delete(&MessageFeedback{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *repository) GetMessageForUser(msgID, userID string) (role, convID string, err error) {
	q := r.db.Table("messages AS m").
		Joins("JOIN conversations AS c ON c.id = m.conversation_id").
		Where("m.id = ? AND m.deleted_at IS NULL AND c.deleted_at IS NULL", msgID)
	q = feedbackOwnerScope(q, userID)
	err = q.Select("m.role, m.conversation_id").Row().Scan(&role, &convID)
	return
}
