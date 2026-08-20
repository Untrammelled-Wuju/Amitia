// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package system

import (
	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/pkg/util"
)

func (h *Handler) LogsRecent(c *gin.Context) { util.SuccessResponse(c, h.service.GetLogsRecent(50)) }

func (h *Handler) LogsRecentErrors(c *gin.Context) {
	util.SuccessResponse(c, h.service.GetLogsRecentErrors(20))
}

func (h *Handler) LogsFiles(c *gin.Context) { util.SuccessResponse(c, h.service.GetLogsFiles()) }

func (h *Handler) LogsFileContent(c *gin.Context) {
	c.String(200, h.service.GetLogsFileContent(c.Param("name")))
}

func (h *Handler) LogsDelete(c *gin.Context) { util.SuccessResponse(c, h.service.DeleteLogs()) }

func (h *Handler) LogsModelErrors(c *gin.Context) {
	util.SuccessResponse(c, h.service.GetLogsModelErrors())
}

func (h *Handler) LogsDeleteModelErrors(c *gin.Context) {
	util.SuccessResponse(c, h.service.DeleteLogsModelErrors())
}

func (h *Handler) LogsPromptTraces(c *gin.Context) {
	util.SuccessResponse(c, h.service.GetLogsPromptTraces(100))
}
