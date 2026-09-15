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

func normalizeConversationOwner(spaceID string) string {
	return requestidentity.NormalizeSpaceID(spaceID)
}

func conversationOwnerMatches(stored, requested string) bool {
	stored = strings.TrimSpace(stored)
	requested = normalizeConversationOwner(requested)
	if stored != "" && stored == requested {
		return true
	}
	return chatLocalSingleUserMode() && requested != "" && (stored == "" || stored == requestidentity.LegacySpaceID)
}

func (s *service) requireConversationOwner(convID, spaceID string) (*Conversation, error) {
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
	if conv == nil || !conversationOwnerMatches(conv.SpaceID, spaceID) {
		return nil, gorm.ErrRecordNotFound
	}
	return conv, nil
}

func (s *service) requireMessageOwner(messageID, spaceID string) (*Message, error) {
	messageID = strings.TrimSpace(messageID)
	if messageID == "" {
		return nil, fmt.Errorf("message id is required")
	}
	var msg Message
	if err := s.db.Where("id = ? AND deleted_at IS NULL", messageID).First(&msg).Error; err != nil {
		return nil, err
	}
	if _, err := s.requireConversationOwner(msg.ConversationID, spaceID); err != nil {
		return nil, err
	}
	return &msg, nil
}

func applyConversationOwnerScope(query *gorm.DB, spaceID string) *gorm.DB {
	owner := normalizeConversationOwner(spaceID)
	if chatLocalSingleUserMode() {
		return query.Where("space_id = ? OR space_id = '' OR space_id IS NULL OR space_id = ?", owner, requestidentity.LegacySpaceID)
	}
	return query.Where("space_id = ?", owner)
}
