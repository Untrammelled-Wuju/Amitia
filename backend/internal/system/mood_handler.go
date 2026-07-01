// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package system

import (
	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/pkg/util"
)

func (h *Handler) GetCompanionMoods(c *gin.Context) { util.SuccessResponse(c, h.service.GetMoods()) }

func (h *Handler) GetCompanionMoodsByConversation(c *gin.Context) {
	util.SuccessResponse(c, h.service.GetMoodsByConversation(c.Param("id")))
}

func (h *Handler) DeleteCompanionMood(c *gin.Context) {
	util.SuccessResponse(c, h.service.DeleteMood(c.Param("id")))
}

func (h *Handler) DeleteCompanionMoodsByConversation(c *gin.Context) {
	util.SuccessResponse(c, h.service.DeleteMoodsByConversation(c.Param("id")))
}
