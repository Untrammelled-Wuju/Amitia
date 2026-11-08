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

func SharedCoreDataWriteAllowed(actor *auth.ActorContext, policy coordination.Policy) bool {
	if actor == nil {
		return false
	}
	if actor.IsLocalTrusted || actor.IsHostDevice {
		return true
	}
	return actor.PrincipalType == auth.PrincipalTrustedDevice && actor.HasPermission(auth.PermSystemAdmin) && policy.Coordinated && policy.Administrator
}

func SharedCoreDataWriteOnly() gin.HandlerFunc {
	return func(c *gin.Context) {
		actor := GetActor(c)
		if actor == nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"code": "mesh.identity_required", "message": "请先配对当前设备", "msg": "请先配对当前设备", "error": "请先配对当前设备", "rawError": "请先配对当前设备"})
			return
		}
		expectedCore := c.GetHeader(ExpectedCoreHeader)
		if expectedCore != "" && (len(expectedCore) > 512 || expectedCore != strings.TrimSpace(expectedCore) || expectedCore != actor.SpaceID.String()) {
			const message = "统筹中心已切换，请重新加载中心数据"
			c.AbortWithStatusJSON(http.StatusConflict, gin.H{"code": "mesh.core_data_scope_expired", "message": message, "msg": message, "error": message, "rawError": message})
			return
		}
		if actor.IsLocalTrusted || actor.IsHostDevice {
			c.Next()
			return
		}
		rawService, serviceExists := c.Get(configurationPolicyServiceContextKey)
		service, serviceValid := rawService.(*coordination.Service)
		var policy coordination.Policy
		var policyValid bool
		var err error
		if serviceExists && serviceValid && service != nil {
			policy, err = service.Get(c.Request.Context(), actor.SpaceID.String(), actor.DeviceID.String())
			if err != nil {
				const message = "无法确认统筹中心数据写权限"
				c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"code": "mesh.core_data_scope_unavailable", "message": message, "msg": message, "error": message, "rawError": err.Error(), "cause": err.Error()})
				return
			}
			policyValid = true
		} else {
			rawPolicy, exists := c.Get(configurationPolicyContextKey)
			policy, policyValid = rawPolicy.(coordination.Policy)
			policyValid = exists && policyValid
		}
		if !policyValid || !policy.Coordinated || !policy.Administrator || !actor.HasPermission(auth.PermSystemAdmin) {
			const message = "当前设备没有统筹中心管理员权限，只能查看中心数据"
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"code": "mesh.core_data_write_forbidden", "message": message, "msg": message, "error": message, "rawError": message})
			return
		}
		if expectedPolicy := c.GetHeader(ExpectedConfigurationPolicyHeader); expectedPolicy != "" {
			current := fmt.Sprintf("%d:%d:%d", policy.ProviderEpoch, policy.ModeRevision, policy.PermissionRevision)
			if expectedCore == "" || len(expectedPolicy) > 80 || expectedPolicy != current {
				const message = "统筹中心数据权限已变化，请重新加载后操作"
				c.AbortWithStatusJSON(http.StatusConflict, gin.H{"code": "mesh.core_data_scope_expired", "message": message, "msg": message, "error": message, "rawError": message})
				return
			}
		}
		c.Next()
	}
}

// SharedCoreAdminOnly protects server-global configuration and control surfaces.
// Local single-user runtimes keep their historical behaviour; a shared Cloud
// Core requires an authenticated administrator actor.
func SharedCoreAdminOnly() gin.HandlerFunc {
	return func(c *gin.Context) {
		actor := GetActor(c)
		if actor == nil || !actor.HasPermission(auth.PermSystemAdmin) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"code": http.StatusForbidden,
				"msg":  "当前设备没有统筹中心管理员权限，模型与系统配置只能由中心设备或获授权的管理员修改",
			})
			return
		}
		expectedCore := c.GetHeader(ExpectedCoreHeader)
		if expectedCore != "" && (len(expectedCore) > 512 || expectedCore != strings.TrimSpace(expectedCore) || expectedCore != actor.SpaceID.String()) {
			c.AbortWithStatusJSON(http.StatusConflict, gin.H{"code": http.StatusConflict, "msg": "统筹中心已切换，请重新加载配置后操作"})
			return
		}
		if expectedPolicy := c.GetHeader(ExpectedConfigurationPolicyHeader); expectedPolicy != "" {
			value, exists := c.Get(configurationPolicyContextKey)
			policy, valid := value.(coordination.Policy)
			current := fmt.Sprintf("%d:%d:%d", policy.ProviderEpoch, policy.ModeRevision, policy.PermissionRevision)
			if expectedCore == "" || !exists || !valid || !actor.IsHostDevice && (!policy.Coordinated || !policy.Administrator) || len(expectedPolicy) > 80 || expectedPolicy != current {
				c.AbortWithStatusJSON(http.StatusConflict, gin.H{"code": http.StatusConflict, "msg": "数据归属或管理员权限已变化，请重新加载配置后操作"})
				return
			}
		}
		if actor.PrincipalType == auth.PrincipalTrustedDevice && !actor.IsHostDevice {
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
