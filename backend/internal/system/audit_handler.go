// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package system

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/pkg/util"
)

func (h *Handler) AuditActions(c *gin.Context) { util.SuccessResponse(c, h.service.GetAuditActions()) }

func (h *Handler) AuditLogs(c *gin.Context) {
	limit := 100
	if raw := c.Query("limit"); raw != "" {
		if value, err := strconv.Atoi(raw); err == nil {
			limit = value
		}
	}
	util.SuccessResponse(c, h.service.GetAuditLogs(limit))
}

func (h *Handler) ClearAuditLogs(c *gin.Context) {
	deleted := h.service.ClearAuditLogs()
	util.SuccessResponse(c, map[string]interface{}{"deleted": deleted})
}

func (h *Handler) AuditSettings(c *gin.Context) {
	util.SuccessResponse(c, h.service.GetAuditSettings())
}

func (h *Handler) UpdateAuditSettings(c *gin.Context) {
	var body map[string]interface{}
	if err := c.ShouldBindJSON(&body); err != nil {
		body = map[string]interface{}{}
	}
	util.SuccessResponse(c, h.service.UpdateAuditSettings(body))
}

func (h *Handler) AuditStats(c *gin.Context) { util.SuccessResponse(c, h.service.GetAuditStats()) }
