// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package system

import (
	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/pkg/util"
)

func (h *Handler) UpdateCheck(c *gin.Context) { util.SuccessResponse(c, h.service.CheckUpdate()) }

func (h *Handler) GetUpdateConfig(c *gin.Context) {
	util.SuccessResponse(c, h.service.GetUpdateConfig())
}

func (h *Handler) UpdateConfig_Update(c *gin.Context) {
	var body map[string]interface{}
	c.ShouldBindJSON(&body)
	util.SuccessResponse(c, h.service.UpdateUpdateConfig(body))
}

func (h *Handler) ReleaseCheckLatest(c *gin.Context) {
	util.SuccessResponse(c, h.service.GetReleaseCheckLatest())
}

func (h *Handler) ReleaseCheckHistory(c *gin.Context) {
	util.SuccessResponse(c, h.service.GetReleaseCheckHistory())
}

func (h *Handler) ReleaseCheckExport(c *gin.Context) {
	util.SuccessResponse(c, h.service.ExportReleaseCheck())
}

func (h *Handler) ReleaseCheckRun(c *gin.Context) {
	util.SuccessResponse(c, h.service.RunReleaseCheck())
}
