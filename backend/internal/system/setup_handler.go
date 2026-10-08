// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package system

import (
	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/pkg/util"
)

func (h *Handler) SetupStatus(c *gin.Context) { util.SuccessResponse(c, h.service.SetupStatus()) }

func (h *Handler) SetupChecks(c *gin.Context) { util.SuccessResponse(c, h.service.SetupChecks()) }

func (h *Handler) SetupFinish(c *gin.Context) {
	h.updateAdministratorSettings(c, "setup-finish", false)
}

func (h *Handler) SetupReset(c *gin.Context) { h.updateAdministratorSettings(c, "setup-reset", false) }

func (h *Handler) SetupStep(c *gin.Context) {
	h.updateAdministratorSettings(c, "setup-step", true)
}

func (h *Handler) OnboardingStatus(c *gin.Context) {
	util.SuccessResponse(c, h.service.OnboardingStatus())
}

func (h *Handler) OnboardingComplete(c *gin.Context) {
	h.updateAdministratorSettings(c, "onboarding-complete", false)
}

func (h *Handler) OnboardingReset(c *gin.Context) {
	h.updateAdministratorSettings(c, "onboarding-reset", false)
}
