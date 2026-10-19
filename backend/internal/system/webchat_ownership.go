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

func (h *Handler) requireWebChatImportConversation(convID, spaceID string) (*chat.Conversation, error) {
	var conv chat.Conversation
	if err := h.webChatOwnedConversationQuery(spaceID).Where("id = ? AND source = ?", strings.TrimSpace(convID), "import").First(&conv).Error; err != nil {
		return nil, err
	}
	return &conv, nil
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
