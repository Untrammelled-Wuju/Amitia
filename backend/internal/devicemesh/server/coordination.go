package server

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/auth"
	"github.com/u-ai/backend/internal/middleware/security"
	"github.com/u-ai/backend/internal/runtimeidentity"
)

func canManageDevice(c *gin.Context, device string) bool {
	actor := security.GetActor(c)
	if actor != nil && (actor.DeviceID.String() == device || actor.HasPermission(auth.PermSystemAdmin)) {
		return true
	}
	c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"code": "mesh.administrator_required", "message": "只有当前设备或 Core 管理员可以执行此操作"})
	return false
}

func makePolicyHandler(deps *RouterDeps) gin.HandlerFunc {
	return func(c *gin.Context) {
		actor := security.GetActor(c)
		if actor == nil || actor.DeviceID == "" {
			c.JSON(401, gin.H{"code": "mesh.identity_required", "message": "请先配对当前设备"})
			return
		}
		if actor.HasPermission(auth.PermSystemAdmin) {
			if err := deps.Coordination.InitializeCoreConsole(c.Request.Context(), actor.SpaceID.String(), actor.DeviceID.String()); err != nil {
				c.JSON(503, gin.H{"code": "mesh.policy_unavailable", "message": err.Error()})
				return
			}
		}
		policy, err := deps.Coordination.Get(c.Request.Context(), actor.SpaceID.String(), actor.DeviceID.String())
		if err != nil {
			c.JSON(503, gin.H{"code": "mesh.policy_unavailable", "message": err.Error()})
			return
		}
		c.JSON(200, gin.H{"policy": policy, "canAdminister": actor.HasPermission(auth.PermSystemAdmin), "aiProvider": "core", "coreId": actor.SpaceID.String(), "coreConsoleDeviceId": deps.LocalCoreDeviceID.String(), "coordinationAvailable": deps.BusinessCoordinationReady})
	}
}

func makeModeHandler(deps *RouterDeps) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !deps.BusinessCoordinationReady {
			c.JSON(503, gin.H{"code": "mesh.coordination_not_ready", "message": "设备角色、聊天及记忆的数据归属适配尚未就绪，暂不能切换统筹模式"})
			return
		}
		actor := security.GetActor(c)
		if actor == nil || actor.DeviceID == "" {
			c.JSON(401, gin.H{"code": "mesh.identity_required", "message": "请先配对当前设备"})
			return
		}
		var req struct {
			Coordinated      bool   `json:"coordinated"`
			ExpectedRevision int64  `json:"expectedRevision" binding:"required"`
			SelectedRole     string `json:"selectedRole"`
		}
		finish, valid := security.BeginDeviceManagementIntent(c, deps.Coordination)
		if !valid {
			return
		}
		defer finish()
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(400, gin.H{"code": "mesh.invalid_policy", "message": err.Error()})
			return
		}
		policy, err := deps.Coordination.ChangeMode(c.Request.Context(), actor.SpaceID.String(), actor.DeviceID.String(), req.ExpectedRevision, req.Coordinated, req.SelectedRole)
		if err != nil {
			c.JSON(409, gin.H{"code": "mesh.policy_conflict", "message": err.Error()})
			return
		}
		c.JSON(200, gin.H{"policy": policy, "message": "统筹模式已更新，正在进行的回复已中断，历史数据保持原归属"})
	}
}

func makeAdministratorHandler(deps *RouterDeps) gin.HandlerFunc {
	return func(c *gin.Context) {
		actor := security.GetActor(c)
		if actor == nil || !actor.HasPermission(auth.PermSystemAdmin) {
			c.JSON(403, gin.H{"code": "mesh.administrator_required", "message": "管理员权限只能由当前 Core 授予"})
			return
		}
		device := runtimeidentity.ParseDeviceID(c.Param("deviceId"))
		finish, valid := security.BeginDeviceManagementIntent(c, deps.Coordination)
		if !valid {
			return
		}
		defer finish()
		if err := deps.DeviceReg.RequireTrustedDevice(c.Request.Context(), actor.SpaceID, device); err != nil {
			c.JSON(403, gin.H{"code": "mesh.device_not_trusted", "message": "设备未处于有效配对状态"})
			return
		}
		var req struct {
			Grant            bool  `json:"grant"`
			ExpectedRevision int64 `json:"expectedRevision" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(400, gin.H{"code": "mesh.invalid_policy", "message": err.Error()})
			return
		}
		policy, err := deps.Coordination.GrantAdministrator(c.Request.Context(), actor.SpaceID.String(), device.String(), req.ExpectedRevision, req.Grant)
		if err != nil {
			c.JSON(409, gin.H{"code": "mesh.policy_conflict", "message": err.Error()})
			return
		}
		c.JSON(200, gin.H{"policy": policy})
	}
}
