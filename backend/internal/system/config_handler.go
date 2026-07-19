// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package system

import (
	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/pkg/util"
)

func (h *Handler) AppConfig(c *gin.Context) { util.SuccessResponse(c, h.service.AppConfig()) }

func (h *Handler) UpdateConfig(c *gin.Context) {
	var body map[string]interface{}
	c.ShouldBindJSON(&body)
	util.SuccessResponse(c, h.service.UpdateAppConfig(body))
}

func (h *Handler) ConfigSettings(c *gin.Context) { util.SuccessResponse(c, h.service.ConfigSettings()) }

func (h *Handler) ConfigExport(c *gin.Context) { util.SuccessResponse(c, h.service.ConfigExport()) }

func (h *Handler) ConfigImportPreview(c *gin.Context) {
	var body map[string]interface{}
	c.ShouldBindJSON(&body)
	util.SuccessResponse(c, h.service.ConfigImportPreviewService(body))
}

func (h *Handler) ConfigImportConfirm(c *gin.Context) {
	var body map[string]interface{}
	c.ShouldBindJSON(&body)
	util.SuccessResponse(c, h.service.ConfigImportConfirmService(body))
}

func (h *Handler) MoodDetectionConfig(c *gin.Context) {
	util.SuccessResponse(c, h.service.MoodDetectionConfig())
}
