// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package system

import (
	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/pkg/util"
)

func (h *Handler) SetupStatus(c *gin.Context) { util.SuccessResponse(c, h.service.SetupStatus()) }

func (h *Handler) SetupChecks(c *gin.Context) { util.SuccessResponse(c, h.service.SetupChecks()) }

func (h *Handler) SetupFinish(c *gin.Context) { util.SuccessResponse(c, h.service.SetupFinish()) }

func (h *Handler) SetupReset(c *gin.Context) { util.SuccessResponse(c, h.service.SetupReset()) }

func (h *Handler) SetupStep(c *gin.Context) {
	var body struct {
		Step string `json:"step"`
	}
	c.ShouldBindJSON(&body)
	util.SuccessResponse(c, h.service.SetupStep(body.Step))
}

func (h *Handler) OnboardingStatus(c *gin.Context) {
	util.SuccessResponse(c, h.service.OnboardingStatus())
}

func (h *Handler) OnboardingComplete(c *gin.Context) {
	util.SuccessResponse(c, h.service.OnboardingComplete())
}

func (h *Handler) OnboardingReset(c *gin.Context) {
	util.SuccessResponse(c, h.service.OnboardingReset())
}
