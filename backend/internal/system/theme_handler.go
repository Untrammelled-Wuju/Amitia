// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package system

import (
	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/pkg/util"
)

func (h *Handler) GetTheme(c *gin.Context) { util.SuccessResponse(c, h.service.GetTheme()) }

func (h *Handler) UpdateTheme(c *gin.Context) {
	var body map[string]interface{}
	c.ShouldBindJSON(&body)
	util.SuccessResponse(c, h.service.UpdateTheme(body))
}

func (h *Handler) ThemePresets(c *gin.Context) { util.SuccessResponse(c, h.service.GetThemePresets()) }
