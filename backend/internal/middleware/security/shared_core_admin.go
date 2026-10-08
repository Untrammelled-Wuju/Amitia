// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package security

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/auth"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

const ExpectedCoreHeader = "X-Amitia-Expected-Core-ID"
const ExpectedConfigurationPolicyHeader = "X-Amitia-Expected-Configuration-Policy"
const configurationPolicyContextKey = "authenticatedConfigurationPolicy"
const configurationPolicyServiceContextKey = "authenticatedConfigurationPolicyService"

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
		expectedCore := c.GetHeader(ExpectedCoreHeader)
		if expectedCore != "" && (len(expectedCore) > 512 || expectedCore != strings.TrimSpace(expectedCore) || expectedCore != actor.SpaceID.String()) {
			c.AbortWithStatusJSON(http.StatusConflict, gin.H{"code": http.StatusConflict, "msg": "云端 Core 已切换，请重新加载配置后操作"})
			return
		}
		if expectedPolicy := c.GetHeader(ExpectedConfigurationPolicyHeader); expectedPolicy != "" {
			value, exists := c.Get(configurationPolicyContextKey)
			policy, valid := value.(coordination.Policy)
			current := fmt.Sprintf("%d:%d:%d", policy.ProviderEpoch, policy.ModeRevision, policy.PermissionRevision)
			if expectedCore == "" || !exists || !valid || !policy.Coordinated || !policy.Administrator || len(expectedPolicy) > 80 || expectedPolicy != current {
				c.AbortWithStatusJSON(http.StatusConflict, gin.H{"code": http.StatusConflict, "msg": "统筹模式或管理员权限已变化，请重新加载配置后操作"})
				return
			}
		}
		if actor.PrincipalType == auth.PrincipalTrustedDevice {
			if rawService, exists := c.Get(configurationPolicyServiceContextKey); exists {
				service, valid := rawService.(*coordination.Service)
				rawPolicy, _ := c.Get(configurationPolicyContextKey)
				policy, policyValid := rawPolicy.(coordination.Policy)
				if !valid || service == nil || !policyValid || !policy.Coordinated || !policy.Administrator {
					c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"code": http.StatusForbidden, "msg": "管理员授权已失效"})
					return
				}
				ctx, scope, finish, err := service.Begin(c.Request.Context(), actor.SpaceID.String(), actor.DeviceID.String(), "", actor.SpaceID.String(), "", actor.RequestID)
				if err != nil {
					c.AbortWithStatusJSON(http.StatusConflict, gin.H{"code": http.StatusConflict, "msg": "云端配置授权已变化，请重新加载"})
					return
				}
				defer finish()
				if !scope.Coordinated || scope.ProviderEpoch != policy.ProviderEpoch || scope.ModeRevision != policy.ModeRevision || scope.PermissionRevision != policy.PermissionRevision {
					c.AbortWithStatusJSON(http.StatusConflict, gin.H{"code": http.StatusConflict, "msg": "云端配置授权已变化，请重新加载"})
					return
				}
				c.Request = c.Request.WithContext(ctx)
			}
		}
		c.Next()
	}
}
