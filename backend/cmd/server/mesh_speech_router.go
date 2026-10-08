package main

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/devicemesh/business"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/middleware/security"
)

func registerMeshSpeechRouter(mesh *gin.RouterGroup, services *AppServices, coreID string) {
	mesh.POST("/speech", func(c *gin.Context) {
		actor := security.GetActor(c)
		if actor == nil || actor.DeviceID == "" {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}
		var input struct {
			RequestID      string                       `json:"requestId"`
			RoleID         string                       `json:"characterId"`
			TargetDeviceID string                       `json:"targetDeviceId"`
			Text           string                       `json:"text"`
			ExpectedScope  *coordination.ExecutionScope `json:"expectedExecutionScope"`
		}
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 32<<10)
		if c.ShouldBindJSON(&input) != nil || input.ExpectedScope == nil {
			c.JSON(400, gin.H{"message": "朗读请求缺少当前角色或页面权限版本"})
			return
		}
		result, err := services.OwnedBusiness.Speech(c.Request.Context(), business.Request{SpaceID: actor.SpaceID.String(), DeviceID: actor.DeviceID.String(), CoreID: coreID, TargetDeviceID: input.TargetDeviceID, RoleID: input.RoleID, RequestID: input.RequestID, ExpectedScope: input.ExpectedScope}, input.Text)
		if err != nil {
			c.JSON(409, gin.H{"code": "mesh.speech_unavailable", "message": err.Error()})
			return
		}
		c.JSON(200, gin.H{"code": 200, "data": result})
	})
}
