package main

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/devicemesh/business"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/middleware/security"
	"github.com/u-ai/backend/pkg/util"
)

func registerMeshChatManagementRouter(mesh *gin.RouterGroup, services *AppServices, coreID string) {
	for _, operation := range []string{"stats", "search", "export", "from-import", "import-promoted", "compression-status", "context-preview", "feedback-stats", "feedback/list", "feedback/create", "feedback/delete", "cleanup/preview", "cleanup/confirm", "cleanup/vacuum"} {
		operation := operation
		mesh.POST("/chat-management/"+operation, func(c *gin.Context) {
			actor := security.GetActor(c)
			if actor == nil || actor.DeviceID == "" {
				c.AbortWithStatus(401)
				return
			}
			var payload struct {
				coordination.ChatManagementRequest
				TargetDeviceID string                       `json:"targetDeviceId"`
				CharacterID    string                       `json:"characterId"`
				ExpectedScope  *coordination.ExecutionScope `json:"expectedExecutionScope"`
			}
			c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 1<<20)
			if err := c.ShouldBindJSON(&payload); err != nil || payload.ExpectedScope == nil || strings.TrimSpace(payload.CharacterID) == "" {
				c.JSON(400, gin.H{"message": util.OriginalErrorMessage(err, "聊天管理参数无效或缺少原服务范围")})
				return
			}
			payload.Operation = operation
			if payload.TargetDeviceID != "" && payload.TargetDeviceID != actor.DeviceID.String() {
				c.JSON(403, gin.H{"message": "会话属于设备本地数据，不能管理其他设备会话"})
				return
			}
			if payload.ExpectedScope.SpaceID != actor.SpaceID.String() || payload.ExpectedScope.InitiatorDeviceID != actor.DeviceID.String() || payload.ExpectedScope.CoreID != coreID || payload.ExpectedScope.RoleID != payload.CharacterID || payload.ExpectedScope.TargetDeviceID != actor.DeviceID.String() {
				c.JSON(409, gin.H{"message": "聊天管理权限或数据归属已变化，请刷新"})
				return
			}
			result, err := services.OwnedBusiness.ManageChat(c.Request.Context(), business.Request{SpaceID: actor.SpaceID.String(), DeviceID: actor.DeviceID.String(), CoreID: coreID, TargetDeviceID: payload.ExpectedScope.TargetDeviceID, ResourceOwnerID: payload.ExpectedScope.ResourceOwnerID, RoleOwnerID: payload.ExpectedScope.RoleOwnerID, RoleID: payload.CharacterID, ExpectedScope: payload.ExpectedScope}, payload.ChatManagementRequest)
			if err != nil {
				c.JSON(409, gin.H{"message": err.Error()})
				return
			}
			c.JSON(200, gin.H{"code": 200, "data": result})
		})
	}
}
