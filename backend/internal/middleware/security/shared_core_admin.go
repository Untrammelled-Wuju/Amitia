// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package security

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/config"
)

// SharedCoreAdminOnly protects server-global configuration and control surfaces.
// Local single-user runtimes keep their historical behaviour; a shared Cloud
// Core requires an authenticated administrator actor.
func SharedCoreAdminOnly() gin.HandlerFunc {
	return func(c *gin.Context) {
		if config.AppCfg != nil && strings.EqualFold(strings.TrimSpace(config.AppCfg.Security.Mode), "local_single_user") {
			c.Next()
			return
		}
		actor := GetActor(c)
		if actor == nil || !actor.HasRole("admin") {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"code": http.StatusForbidden,
				"msg":  "shared Cloud Core administration requires admin role",
			})
			return
		}
		c.Next()
	}
}
