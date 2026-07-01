// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package system

import (
	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/pkg/util"
)

func (h *Handler) SecurityAccessConfig(c *gin.Context) {
	util.SuccessResponse(c, h.service.GetSecurityAccessConfig())
}

func (h *Handler) UpdateSecurityAccessConfig(c *gin.Context) {
	var body map[string]interface{}
	c.ShouldBindJSON(&body)
	util.SuccessResponse(c, h.service.UpdateSecurityAccessConfig(body))
}

func (h *Handler) SecurityAccessStatus(c *gin.Context) {
	util.SuccessResponse(c, h.service.GetSecurityAccessStatus())
}

func (h *Handler) SecurityAccountCheck(c *gin.Context) {
	util.SuccessResponse(c, h.service.SecurityAccountCheck())
}

func (h *Handler) SecurityExposureCheck(c *gin.Context) {
	util.SuccessResponse(c, h.service.SecurityExposureCheck())
}

func (h *Handler) SecurityStatus(c *gin.Context) {
	util.SuccessResponse(c, h.service.GetSecurityStatus())
}
