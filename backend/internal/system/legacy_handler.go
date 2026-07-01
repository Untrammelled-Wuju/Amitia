// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package system

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/pkg/util"
)

func (h *Handler) LegacyListConversations(c *gin.Context) {
	util.SuccessResponse(c, h.service.LegacyListConversations())
}

func (h *Handler) LegacyGetMessages(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "50"))
	util.SuccessResponse(c, h.service.LegacyGetMessages(c.Param("id"), page, pageSize))
}

func (h *Handler) LegacyDeleteConversation(c *gin.Context) {
	util.SuccessResponse(c, h.service.LegacyDeleteConversation(c.Param("id")))
}
