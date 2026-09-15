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
	ListForSpace(spaceID string) map[string]interface{}
	GetByConversationForSpace(id, spaceID string) map[string]interface{}
	DeleteForSpace(id, spaceID string) bool
	DeleteByConversationForSpace(id, spaceID string) bool
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

func (s *service) ownedMessages(spaceID string) *gorm.DB {
	owner := requestidentity.NormalizeSpaceID(spaceID)
	q := s.db.Table("messages AS m").Joins("JOIN conversations AS c ON c.id = m.conversation_id").
		Where("m.deleted_at IS NULL AND c.deleted_at IS NULL")
	if moodLocalSingleUserMode() {
		return q.Where("c.space_id = ? OR c.space_id = '' OR c.space_id IS NULL OR c.space_id = ?", owner, requestidentity.LegacySpaceID)
	}
	return q.Where("c.space_id = ?", owner)
}

func (s *service) ListForSpace(spaceID string) map[string]interface{} {
	var items []map[string]interface{}
	s.ownedMessages(spaceID).
		Select("m.mood as name, COUNT(*) as count, MAX(m.created_at) as lastDetected").
		Where("m.mood IS NOT NULL AND m.mood != ''").
		Group("m.mood").Order("count DESC").Scan(&items)
	if items == nil {
		items = []map[string]interface{}{}
	}
	return map[string]interface{}{"moods": items}
}

func (s *service) GetByConversationForSpace(id, spaceID string) map[string]interface{} {
	var items []moodItem
	s.ownedMessages(spaceID).
		Select("m.id, m.mood").
		Where("m.conversation_id = ? AND m.mood IS NOT NULL AND m.mood != ''", id).
		Order("m.created_at DESC").Limit(50).Scan(&items)
	if items == nil {
		items = []moodItem{}
	}
	return map[string]interface{}{"items": items, "conversationId": id}
}

func (s *service) DeleteForSpace(id, spaceID string) bool {
	var messageID string
	if err := s.ownedMessages(spaceID).Select("m.id").Where("m.id = ?", id).Limit(1).Row().Scan(&messageID); err != nil || messageID == "" {
		return false
	}
	return s.db.Table("messages").Where("id = ?", messageID).Update("mood", "").RowsAffected > 0
}

func (s *service) DeleteByConversationForSpace(id, spaceID string) bool {
	var messageIDs []string
	if err := s.ownedMessages(spaceID).Select("m.id").Where("m.conversation_id = ?", id).Pluck("m.id", &messageIDs).Error; err != nil || len(messageIDs) == 0 {
		return false
	}
	return s.db.Table("messages").Where("id IN ?", messageIDs).Update("mood", "").RowsAffected > 0
}
