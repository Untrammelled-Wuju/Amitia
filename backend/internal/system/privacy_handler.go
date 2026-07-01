// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package system

import (
	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/pkg/util"
)

func (h *Handler) PrivacyScan(c *gin.Context) { util.SuccessResponse(c, h.service.PrivacyScan()) }

func (h *Handler) PrivacyMask(c *gin.Context) { util.SuccessResponse(c, h.service.PrivacyMask()) }

func (h *Handler) PrivacyScanResults(c *gin.Context) {
	util.SuccessResponse(c, h.service.PrivacyScanResults())
}

func (h *Handler) PrivacyScanResultsGet(c *gin.Context) {
	util.SuccessResponse(c, h.service.GetPrivacyScanResult(c.Param("id")))
}
