// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package security

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/auth"
)

// SharedCoreAdminOnly protects server-global configuration and control surfaces.
// Local single-user runtimes keep their historical behaviour; a shared Cloud
// Core requires an authenticated administrator actor.
func SharedCoreAdminOnly() gin.HandlerFunc {
	return func(c *gin.Context) {
		actor := GetActor(c)
		if actor == nil || !actor.HasPermission(auth.PermSystemAdmin) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"code": http.StatusForbidden,
				"msg":  "当前设备没有云端管理员权限，模型与系统配置只能由 Core 或获授权的管理员修改",
			})
			return
		}
		c.Next()
	}
}
