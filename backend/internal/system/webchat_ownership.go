// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package system

import (
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/requestidentity"
	"gorm.io/gorm"
)

func webChatSpaceID(c *gin.Context) string {
	return requestidentity.ResolveGin(c)
}

func (h *Handler) webChatCharacterQuery(spaceID string) *gorm.DB {
	q := h.db.Table("characters").Where("deleted_at IS NULL")
	owner := requestidentity.NormalizeSpaceID(spaceID)
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
