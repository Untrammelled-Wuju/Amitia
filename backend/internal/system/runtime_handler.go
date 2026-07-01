// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package system

import (
	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/pkg/util"
)

func (h *Handler) RuntimeHealth(c *gin.Context) {
	util.SuccessResponse(c, h.service.GetRuntimeHealth())
}

func (h *Handler) HealthHistory(c *gin.Context) {
	util.SuccessResponse(c, h.service.GetRuntimeHealthHistory())
}

func (h *Handler) RuntimeStatus(c *gin.Context) {
	util.SuccessResponse(c, h.service.GetRuntimeStatus())
}

func (h *Handler) CheckDBIntegrity(c *gin.Context) {
	util.SuccessResponse(c, h.service.CheckDBIntegrity())
}

func (h *Handler) CheckNow(c *gin.Context) { util.SuccessResponse(c, h.service.RunNow()) }

func (h *Handler) CleanupTemp(c *gin.Context) { util.SuccessResponse(c, h.service.CleanupTemp()) }

func (h *Handler) ValidateMode(c *gin.Context) { util.SuccessResponse(c, h.service.ValidateMode()) }

func (h *Handler) RotateLogs(c *gin.Context) { util.SuccessResponse(c, h.service.RotateLogs()) }

func (h *Handler) LongRunningConfig(c *gin.Context) {
	util.SuccessResponse(c, h.service.GetLongRunningConfig())
}

func (h *Handler) UpdateLongRunningConfig(c *gin.Context) {
	var body map[string]interface{}
	c.ShouldBindJSON(&body)
	util.SuccessResponse(c, h.service.UpdateLongRunningConfig(body))
}

func (h *Handler) LongRunningStatus(c *gin.Context) {
	util.SuccessResponse(c, h.service.GetLongRunningStatus())
}

func (h *Handler) GetRuntimeMode(c *gin.Context) { util.SuccessResponse(c, h.service.GetRuntimeMode()) }

func (h *Handler) UpdateRuntimeMode(c *gin.Context) {
	var body map[string]interface{}
	c.ShouldBindJSON(&body)
	util.SuccessResponse(c, h.service.UpdateRuntimeMode(body))
}
