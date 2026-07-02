// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package chat

import (
	"context"

	"github.com/u-ai/backend/config"
)

func (s *service) trimContextWindow(ctx context.Context, convID string) {
	if ctx.Err() != nil {
		return
	}
	maxRounds := 20
	if config.AppCfg != nil && config.AppCfg.Chat.ContextWindowMaxRounds > 0 {
		maxRounds = config.AppCfg.Chat.ContextWindowMaxRounds
	}
	if maxRounds <= 0 {
		maxRounds = 20
	}
	var ids []string
	if err := s.db.WithContext(ctx).Table("messages").Select("id").Where("conversation_id = ? AND role IN ('user','assistant') AND include_in_context = 1", convID).Order("sequence DESC").Limit(maxRounds*2+100).Pluck("id", &ids).Error; err != nil {
		return
	}
	if len(ids) <= maxRounds*2 {
		return
	}
	cutoff := ids[maxRounds*2-1]
	if ctx.Err() != nil {
		return
	}
	s.db.WithContext(ctx).Exec("UPDATE messages SET include_in_context = 0 WHERE conversation_id = ? AND include_in_context = 1 AND sequence < (SELECT sequence FROM messages WHERE id = ?)", convID, cutoff)
}

func (s *service) loadHistory(convID string) []map[string]string {
	return s.loadHistoryExcluding(convID, "")
}

func (s *service) loadHistoryExcluding(convID, excludeID string) []map[string]string {
	var messages []Message
	query := s.db.Where("conversation_id = ? AND include_in_context = 1", convID)
	if excludeID != "" {
		query = query.Where("id <> ?", excludeID)
	}
	query.Order("sequence ASC").Find(&messages)
	if messages == nil {
		messages = []Message{}
	}
	history := make([]map[string]string, len(messages))
	for i, m := range messages {
		history[i] = map[string]string{"role": m.Role, "content": m.Content}
	}
	return history
}

func (s *service) findRequestMessages(convID, requestID string) (Message, []Message, bool) {
	if convID == "" || requestID == "" {
		return Message{}, nil, false
	}
	var user Message
	userFound := s.db.Where("conversation_id = ? AND request_id = ? AND role = ?", convID, requestID, "user").Order("sequence ASC").First(&user).Error == nil
	var assistants []Message
	s.db.Where("conversation_id = ? AND request_id = ? AND role = ?", convID, requestID, "assistant").Order("sequence ASC").Find(&assistants)
	return user, assistants, userFound
}
