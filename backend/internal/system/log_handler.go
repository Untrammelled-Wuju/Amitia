// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package system

import (
	"errors"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/pkg/util"
)

const recentLogLimit = 200

func (h *Handler) LogsRecent(c *gin.Context) {
	limit, err := parseLogLimit(c.Query("limit"), recentLogLimit)
	if err != nil {
		util.ErrorResponse(c, 400, err.Error(), nil)
		return
	}
	util.SuccessResponse(c, h.service.GetLogsRecent(limit))
}

func parseLogLimit(raw string, maximum int) (int, error) {
	if raw == "" {
		return maximum, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return 0, err
	}
	if value <= 0 {
		return 0, errors.New("limit must be greater than zero")
	}
	if value > maximum {
		return maximum, nil
	}
	return value, nil
}

func (h *Handler) LogsRecentErrors(c *gin.Context) {
	util.SuccessResponse(c, h.service.GetLogsRecentErrors(20))
}

func (h *Handler) LogsFiles(c *gin.Context) { util.SuccessResponse(c, h.service.GetLogsFiles()) }

func (h *Handler) LogsFileContent(c *gin.Context) {
	c.String(200, h.service.GetLogsFileContent(c.Param("name")))
}

func (h *Handler) LogsDelete(c *gin.Context) { h.administratorAction(c, "logs-delete", "") }

func (h *Handler) LogsModelErrors(c *gin.Context) {
	util.SuccessResponse(c, h.service.GetLogsModelErrors())
}

func (h *Handler) LogsDeleteModelErrors(c *gin.Context) {
	h.administratorAction(c, "model-errors-delete", "")
}

func (h *Handler) LogsPromptTraces(c *gin.Context) {
	util.SuccessResponse(c, h.service.GetLogsPromptTraces(100))
}
