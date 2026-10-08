package main

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/devicemesh/business"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/extension/kernel/task_runtime"
	"github.com/u-ai/backend/internal/middleware/security"
)

type meshTaskCatalogRequest struct {
	TargetDeviceID string                       `json:"targetDeviceId"`
	RoleID         string                       `json:"characterId"`
	RequestID      string                       `json:"requestId"`
	ExpectedScope  *coordination.ExecutionScope `json:"expectedExecutionScope"`
	Cursor         string                       `json:"cursor,omitempty"`
	Limit          int                          `json:"limit,omitempty"`
}

func registerMeshTaskCatalogRouter(group *gin.RouterGroup, services *AppServices, coreID string) {
	group.POST("/tasks/catalog", func(c *gin.Context) {
		actor := security.GetActor(c)
		if !meshTaskCallerAllowed(actor, coreID) {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}
		if services.DeviceMesh == nil || !services.DeviceMesh.BusinessCoordinationReady || services.OwnedBusiness == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"message": "设备任务发现服务尚未就绪"})
			return
		}
		var input meshTaskCatalogRequest
		decoder := json.NewDecoder(http.MaxBytesReader(c.Writer, c.Request.Body, 16<<10))
		decoder.DisallowUnknownFields()
		if decoder.Decode(&input) != nil || decoder.Decode(&struct{}{}) != io.EOF || input.TargetDeviceID == "" || len(input.TargetDeviceID) > 256 || input.RoleID == "" || len(input.RoleID) > 256 || input.RequestID == "" || len(input.RequestID) > 128 || input.ExpectedScope == nil || input.ExpectedScope.CoreID != coreID || input.ExpectedScope.RoleID != input.RoleID || input.ExpectedScope.RoleRevision < 1 || len(input.Cursor) > 1024 || input.Limit < 0 || input.Limit > 8 {
			c.JSON(http.StatusBadRequest, gin.H{"message": "设备任务发现参数或权限范围无效"})
			return
		}
		ctx, authority, finish, err := services.OwnedBusiness.OpenTaskExecution(c.Request.Context(), business.Request{SpaceID: coreID, CoreID: coreID, DeviceID: actor.DeviceID.String(), TargetDeviceID: input.TargetDeviceID, RoleID: input.RoleID, RequestID: input.RequestID, ExpectedScope: input.ExpectedScope})
		if err != nil {
			writeMeshTaskSubmissionError(c, err)
			return
		}
		defer finish()
		page, err := services.DeviceMesh.TargetTaskCatalog(ctx, authority, task_runtime.DeviceTaskCatalogRequest{Cursor: input.Cursor, Limit: input.Limit})
		if err == nil {
			err = coordination.ValidateCurrent(ctx)
		}
		if err != nil {
			writeMeshTaskSubmissionError(c, err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"code": 200, "data": page})
	})
}
