// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package chat

import (
	"errors"
	"fmt"
	"strings"

	"github.com/u-ai/backend/config"
	"github.com/u-ai/backend/internal/requestidentity"
	"gorm.io/gorm"
)

func chatLocalSingleUserMode() bool {
	return config.AppCfg != nil && strings.EqualFold(strings.TrimSpace(config.AppCfg.Security.Mode), "local_single_user")
}

func normalizeConversationOwner(userID string) string {
	return requestidentity.NormalizeUserID(userID)
}

func conversationOwnerMatches(stored, requested string) bool {
	stored = strings.TrimSpace(stored)
	requested = normalizeConversationOwner(requested)
	if stored != "" && stored == requested {
		return true
	}
	return chatLocalSingleUserMode() && requested != "" && (stored == "" || stored == requestidentity.DefaultUserID)
}

func (s *service) requireConversationOwner(convID, userID string) (*Conversation, error) {
	convID = strings.TrimSpace(convID)
	if convID == "" {
		return nil, fmt.Errorf("conversation id is required")
	}
	conv, err := s.repo.GetConversation(convID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, gorm.ErrRecordNotFound
		}
		return nil, err
	}
	if conv == nil || !conversationOwnerMatches(conv.UserID, userID) {
		return nil, gorm.ErrRecordNotFound
	}
	return conv, nil
}

func (s *service) requireMessageOwner(messageID, userID string) (*Message, error) {
	messageID = strings.TrimSpace(messageID)
	if messageID == "" {
		return nil, fmt.Errorf("message id is required")
	}
	var msg Message
	if err := s.db.Where("id = ? AND deleted_at IS NULL", messageID).First(&msg).Error; err != nil {
		return nil, err
	}
	if _, err := s.requireConversationOwner(msg.ConversationID, userID); err != nil {
		return nil, err
	}
	return &msg, nil
}

func applyConversationOwnerScope(query *gorm.DB, userID string) *gorm.DB {
	owner := normalizeConversationOwner(userID)
	if chatLocalSingleUserMode() {
		return query.Where("user_id = ? OR user_id = '' OR user_id IS NULL OR user_id = ?", owner, requestidentity.DefaultUserID)
	}
	return query.Where("user_id = ?", owner)
}
