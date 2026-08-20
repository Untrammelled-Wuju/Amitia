// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package sync

import (
	"context"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/middleware/security"
)

type DeviceOwnershipValidator interface {
	RequireOwned(ctx context.Context, userID string, deviceID string) error
}

type Handler struct {
	svc        *Service
	ownDevices DeviceOwnershipValidator
}

func NewHandler(svc *Service, ownDevices DeviceOwnershipValidator) *Handler {
	return &Handler{svc: svc, ownDevices: ownDevices}
}

func (h *Handler) RegisterRoutes(r *gin.RouterGroup, authMW gin.HandlerFunc) {
	sync := r.Group("/v1/sync")
	sync.Use(authMW)

	sync.POST("/pull", h.HandlePull)
	sync.POST("/push", h.HandlePush)
	sync.POST("/ack", h.HandleAck)
	sync.GET("/status", h.HandleStatus)
	sync.GET("/gap", h.HandleGap)
}

func (h *Handler) HandlePull(c *gin.Context) {
	var req PullRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"code": "invalid_request", "message": err.Error()})
		return
	}

	actor := security.GetActor(c)
	if actor == nil || actor.UserID == "" {
		c.JSON(401, gin.H{"code": "unauthorized", "message": "authentication required"})
		return
	}
	req.UserID = string(actor.UserID)

	if req.DeviceID != "" && h.ownDevices != nil {
		if err := h.ownDevices.RequireOwned(c.Request.Context(), req.UserID, req.DeviceID); err != nil {
			c.JSON(403, gin.H{"code": "forbidden", "message": "device not owned by user"})
			return
		}
	}

	result, err := h.svc.Pull.Pull(req)
	if err != nil {
		c.JSON(500, gin.H{"code": "pull_failed", "message": err.Error()})
		return
	}

	c.JSON(200, result)
}

func (h *Handler) HandlePush(c *gin.Context) {
	var req PushRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"code": "invalid_request", "message": err.Error()})
		return
	}

	actor := security.GetActor(c)
	if actor == nil || actor.UserID == "" {
		c.JSON(401, gin.H{"code": "unauthorized", "message": "authentication required"})
		return
	}
	req.UserID = string(actor.UserID)

	if req.DeviceID != "" && h.ownDevices != nil {
		if err := h.ownDevices.RequireOwned(c.Request.Context(), req.UserID, req.DeviceID); err != nil {
			c.JSON(403, gin.H{"code": "forbidden", "message": "device not owned by user"})
			return
		}
	}

	result, err := h.svc.Push.Push(req)
	if err != nil {
		c.JSON(500, gin.H{"code": "push_failed", "message": err.Error()})
		return
	}

	c.JSON(200, result)
}

func (h *Handler) HandleAck(c *gin.Context) {
	var req struct {
		DeviceID    string   `json:"deviceId" binding:"required"`
		LastApplied Sequence `json:"lastApplied" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"code": "invalid_request", "message": err.Error()})
		return
	}

	actor := security.GetActor(c)
	if actor == nil || actor.UserID == "" {
		c.JSON(401, gin.H{"code": "unauthorized", "message": "authentication required"})
		return
	}

	if req.DeviceID != "" && h.ownDevices != nil {
		if err := h.ownDevices.RequireOwned(c.Request.Context(), string(actor.UserID), req.DeviceID); err != nil {
			c.JSON(403, gin.H{"code": "forbidden", "message": "device not owned by user"})
			return
		}
	}

	if err := h.svc.Pull.MarkApplied(string(actor.UserID), req.DeviceID, ScopeDevice, req.LastApplied); err != nil {
		c.JSON(500, gin.H{"code": "ack_failed", "message": err.Error()})
		return
	}

	c.JSON(200, gin.H{"code": "ok"})
}

func (h *Handler) HandleStatus(c *gin.Context) {
	deviceID := c.Query("deviceId")
	if deviceID == "" {
		c.JSON(400, gin.H{"code": "missing_device_id", "message": "deviceId required"})
		return
	}

	actor := security.GetActor(c)
	if actor == nil || actor.UserID == "" {
		c.JSON(401, gin.H{"code": "unauthorized", "message": "authentication required"})
		return
	}

	if h.ownDevices != nil {
		if err := h.ownDevices.RequireOwned(c.Request.Context(), string(actor.UserID), deviceID); err != nil {
			c.JSON(403, gin.H{"code": "forbidden", "message": "device not owned by user"})
			return
		}
	}

	status, err := h.svc.Pull.GetStatus(string(actor.UserID), deviceID, ScopeDevice)
	if err != nil {
		c.JSON(500, gin.H{"code": "status_failed", "message": err.Error()})
		return
	}

	c.JSON(200, status)
}

func (h *Handler) HandleGap(c *gin.Context) {
	var cursor Sequence
	if s := c.Query("cursor"); s != "" {
		if n, err := strconv.ParseInt(s, 10, 64); err == nil {
			cursor = Sequence(n)
		}
	}

	report, err := h.svc.Gap.Check(cursor, 0)
	if err != nil {
		c.JSON(500, gin.H{"code": "gap_check_failed", "message": err.Error()})
		return
	}

	c.JSON(200, report)
}
