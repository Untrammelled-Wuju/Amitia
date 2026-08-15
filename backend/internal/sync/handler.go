// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package sync

import (
	"strconv"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) RegisterRoutes(r *gin.Engine, authMW gin.HandlerFunc) {
	sync := r.Group("/api/v1/sync")
	sync.Use(authMW)

	sync.POST("/pull", h.HandlePull)
	sync.POST("/push", h.HandlePush)
	sync.GET("/status", h.HandleStatus)
	sync.GET("/gap", h.HandleGap)
}

func (h *Handler) HandlePull(c *gin.Context) {
	var req PullRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"code": "invalid_request", "message": err.Error()})
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

	result, err := h.svc.Push.Push(req)
	if err != nil {
		c.JSON(500, gin.H{"code": "push_failed", "message": err.Error()})
		return
	}

	c.JSON(200, result)
}

func (h *Handler) HandleStatus(c *gin.Context) {
	deviceID := c.Query("deviceId")
	if deviceID == "" {
		c.JSON(400, gin.H{"code": "missing_device_id", "message": "deviceId required"})
		return
	}

	status, err := h.svc.Pull.GetStatus(deviceID)
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
