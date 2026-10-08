// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package system

import (
	"context"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/auth"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/middleware/security"
	"github.com/u-ai/backend/pkg/util"
)

func notificationScope(c *gin.Context, body map[string]interface{}) (string, string) {
	actor := security.GetActor(c)
	if actor == nil {
		actor, _ = auth.FromContext(c.Request.Context())
	}
	spaceID, deviceID := "", ""
	if actor != nil {
		spaceID = strings.TrimSpace(actor.SpaceID.String())
		deviceID = strings.TrimSpace(actor.DeviceID.String())
	}
	if requested := strings.TrimSpace(c.Query("deviceId")); requested != "" {
		deviceID = requested
	}
	if body != nil {
		if value, ok := body["deviceId"].(string); ok && strings.TrimSpace(value) != "" {
			deviceID = strings.TrimSpace(value)
		}
	}
	return spaceID, deviceID
}

type scopedNotificationService interface {
	NotificationsContext(context.Context, string, map[string]interface{}, string, string) (map[string]interface{}, error)
}

func (h *Handler) handleScopedNotification(c *gin.Context, operation string, bindBody bool) {
	var body map[string]interface{}
	if bindBody {
		if err := c.ShouldBindJSON(&body); err != nil {
			util.ErrorResponse(c, 400, err.Error(), nil)
			return
		}
	}
	actor := security.GetActor(c)
	if actor == nil {
		actor, _ = auth.FromContext(c.Request.Context())
	}
	if actor == nil {
		util.ErrorResponse(c, 403, "当前请求没有可信设备身份", nil)
		return
	}
	space, device := notificationScope(c, body)
	if device != strings.TrimSpace(actor.DeviceID.String()) {
		if !actor.HasPermission(auth.PermSystemAdmin) {
			util.ErrorResponse(c, 403, "当前设备不能访问其他设备的通知设置", nil)
			return
		}
		if actor.PrincipalType == auth.PrincipalTrustedDevice {
			raw, exists := c.Get("authenticatedConfigurationPolicyService")
			service, valid := raw.(*coordination.Service)
			rawPolicy, _ := c.Get("authenticatedConfigurationPolicy")
			policy, policyValid := rawPolicy.(coordination.Policy)
			if !exists || !valid || service == nil || !policyValid || !policy.Coordinated || !policy.Administrator {
				util.ErrorResponse(c, 403, "缺少当前 Core 管理授权", nil)
				return
			}
		}
	}
	if actor.PrincipalType == auth.PrincipalTrustedDevice {
		if _, scoped := coordination.FromContext(c.Request.Context()); !scoped {
			raw, exists := c.Get("authenticatedConfigurationPolicyService")
			authority, valid := raw.(*coordination.Service)
			rawPolicy, _ := c.Get("authenticatedConfigurationPolicy")
			policy, policyValid := rawPolicy.(coordination.Policy)
			if !exists || !valid || authority == nil || !policyValid {
				util.ErrorResponse(c, 403, "缺少当前 Core 设备授权", nil)
				return
			}
			ctx, scope, finish, err := authority.Begin(c.Request.Context(), space, actor.DeviceID.String(), "", space, "", actor.RequestID)
			if err != nil {
				util.ErrorResponse(c, 409, "当前 Core 设备授权已变化", nil)
				return
			}
			defer finish()
			if scope.ProviderEpoch != policy.ProviderEpoch || scope.PermissionRevision != policy.PermissionRevision || scope.ModeRevision != policy.ModeRevision || scope.Coordinated != policy.Coordinated {
				util.ErrorResponse(c, 409, "当前 Core 设备授权已变化", nil)
				return
			}
			c.Request = c.Request.WithContext(ctx)
		}
	}
	svc, ok := h.service.(scopedNotificationService)
	if !ok {
		util.ErrorResponse(c, 409, "当前通知服务不支持可撤销的设备操作", nil)
		return
	}
	result, err := svc.NotificationsContext(c.Request.Context(), operation, body, space, device)
	if err != nil {
		util.ErrorResponse(c, 409, err.Error(), nil)
		return
	}
	util.SuccessResponse(c, result)
}

func (h *Handler) NotificationsSettings(c *gin.Context) {
	h.handleScopedNotification(c, "settings", false)
}

func (h *Handler) UpdateNotificationsSettings(c *gin.Context) {
	h.handleScopedNotification(c, "update", true)
}

func (h *Handler) NotificationsStatus(c *gin.Context) {
	h.handleScopedNotification(c, "status", false)
}

func (h *Handler) NotificationsSubscribe(c *gin.Context) {
	h.handleScopedNotification(c, "subscribe", true)
}

func (h *Handler) NotificationsTest(c *gin.Context) {
	h.handleScopedNotification(c, "test", true)
}

func (h *Handler) NotificationsUnsubscribe(c *gin.Context) {
	h.handleScopedNotification(c, "unsubscribe", true)
}
