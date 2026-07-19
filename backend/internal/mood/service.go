// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package mood

import (
	"github.com/u-ai/backend/pkg/app"
	"gorm.io/gorm"
)

type Service interface {
	List() map[string]interface{}
	GetByConversation(id string) map[string]interface{}
	Delete(id string) bool
	DeleteByConversation(id string) bool
}

type moodItem struct {
	MessageID string `gorm:"column:id" json:"messageId"`
	MoodLabel string `gorm:"column:mood" json:"moodLabel"`
}

type service struct {
	db *gorm.DB
}

func NewService(ctx *app.AppContext) Service { return &service{db: ctx.DB} }

func (s *service) List() map[string]interface{} {
	var items []map[string]interface{}
	s.db.Raw("SELECT DISTINCT mood as name, COUNT(*) as count, MAX(created_at) as lastDetected FROM messages WHERE mood IS NOT NULL AND mood != '' GROUP BY mood ORDER BY count DESC").Scan(&items)
	if items == nil {
		items = []map[string]interface{}{}
	}
	return map[string]interface{}{"moods": items}
}

func (s *service) GetByConversation(id string) map[string]interface{} {
	var items []moodItem
	s.db.Table("messages").Select("id, mood").Where("conversation_id = ? AND mood IS NOT NULL AND mood != ''", id).Order("created_at DESC").Limit(50).Find(&items)
	if items == nil {
		items = []moodItem{}
	}
	return map[string]interface{}{"items": items, "conversationId": id}
}

func (s *service) Delete(id string) bool {
	return s.db.Table("messages").Where("id = ?", id).Update("mood", "").RowsAffected > 0
}

func (s *service) DeleteByConversation(id string) bool {
	return s.db.Table("messages").Where("conversation_id = ?", id).Update("mood", "").RowsAffected > 0
}
