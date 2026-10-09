// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package sync

import (
	"context"
	"errors"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/extension/kernel/host_registry"
	"github.com/u-ai/backend/internal/middleware/security"
	"github.com/u-ai/backend/internal/runtimeidentity"
)

type DeviceOwnershipValidator interface {
	RequireTrustedDevice(ctx context.Context, spaceID runtimeidentity.SpaceID, deviceID runtimeidentity.DeviceID) error
}

type Handler struct {
	svc        *Service
	ownDevices DeviceOwnershipValidator
}

func NewHandler(svc *Service, ownDevices DeviceOwnershipValidator) *Handler {
	return &Handler{svc: svc, ownDevices: ownDevices}
}

func (h *Handler) requireDevice(c *gin.Context, spaceID, deviceID string) bool {
	if h.ownDevices == nil {
		c.JSON(503, gin.H{"code": "device_registry_unavailable", "message": "device registry unavailable"})
		return false
	}
	err := h.ownDevices.RequireTrustedDevice(c.Request.Context(), runtimeidentity.SpaceID(spaceID), runtimeidentity.DeviceID(deviceID))
	if err == nil {
		return true
	}
	switch {
	case errors.Is(err, host_registry.ErrDeviceNotTrusted):
		c.JSON(403, gin.H{"code": "device_not_trusted", "message": "device is not trusted"})
	case errors.Is(err, host_registry.ErrDeviceNotFound), errors.Is(err, host_registry.ErrDeviceOwnedByOther):
		c.JSON(403, gin.H{"code": "forbidden", "message": "device not owned by space"})
	default:
		c.JSON(503, gin.H{"code": "device_registry_unavailable", "message": "device registry unavailable"})
	}
	return false
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
	if actor == nil || actor.SpaceID == "" {
		c.JSON(401, gin.H{"code": "unauthorized", "message": "authentication required"})
		return
	}
	req.SpaceID = string(actor.SpaceID)

	if !h.requireDevice(c, req.SpaceID, req.DeviceID) {
		return
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
	if actor == nil || actor.SpaceID == "" {
		c.JSON(401, gin.H{"code": "unauthorized", "message": "authentication required"})
		return
	}
	req.SpaceID = string(actor.SpaceID)

	if !h.requireDevice(c, req.SpaceID, req.DeviceID) {
		return
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
	if actor == nil || actor.SpaceID == "" {
		c.JSON(401, gin.H{"code": "unauthorized", "message": "authentication required"})
		return
	}

	if !h.requireDevice(c, string(actor.SpaceID), req.DeviceID) {
		return
	}

	if err := h.svc.Pull.MarkApplied(string(actor.SpaceID), req.DeviceID, ScopeDevice, req.LastApplied); err != nil {
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
	if actor == nil || actor.SpaceID == "" {
		c.JSON(401, gin.H{"code": "unauthorized", "message": "authentication required"})
		return
	}

	if !h.requireDevice(c, string(actor.SpaceID), deviceID) {
		return
	}

	status, err := h.svc.Pull.GetStatus(string(actor.SpaceID), deviceID, ScopeDevice)
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
