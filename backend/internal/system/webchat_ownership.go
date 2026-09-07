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
	ListConversationsForUser(q chat.ConversationQuery, userID string) (*chat.ConversationListResponse, error)
	GetConversationForUser(id, userID string) (*chat.Conversation, error)
	GetMessagesForUser(convID, userID string, page, pageSize int) ([]chat.Message, int64, error)
	CreateConversationForUser(req *chat.CreateConversationRequest, userID string) (*chat.Conversation, error)
	EnsureChannelConversationForUser(channel, userID string) (*chat.Conversation, error)
	DeleteConversationForUser(id, userID string) (bool, error)
	ChangeCharacterForUser(id, characterID, userID string) (*chat.Conversation, error)
	DeleteMessagesForUser(convID, userID string) error
}

func webChatUserID(c *gin.Context) string {
	return requestidentity.ResolveGin(c, "")
}

func webChatLocalSingleUserMode() bool {
	return config.AppCfg != nil && strings.EqualFold(strings.TrimSpace(config.AppCfg.Security.Mode), "local_single_user")
}

func webChatOwnerQuery(db *gorm.DB, userID string) *gorm.DB {
	owner := requestidentity.NormalizeUserID(userID)
	if webChatLocalSingleUserMode() {
		return db.Where("(user_id = ? OR user_id = '' OR user_id IS NULL OR user_id = ?)", owner, requestidentity.DefaultUserID)
	}
	return db.Where("user_id = ?", owner)
}

func (h *Handler) webChatOwnedConversationQuery(userID string) *gorm.DB {
	return webChatOwnerQuery(h.db.Model(&chat.Conversation{}).Where("deleted_at IS NULL"), userID)
}

func (h *Handler) requireWebChatConversation(convID, userID string) (*chat.Conversation, error) {
	var conv chat.Conversation
	if err := h.webChatOwnedConversationQuery(userID).Where("id = ?", strings.TrimSpace(convID)).First(&conv).Error; err != nil {
		return nil, err
	}
	return &conv, nil
}

func (h *Handler) requireWebChatImportConversation(convID, userID string) (*chat.Conversation, error) {
	var conv chat.Conversation
	if err := h.webChatOwnedConversationQuery(userID).Where("id = ? AND source = ?", strings.TrimSpace(convID), "import").First(&conv).Error; err != nil {
		return nil, err
	}
	return &conv, nil
}

// requireWebChatConversationOrAbsent allows a caller to reserve a brand-new
// conversation id, but rejects an id that already belongs to another user.
// This must run before touching process-global chat buffers or workspace state.
func (h *Handler) requireWebChatConversationOrAbsent(convID, userID string) error {
	convID = strings.TrimSpace(convID)
	if convID == "" {
		return nil
	}
	var owned int64
	if err := h.webChatOwnedConversationQuery(userID).Where("id = ?", convID).Count(&owned).Error; err != nil {
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

func (h *Handler) webChatOwnedMessageQuery(userID string) *gorm.DB {
	q := h.db.Table("messages").Joins("JOIN conversations ON conversations.id = messages.conversation_id").
		Where("messages.deleted_at IS NULL AND conversations.deleted_at IS NULL")
	owner := requestidentity.NormalizeUserID(userID)
	if webChatLocalSingleUserMode() {
		return q.Where("(conversations.user_id = ? OR conversations.user_id = '' OR conversations.user_id IS NULL OR conversations.user_id = ?)", owner, requestidentity.DefaultUserID)
	}
	return q.Where("conversations.user_id = ?", owner)
}

func (h *Handler) webChatCharacterQuery(userID string) *gorm.DB {
	q := h.db.Table("characters").Where("deleted_at IS NULL")
	owner := requestidentity.NormalizeUserID(userID)
	if webChatLocalSingleUserMode() {
		return q.Where("(user_id = ? OR user_id = '' OR user_id IS NULL OR user_id = ?)", owner, requestidentity.DefaultUserID)
	}
	return q.Where("user_id = ?", owner)
}

func (h *Handler) requireWebChatCharacter(characterID, userID string) error {
	if strings.TrimSpace(characterID) == "" {
		return nil
	}
	var count int64
	if err := h.webChatCharacterQuery(userID).Where("id = ?", strings.TrimSpace(characterID)).Count(&count).Error; err != nil {
		return err
	}
	if count == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}
