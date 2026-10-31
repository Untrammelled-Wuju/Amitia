package main

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/devicemesh/business"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/middleware/security"
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
				c.JSON(400, gin.H{"message": err.Error()})
				return
			}
			payload.Operation = operation
			result, err := services.OwnedBusiness.ManageChat(c.Request.Context(), business.Request{SpaceID: actor.SpaceID.String(), DeviceID: actor.DeviceID.String(), CoreID: coreID, TargetDeviceID: payload.TargetDeviceID, RoleID: payload.CharacterID, ExpectedScope: payload.ExpectedScope}, payload.ChatManagementRequest)
			if err != nil {
				c.JSON(409, gin.H{"message": err.Error()})
				return
			}
			c.JSON(200, gin.H{"code": 200, "data": result})
		})
	}
}
