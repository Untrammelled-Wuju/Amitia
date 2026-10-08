package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/extension/kernel/task_runtime"
)

func withLocalSourceTaskApproval(ctx context.Context, services *AppServices, approval task_runtime.SourceTaskApproval, execute func(context.Context) error) error {
	if services == nil || services.DeviceMesh == nil || services.DeviceMesh.LocalHandler == nil || services.KernelContainer == nil || services.KernelContainer.DeviceRegistry == nil || services.KernelContainer.TaskRuntimeService == nil {
		return coordination.ErrWrongOwner
	}
	local := services.DeviceMesh.LocalHandler
	identity, err := local.LoadIdentity()
	if err != nil {
		return err
	}
	credential, err := local.LoadCredential()
	if err != nil {
		return err
	}
	scope := approval.Binding.Scope
	if identity == nil || credential == nil || !credential.ExpiresAt.After(time.Now()) || credential.SpaceID.String() != scope.CoreID || identity.DeviceID.String() != scope.TargetDeviceID || credential.DeviceID != identity.DeviceID || credential.RuntimeID != identity.RuntimeID || approval.Binding.ExecutionTarget.RuntimeID != identity.RuntimeID {
		return coordination.ErrWrongOwner
	}
	target := approval.Binding.ExecutionTarget
	return local.CredentialStore().WithActiveSessionCredential(ctx, credential, target.RuntimeSessionID, target.ConnectionGeneration, func(current context.Context) error {
		current = coordination.WithScope(current, scope)
		current = coordination.WithAdditionalGuard(current, func(check context.Context) error {
			return coordination.ValidateSourceAuthority(check, services.KernelContainer.DeviceRegistry.Database(), scope)
		})
		check := func() error {
			if err := services.KernelContainer.TaskRuntimeService.ValidateSourceTaskApproval(current, approval); err != nil {
				return err
			}
			if err := execute(current); err != nil {
				return err
			}
			return coordination.ValidateCurrent(current)
		}
		if scope.Coordinated {
			return check()
		}
		port, ok := services.DeviceMesh.LocalDeviceDataPort.(*meshLocalDataPort)
		if !ok {
			return coordination.ErrWrongOwner
		}
		return port.WithSourceRole(current, scope, check)
	})
}

func registerLocalSourceTaskApprovalRouter(group *gin.RouterGroup, services *AppServices) {
	group.POST("/task-approvals/:id/revoke", func(c *gin.Context) {
		var body struct {
			Revision int64 `json:"expectedRevision"`
		}
		decoder := json.NewDecoder(http.MaxBytesReader(c.Writer, c.Request.Body, 1024))
		decoder.DisallowUnknownFields()
		if decoder.Decode(&body) != nil || decoder.Decode(&struct{}{}) != io.EOF || body.Revision < 1 || len(c.Param("id")) > 128 {
			c.JSON(http.StatusBadRequest, gin.H{"message": "撤销审批的版本无效"})
			return
		}
		if services == nil || services.KernelContainer == nil || services.KernelContainer.TaskRuntimeService == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"message": "设备任务审批服务不可用"})
			return
		}
		runtime := services.KernelContainer.TaskRuntimeService
		approval, err := runtime.SourceTaskApproval(c.Param("id"))
		if err == nil {
			err = withLocalSourceTaskApproval(c.Request.Context(), services, approval, func(current context.Context) error {
				approval, err = runtime.RevokeSourceTaskApproval(current, approval.ID, body.Revision)
				return err
			})
		}
		if err != nil {
			writeMeshTaskSubmissionError(c, err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"code": 200, "data": approval})
	})
	group.GET("/task-approvals", func(c *gin.Context) {
		if services == nil || services.KernelContainer == nil || services.KernelContainer.TaskRuntimeService == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"message": "设备任务审批服务不可用"})
			return
		}
		runtime := services.KernelContainer.TaskRuntimeService
		items := make([]task_runtime.SourceTaskApprovalDetails, 0)
		for _, approval := range runtime.ListSourceTaskApprovals() {
			var details task_runtime.SourceTaskApprovalDetails
			if err := withLocalSourceTaskApproval(c.Request.Context(), services, approval, func(current context.Context) error {
				var err error
				details, err = runtime.DescribeSourceTaskApproval(current, approval)
				return err
			}); err != nil {
				continue
			}
			items = append(items, details)
		}
		c.JSON(http.StatusOK, gin.H{"code": 200, "data": items})
	})
	group.POST("/task-approvals/:id/decision", func(c *gin.Context) {
		var body struct {
			Revision int64 `json:"expectedRevision"`
			Approved *bool `json:"approved"`
		}
		decoder := json.NewDecoder(http.MaxBytesReader(c.Writer, c.Request.Body, 1024))
		decoder.DisallowUnknownFields()
		if decoder.Decode(&body) != nil || decoder.Decode(&struct{}{}) != io.EOF || body.Revision < 1 || body.Approved == nil || len(c.Param("id")) > 128 {
			c.JSON(http.StatusBadRequest, gin.H{"message": "审批决定或版本无效"})
			return
		}
		if services == nil || services.KernelContainer == nil || services.KernelContainer.TaskRuntimeService == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"message": "设备任务审批服务不可用"})
			return
		}
		runtime := services.KernelContainer.TaskRuntimeService
		approval, err := runtime.SourceTaskApproval(c.Param("id"))
		if err == nil {
			err = withLocalSourceTaskApproval(c.Request.Context(), services, approval, func(current context.Context) error {
				approval, err = runtime.DecideSourceTaskApproval(current, approval.ID, body.Revision, *body.Approved)
				return err
			})
		}
		if err != nil {
			writeMeshTaskSubmissionError(c, err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"code": 200, "data": approval})
	})
}
