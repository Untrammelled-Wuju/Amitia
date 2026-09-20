// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package system

import (
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/config"
	"github.com/u-ai/backend/internal/chat"
	"github.com/u-ai/backend/internal/requestidentity"
	"gorm.io/gorm"
)

type webChatScopedService interface {
	ListConversationsForSpace(q chat.ConversationQuery, spaceID string) (*chat.ConversationListResponse, error)
	GetConversationForSpace(id, spaceID string) (*chat.Conversation, error)
	GetMessagesForSpace(convID, spaceID string, page, pageSize int) ([]chat.Message, int64, error)
	CreateConversationForSpace(req *chat.CreateConversationRequest, spaceID string) (*chat.Conversation, error)
	EnsureChannelConversationForSpace(channel, spaceID string) (*chat.Conversation, error)
	DeleteConversationForSpace(id, spaceID string) (bool, error)
	DeleteMessagesForSpace(convID, spaceID string) error
}

type webChatMessageEditService interface {
	UpdateMessageForSpace(id, spaceID, content string) (*chat.Message, error)
}

func webChatSpaceID(c *gin.Context) string {
	return requestidentity.ResolveGin(c)
}

func webChatLocalSingleUserMode() bool {
	return config.AppCfg != nil && strings.EqualFold(strings.TrimSpace(config.AppCfg.Security.Mode), "local_single_user")
}

func webChatOwnerQuery(db *gorm.DB, spaceID string) *gorm.DB {
	owner := requestidentity.NormalizeSpaceID(spaceID)
	if webChatLocalSingleUserMode() {
		return db.Where("(space_id = ? OR space_id = '' OR space_id IS NULL OR space_id = ?)", owner, requestidentity.LegacySpaceID)
	}
	return db.Where("space_id = ?", owner)
}

func (h *Handler) webChatOwnedConversationQuery(spaceID string) *gorm.DB {
	return webChatOwnerQuery(h.db.Model(&chat.Conversation{}).Where("deleted_at IS NULL"), spaceID)
}

func (h *Handler) requireWebChatConversation(convID, spaceID string) (*chat.Conversation, error) {
	var conv chat.Conversation
	if err := h.webChatOwnedConversationQuery(spaceID).Where("id = ?", strings.TrimSpace(convID)).First(&conv).Error; err != nil {
		return nil, err
	}
	return &conv, nil
}

func (h *Handler) requireWebChatImportConversation(convID, spaceID string) (*chat.Conversation, error) {
	var conv chat.Conversation
	if err := h.webChatOwnedConversationQuery(spaceID).Where("id = ? AND source = ?", strings.TrimSpace(convID), "import").First(&conv).Error; err != nil {
		return nil, err
	}
	return &conv, nil
}

// requireWebChatConversationOrAbsent allows a caller to reserve a brand-new
// conversation id, but rejects an id that already belongs to another user.
// This must run before touching process-global chat buffers or workspace state.
func (h *Handler) requireWebChatConversationOrAbsent(convID, spaceID string) error {
	convID = strings.TrimSpace(convID)
	if convID == "" {
		return nil
	}
	var owned int64
	if err := h.webChatOwnedConversationQuery(spaceID).Where("id = ?", convID).Count(&owned).Error; err != nil {
		return err
	}
	if owned > 0 {
		return nil
	}
	var any int64
	if err := h.db.Unscoped().Model(&chat.Conversation{}).Where("id = ?", convID).Count(&any).Error; err != nil {
		return err
	}
	if any > 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (h *Handler) webChatOwnedMessageQuery(spaceID string) *gorm.DB {
	q := h.db.Table("messages").Joins("JOIN conversations ON conversations.id = messages.conversation_id").
		Where("messages.deleted_at IS NULL AND conversations.deleted_at IS NULL")
	owner := requestidentity.NormalizeSpaceID(spaceID)
	if webChatLocalSingleUserMode() {
		return q.Where("(conversations.space_id = ? OR conversations.space_id = '' OR conversations.space_id IS NULL OR conversations.space_id = ?)", owner, requestidentity.LegacySpaceID)
	}
	return q.Where("conversations.space_id = ?", owner)
}

func (h *Handler) webChatCharacterQuery(spaceID string) *gorm.DB {
	q := h.db.Table("characters").Where("deleted_at IS NULL")
	owner := requestidentity.NormalizeSpaceID(spaceID)
	if webChatLocalSingleUserMode() {
		return q.Where("(space_id = ? OR space_id = '' OR space_id IS NULL OR space_id = ?)", owner, requestidentity.LegacySpaceID)
	}
	return q.Where("space_id = ?", owner)
}

func (h *Handler) requireWebChatCharacter(characterID, spaceID string) error {
	if strings.TrimSpace(characterID) == "" {
		return nil
	}
	var count int64
	if err := h.webChatCharacterQuery(spaceID).Where("id = ?", strings.TrimSpace(characterID)).Count(&count).Error; err != nil {
		return err
	}
	if count == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}
