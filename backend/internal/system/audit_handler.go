// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package system

import (
	"context"
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
	h.auditData(c, "logs", limit)
}

func (h *Handler) ClearAuditLogs(c *gin.Context) {
	h.administratorAction(c, "audit-clear", "")
}

func (h *Handler) AuditSettings(c *gin.Context) {
	util.SuccessResponse(c, h.service.GetAuditSettings())
}

func (h *Handler) UpdateAuditSettings(c *gin.Context) {
	h.updateAdministratorSettings(c, "audit", true)
}

func (h *Handler) AuditStats(c *gin.Context) { h.auditData(c, "stats", 0) }

func (h *Handler) auditData(c *gin.Context, operation string, limit int) {
	svc, ok := h.service.(interface {
		AuditDataContext(context.Context, string, int) (interface{}, error)
	})
	if !ok {
		util.ErrorResponse(c, 409, "当前服务不支持可撤销的审计查询", nil)
		return
	}
	result, err := svc.AuditDataContext(c.Request.Context(), operation, limit)
	if err != nil {
		util.ErrorResponse(c, 409, err.Error(), nil)
		return
	}
	util.SuccessResponse(c, result)
}
