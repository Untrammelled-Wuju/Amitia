// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package system

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/auth"
	"github.com/u-ai/backend/internal/safety"
	"github.com/u-ai/backend/pkg/util"
)

func (h *Handler) SafetyEvents(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))
	scope, err := safety.ScopeFromContextWithCoordinator(c.Request.Context(), h.coordination)
	if err != nil {
		c.JSON(409, gin.H{"message": err.Error()})
		return
	}
	items, total, err := h.service.SafetyEvents(c.Request.Context(), scope, page, pageSize, c.Query("status"))
	if err != nil {
		c.JSON(409, gin.H{"message": err.Error()})
		return
	}
	util.SuccessResponse(c, gin.H{"items": items, "total": total})
}

func (h *Handler) DeleteSafetyEvents(c *gin.Context) {
	scope, err := safety.ScopeFromContextWithCoordinator(c.Request.Context(), h.coordination)
	if err != nil {
		c.JSON(409, gin.H{"message": err.Error()})
		return
	}
	deleted, err := h.service.DeleteSafetyEvents(c.Request.Context(), scope)
	if err != nil {
		c.JSON(409, gin.H{"message": err.Error()})
		return
	}
	util.SuccessResponse(c, gin.H{"deleted": deleted})
}

func (h *Handler) HandleSafetyEvent(c *gin.Context) {
	var body struct {
		ExpectedRevision int64 `json:"expectedRevision"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || body.ExpectedRevision < 1 {
		c.JSON(400, gin.H{"message": "安全事件版本无效"})
		return
	}
	actor, ok := auth.FromContext(c.Request.Context())
	if !ok || actor == nil {
		c.AbortWithStatus(401)
		return
	}
	scope, err := safety.ScopeFromContextWithCoordinator(c.Request.Context(), h.coordination)
	if err != nil {
		c.JSON(409, gin.H{"message": err.Error()})
		return
	}
	incident, err := h.service.HandleSafetyEvent(c.Request.Context(), scope, c.Param("id"), body.ExpectedRevision)
	if err != nil {
		c.JSON(409, gin.H{"message": err.Error()})
		return
	}
	if incident.ResolvedBy == "" {
		incident.ResolvedBy = actor.DeviceID.String()
	}
	util.SuccessResponse(c, incident)
}
