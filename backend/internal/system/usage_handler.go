// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package system

import (
	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/pkg/util"
)

func (h *Handler) UsageOverview(c *gin.Context) {
	util.SuccessResponse(c, h.service.GetUsageOverview())
}

func (h *Handler) UsageDaily(c *gin.Context) { util.SuccessResponse(c, h.service.GetUsageDaily()) }

func (h *Handler) UsageModels(c *gin.Context) { util.SuccessResponse(c, h.service.GetUsageModels()) }

func (h *Handler) UsageSources(c *gin.Context) { util.SuccessResponse(c, h.service.GetUsageSources()) }

func (h *Handler) UsageClear(c *gin.Context) { util.SuccessResponse(c, h.service.ClearUsage()) }
