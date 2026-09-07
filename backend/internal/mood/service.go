// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package mood

import (
	"strings"

	"github.com/u-ai/backend/config"
	"github.com/u-ai/backend/internal/requestidentity"
	"github.com/u-ai/backend/pkg/app"
	"gorm.io/gorm"
)

type Service interface {
	ListForUser(userID string) map[string]interface{}
	GetByConversationForUser(id, userID string) map[string]interface{}
	DeleteForUser(id, userID string) bool
	DeleteByConversationForUser(id, userID string) bool
}

type moodItem struct {
	MessageID string `gorm:"column:id" json:"messageId"`
	MoodLabel string `gorm:"column:mood" json:"moodLabel"`
}

type service struct {
	db *gorm.DB
}

func NewService(ctx *app.AppContext) Service { return &service{db: ctx.DB} }

func moodLocalSingleUserMode() bool {
	return config.AppCfg != nil && strings.EqualFold(strings.TrimSpace(config.AppCfg.Security.Mode), "local_single_user")
}

func (s *service) ownedMessages(userID string) *gorm.DB {
	owner := requestidentity.NormalizeUserID(userID)
	q := s.db.Table("messages AS m").Joins("JOIN conversations AS c ON c.id = m.conversation_id").
		Where("m.deleted_at IS NULL AND c.deleted_at IS NULL")
	if moodLocalSingleUserMode() {
		return q.Where("c.user_id = ? OR c.user_id = '' OR c.user_id IS NULL OR c.user_id = ?", owner, requestidentity.DefaultUserID)
	}
	return q.Where("c.user_id = ?", owner)
}

func (s *service) ListForUser(userID string) map[string]interface{} {
	var items []map[string]interface{}
	s.ownedMessages(userID).
		Select("m.mood as name, COUNT(*) as count, MAX(m.created_at) as lastDetected").
		Where("m.mood IS NOT NULL AND m.mood != ''").
		Group("m.mood").Order("count DESC").Scan(&items)
	if items == nil {
		items = []map[string]interface{}{}
	}
	return map[string]interface{}{"moods": items}
}

func (s *service) GetByConversationForUser(id, userID string) map[string]interface{} {
	var items []moodItem
	s.ownedMessages(userID).
		Select("m.id, m.mood").
		Where("m.conversation_id = ? AND m.mood IS NOT NULL AND m.mood != ''", id).
		Order("m.created_at DESC").Limit(50).Scan(&items)
	if items == nil {
		items = []moodItem{}
	}
	return map[string]interface{}{"items": items, "conversationId": id}
}

func (s *service) DeleteForUser(id, userID string) bool {
	var messageID string
	if err := s.ownedMessages(userID).Select("m.id").Where("m.id = ?", id).Limit(1).Row().Scan(&messageID); err != nil || messageID == "" {
		return false
	}
	return s.db.Table("messages").Where("id = ?", messageID).Update("mood", "").RowsAffected > 0
}

func (s *service) DeleteByConversationForUser(id, userID string) bool {
	var messageIDs []string
	if err := s.ownedMessages(userID).Select("m.id").Where("m.conversation_id = ?", id).Pluck("m.id", &messageIDs).Error; err != nil || len(messageIDs) == 0 {
		return false
	}
	return s.db.Table("messages").Where("id IN ?", messageIDs).Update("mood", "").RowsAffected > 0
}
