package main

import (
	"fmt"
	"strings"

	"github.com/u-ai/backend/config"
	"github.com/u-ai/backend/internal/character"
	"github.com/u-ai/backend/internal/chat"
	"github.com/u-ai/backend/internal/extension/kernel/capability"
	"github.com/u-ai/backend/internal/requestidentity"
	"gorm.io/gorm"
)

type agentAdminScopedChatService interface {
	GetStatsForSpace(spaceID string) (*chat.ChatStatsResponse, error)
	CreateConversationForSpace(req *chat.CreateConversationRequest, spaceID string) (*chat.Conversation, error)
	ListConversationsForSpace(q chat.ConversationQuery, spaceID string) (*chat.ConversationListResponse, error)
	GetConversationForSpace(id, spaceID string) (*chat.Conversation, error)
	DeleteConversationForSpace(id, spaceID string) (bool, error)
	GetMessagesForSpace(convID, spaceID string, page, pageSize int) ([]chat.Message, int64, error)
}

func agentAdminSpaceID(inv capability.ToolInvocationContext) string {
	if spaceID := strings.TrimSpace(inv.SpaceID); spaceID != "" {
		return requestidentity.NormalizeSpaceID(spaceID)
	}
	if inv.ExecContext != nil {
		if spaceID := strings.TrimSpace(string(inv.ExecContext.SpaceID)); spaceID != "" {
			return requestidentity.NormalizeSpaceID(spaceID)
		}
	}
	return requestidentity.CanonicalSpaceID()
}

func agentAdminLocalSingleUserMode() bool {
	return config.AppCfg != nil && strings.EqualFold(strings.TrimSpace(config.AppCfg.Security.Mode), "local_single_user")
}

func agentAdminOwnerQuery(db *gorm.DB, spaceID string) *gorm.DB {
	owner := requestidentity.NormalizeSpaceID(spaceID)
	if agentAdminLocalSingleUserMode() {
		return db.Where("(space_id = ? OR space_id = '' OR space_id IS NULL OR space_id = ?)", owner, requestidentity.LegacySpaceID)
	}
	return db.Where("space_id = ?", owner)
}

func agentAdminOwnerMatches(stored, requested string) bool {
	stored = strings.TrimSpace(stored)
	requested = requestidentity.NormalizeSpaceID(requested)
	if stored == requested {
		return true
	}
	return agentAdminLocalSingleUserMode() && (stored == "" || stored == requestidentity.LegacySpaceID)
}

func (c *serverAgentAdminController) scopedChat() (agentAdminScopedChatService, error) {
	svc, ok := c.chat.(agentAdminScopedChatService)
	if !ok {
		return nil, fmt.Errorf("chat service does not provide user-scoped operations")
	}
	return svc, nil
}

func (c *serverAgentAdminController) ownedCharacterList(spaceID string, includeDisabled bool) ([]character.Character, error) {
	var chars []character.Character
	q := agentAdminOwnerQuery(c.db.Model(&character.Character{}).Where("deleted_at IS NULL"), spaceID).Order("sort_order, created_at")
	if !includeDisabled {
		q = q.Where("status = ?", "enabled")
	}
	if err := q.Find(&chars).Error; err != nil {
		return nil, err
	}
	if chars == nil {
		chars = []character.Character{}
	}
	return chars, nil
}
