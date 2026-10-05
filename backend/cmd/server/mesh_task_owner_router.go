package main

import (
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/auth"
	"github.com/u-ai/backend/internal/extension/kernel/task_runtime"
	"github.com/u-ai/backend/internal/middleware/security"
)

func registerMeshTaskOwnerRouter(group *gin.RouterGroup, services *AppServices) {
	group.POST("/tasks/:taskRunId/owner-rpc", func(c *gin.Context) {
		actor := security.GetActor(c)
		if actor == nil || actor.PrincipalType != auth.PrincipalTrustedDevice || actor.DeviceID == "" || actor.RuntimeID == "" {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}
		if services.KernelContainer == nil || services.KernelContainer.TaskRuntimeService == nil || services.DeviceMesh == nil || services.DeviceMesh.PendingTasks == nil || services.DeviceMesh.Hub == nil {
			c.JSON(503, gin.H{"message": "任务所有者服务尚未就绪"})
			return
		}
		var request task_runtime.RemoteTaskOwnerRequest
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 2<<20)
		if c.ShouldBindJSON(&request) != nil {
			c.JSON(400, gin.H{"message": "任务所有者请求无效"})
			return
		}
		id := c.Param("taskRunId")
		check := func() error {
			connection, ok := services.DeviceMesh.Hub.GetByDevice(actor.SpaceID, actor.DeviceID)
			if !ok || connection == nil || connection.RuntimeID != actor.RuntimeID || connection.SessionID.String() != request.SessionID || connection.Generation != request.ConnectionGeneration || !services.DeviceMesh.PendingTasks.ValidateOwnerBound(id, request.AttemptID, request.LeaseID, request.SessionID, request.ConnectionGeneration, request.AuthorityCallID) {
				return task_runtime.NewTaskError(task_runtime.ErrTaskRuntimeSessionBindingInvalid, "任务执行设备的连接或派发租约已失效")
			}
			return nil
		}
		result, err := services.KernelContainer.TaskRuntimeService.CallRemoteOwner(c.Request.Context(), id, request, check)
		if err != nil {
			c.JSON(409, gin.H{"message": err.Error()})
			return
		}
		c.JSON(200, gin.H{"code": 200, "data": json.RawMessage(result)})
	})
}
