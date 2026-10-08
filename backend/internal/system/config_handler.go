// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package system

import (
	"context"
	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/pkg/util"
	"net/http"
)

func (h *Handler) AppConfig(c *gin.Context) { util.SuccessResponse(c, h.service.AppConfig()) }

func (h *Handler) UpdateConfig(c *gin.Context) {
	var body map[string]interface{}
	if err := c.ShouldBindJSON(&body); err != nil {
		util.ErrorResponse(c, 400, err.Error(), nil)
		return
	}
	h.updateScopedAppConfig(c, "config", body)
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
	if err := c.ShouldBindJSON(&body); err != nil {
		util.ErrorResponse(c, 400, err.Error(), nil)
		return
	}
	h.updateScopedAppConfig(c, "import", body)
}

func (h *Handler) MoodDetectionConfig(c *gin.Context) {
	if c.Request.Method == http.MethodPut {
		var body map[string]interface{}
		if err := c.ShouldBindJSON(&body); err != nil {
			util.ErrorResponse(c, http.StatusBadRequest, "请求参数错误", err.Error())
			return
		}
		h.updateScopedAppConfig(c, "mood", body)
		return
	}
	util.SuccessResponse(c, h.service.MoodDetectionConfig())
}

type scopedAppConfigurationService interface {
	UpdateAppConfigContext(context.Context, map[string]interface{}) (map[string]interface{}, error)
	ConfigImportConfirmContext(context.Context, map[string]interface{}) (map[string]interface{}, error)
	UpdateMoodDetectionConfigContext(context.Context, map[string]interface{}) (map[string]interface{}, error)
	UpdateThemeContext(context.Context, map[string]interface{}) (map[string]interface{}, error)
}

func (h *Handler) updateScopedAppConfig(c *gin.Context, operation string, body map[string]interface{}) {
	svc, ok := h.service.(scopedAppConfigurationService)
	if !ok {
		util.ErrorResponse(c, 409, "当前配置服务不支持可撤销的管理操作", nil)
		return
	}
	var result map[string]interface{}
	var err error
	switch operation {
	case "config":
		result, err = svc.UpdateAppConfigContext(c.Request.Context(), body)
	case "import":
		result, err = svc.ConfigImportConfirmContext(c.Request.Context(), body)
	case "mood":
		result, err = svc.UpdateMoodDetectionConfigContext(c.Request.Context(), body)
	case "theme":
		result, err = svc.UpdateThemeContext(c.Request.Context(), body)
	default:
		util.ErrorResponse(c, 400, "未知配置操作", nil)
		return
	}
	if err != nil {
		util.ErrorResponse(c, 409, err.Error(), nil)
		return
	}
	util.SuccessResponse(c, result)
}
