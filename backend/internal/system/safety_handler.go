// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package system

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/pkg/util"
)

func (h *Handler) CheckInputSafety(c *gin.Context) {
	var body struct {
		Text string `json:"text"`
	}
	c.ShouldBindJSON(&body)
	util.SuccessResponse(c, h.service.CheckSafety(body.Text))
}

func (h *Handler) CheckOutputSafety(c *gin.Context) {
	var body struct {
		Text string `json:"text"`
	}
	c.ShouldBindJSON(&body)
	util.SuccessResponse(c, h.service.CheckSafety(body.Text))
}

func (h *Handler) SafetyImportCheck(c *gin.Context) {
	var body map[string]interface{}
	c.ShouldBindJSON(&body)
	util.SuccessResponse(c, h.service.SafetyImportCheck(body))
}

func (h *Handler) SafetyEvents(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))
	util.SuccessResponse(c, h.service.SafetyEvents(page, pageSize))
}

func (h *Handler) DeleteSafetyEvents(c *gin.Context) {
	util.SuccessResponse(c, h.service.DeleteSafetyEvents())
}
