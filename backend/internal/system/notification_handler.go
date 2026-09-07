// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package system

import (
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/auth"
	"github.com/u-ai/backend/pkg/util"
)

func notificationScope(c *gin.Context, body map[string]interface{}) (string, string) {
	userID := ""
	deviceID := strings.TrimSpace(c.Query("deviceId"))
	if actor, ok := auth.FromContext(c.Request.Context()); ok && actor != nil {
		userID = strings.TrimSpace(actor.UserID.String())
		if deviceID == "" {
			deviceID = strings.TrimSpace(actor.DeviceID.String())
		}
	}
	if body != nil {
		if value, ok := body["deviceId"].(string); ok && strings.TrimSpace(value) != "" {
			deviceID = strings.TrimSpace(value)
		}
	}
	return userID, deviceID
}

func (h *Handler) NotificationsSettings(c *gin.Context) {
	userID, deviceID := notificationScope(c, nil)
	util.SuccessResponse(c, h.service.GetNotificationsSettings(userID, deviceID))
}

func (h *Handler) UpdateNotificationsSettings(c *gin.Context) {
	var body map[string]interface{}
	_ = c.ShouldBindJSON(&body)
	userID, deviceID := notificationScope(c, body)
	util.SuccessResponse(c, h.service.UpdateNotificationsSettings(body, userID, deviceID))
}

func (h *Handler) NotificationsStatus(c *gin.Context) {
	userID, deviceID := notificationScope(c, nil)
	util.SuccessResponse(c, h.service.GetNotificationsStatus(userID, deviceID))
}

func (h *Handler) NotificationsSubscribe(c *gin.Context) {
	var body map[string]interface{}
	_ = c.ShouldBindJSON(&body)
	userID, deviceID := notificationScope(c, body)
	util.SuccessResponse(c, h.service.NotificationsSubscribe(body, userID, deviceID))
}

func (h *Handler) NotificationsTest(c *gin.Context) {
	var body map[string]interface{}
	_ = c.ShouldBindJSON(&body)
	userID, deviceID := notificationScope(c, body)
	util.SuccessResponse(c, h.service.NotificationsTest(userID, deviceID))
}

func (h *Handler) NotificationsUnsubscribe(c *gin.Context) {
	var body map[string]interface{}
	_ = c.ShouldBindJSON(&body)
	userID, deviceID := notificationScope(c, body)
	util.SuccessResponse(c, h.service.NotificationsUnsubscribe(userID, deviceID))
}
