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
	spaceID := ""
	deviceID := strings.TrimSpace(c.Query("deviceId"))
	if actor, ok := auth.FromContext(c.Request.Context()); ok && actor != nil {
		spaceID = strings.TrimSpace(actor.SpaceID.String())
		if deviceID == "" {
			deviceID = strings.TrimSpace(actor.DeviceID.String())
		}
	}
	if body != nil {
		if value, ok := body["deviceId"].(string); ok && strings.TrimSpace(value) != "" {
			deviceID = strings.TrimSpace(value)
		}
	}
	return spaceID, deviceID
}

func (h *Handler) NotificationsSettings(c *gin.Context) {
	spaceID, deviceID := notificationScope(c, nil)
	util.SuccessResponse(c, h.service.GetNotificationsSettings(spaceID, deviceID))
}

func (h *Handler) UpdateNotificationsSettings(c *gin.Context) {
	var body map[string]interface{}
	_ = c.ShouldBindJSON(&body)
	spaceID, deviceID := notificationScope(c, body)
	util.SuccessResponse(c, h.service.UpdateNotificationsSettings(body, spaceID, deviceID))
}

func (h *Handler) NotificationsStatus(c *gin.Context) {
	spaceID, deviceID := notificationScope(c, nil)
	util.SuccessResponse(c, h.service.GetNotificationsStatus(spaceID, deviceID))
}

func (h *Handler) NotificationsSubscribe(c *gin.Context) {
	var body map[string]interface{}
	_ = c.ShouldBindJSON(&body)
	spaceID, deviceID := notificationScope(c, body)
	util.SuccessResponse(c, h.service.NotificationsSubscribe(body, spaceID, deviceID))
}

func (h *Handler) NotificationsTest(c *gin.Context) {
	var body map[string]interface{}
	_ = c.ShouldBindJSON(&body)
	spaceID, deviceID := notificationScope(c, body)
	util.SuccessResponse(c, h.service.NotificationsTest(spaceID, deviceID))
}

func (h *Handler) NotificationsUnsubscribe(c *gin.Context) {
	var body map[string]interface{}
	_ = c.ShouldBindJSON(&body)
	spaceID, deviceID := notificationScope(c, body)
	util.SuccessResponse(c, h.service.NotificationsUnsubscribe(spaceID, deviceID))
}
