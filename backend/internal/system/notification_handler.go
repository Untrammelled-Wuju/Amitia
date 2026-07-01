// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package system

import (
	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/pkg/util"
)

func (h *Handler) NotificationsSettings(c *gin.Context) {
	util.SuccessResponse(c, h.service.GetNotificationsSettings())
}

func (h *Handler) UpdateNotificationsSettings(c *gin.Context) {
	var body map[string]interface{}
	c.ShouldBindJSON(&body)
	util.SuccessResponse(c, h.service.UpdateNotificationsSettings(body))
}

func (h *Handler) NotificationsStatus(c *gin.Context) {
	util.SuccessResponse(c, h.service.GetNotificationsStatus())
}

func (h *Handler) NotificationsSubscribe(c *gin.Context) {
	var body map[string]interface{}
	c.ShouldBindJSON(&body)
	util.SuccessResponse(c, h.service.NotificationsSubscribe(body))
}

func (h *Handler) NotificationsTest(c *gin.Context) {
	util.SuccessResponse(c, h.service.NotificationsTest())
}

func (h *Handler) NotificationsUnsubscribe(c *gin.Context) {
	util.SuccessResponse(c, h.service.NotificationsUnsubscribe())
}
