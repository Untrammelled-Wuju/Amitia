package security

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/auth"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

func BeginDeviceManagementIntent(c *gin.Context, service *coordination.Service) (func(), bool) {
	actor := GetActor(c)
	if actor == nil || actor.SpaceID == "" || actor.DeviceID == "" {
		c.AbortWithStatusJSON(401, gin.H{"code": "mesh.identity_required", "message": "设备管理身份无法确认"})
		return nil, false
	}
	expectedCore, expectedPolicy := c.GetHeader(ExpectedCoreHeader), c.GetHeader(ExpectedConfigurationPolicyHeader)
	required := actor.PrincipalType == auth.PrincipalTrustedDevice
	if !required && expectedCore == "" && expectedPolicy == "" {
		return func() {}, true
	}
	reject := func(message string) (func(), bool) {
		c.AbortWithStatusJSON(http.StatusConflict, gin.H{"code": "mesh.management_scope_changed", "message": message})
		return nil, false
	}
	if service == nil || expectedCore != actor.SpaceID.String() || expectedPolicy == "" || len(expectedPolicy) > 80 {
		return reject("原Core或设备管理权限无法确认，请重新加载")
	}
	var policy coordination.Policy
	if raw, exists := c.Get(configurationPolicyContextKey); exists {
		var valid bool
		policy, valid = raw.(coordination.Policy)
		if !valid {
			return reject("认证权限状态无效")
		}
	} else if required {
		return reject("设备认证权限快照缺失")
	} else {
		var err error
		policy, err = service.Get(c.Request.Context(), actor.SpaceID.String(), actor.DeviceID.String())
		if err != nil {
			return reject("设备管理权限暂不可用")
		}
	}
	if expectedPolicy != fmt.Sprintf("%d:%d:%d", policy.ProviderEpoch, policy.ModeRevision, policy.PermissionRevision) {
		return reject("Core、统筹模式或权限已变化，请重新加载")
	}
	ctx, scope, finish, err := service.Begin(c.Request.Context(), actor.SpaceID.String(), actor.DeviceID.String(), "", actor.SpaceID.String(), "", actor.RequestID)
	if err != nil {
		return reject("设备管理授权已变化")
	}
	if scope.ProviderEpoch != policy.ProviderEpoch || scope.ModeRevision != policy.ModeRevision || scope.PermissionRevision != policy.PermissionRevision || scope.Coordinated != policy.Coordinated {
		finish()
		return reject("原设备管理状态已变化")
	}
	guarded, err := coordination.WithRequestAuthority(ctx, ctx)
	if err != nil {
		finish()
		return reject("设备管理授权已变化")
	}
	c.Request = c.Request.WithContext(guarded)
	return finish, true
}
