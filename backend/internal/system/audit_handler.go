// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package system

import (
	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/pkg/util"
)

func (h *Handler) AuditActions(c *gin.Context) { util.SuccessResponse(c, h.service.GetAuditActions()) }

func (h *Handler) AuditSettings(c *gin.Context) {
	util.SuccessResponse(c, h.service.GetAuditSettings())
}

func (h *Handler) UpdateAuditSettings(c *gin.Context) {
	var body map[string]interface{}
	c.ShouldBindJSON(&body)
	util.SuccessResponse(c, h.service.UpdateAuditSettings(body))
}

func (h *Handler) AuditStats(c *gin.Context) { util.SuccessResponse(c, h.service.GetAuditStats()) }
