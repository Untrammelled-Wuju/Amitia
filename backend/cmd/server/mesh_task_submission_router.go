package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/auth"
	"github.com/u-ai/backend/internal/devicemesh/business"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/extension/kernel/capability"
	"github.com/u-ai/backend/internal/extension/kernel/scope"
	"github.com/u-ai/backend/internal/extension/kernel/task_runtime"
	"github.com/u-ai/backend/internal/middleware/security"
	"github.com/u-ai/backend/internal/runtimeidentity"
)

type meshTaskSubmission struct {
	TaskDefinitionID     string                                      `json:"taskDefinitionId"`
	TargetDeviceID       string                                      `json:"targetDeviceId"`
	RoleID               string                                      `json:"characterId,omitempty"`
	RequestID            string                                      `json:"requestId"`
	Input                json.RawMessage                             `json:"input"`
	ExpectedCoreID       string                                      `json:"expectedCoreId"`
	ExpectedModeRevision int64                                       `json:"expectedModeRevision"`
	ExpectedRoleRevision int64                                       `json:"expectedRoleRevision"`
	ExpectedScope        *coordination.ExecutionScope                `json:"expectedExecutionScope"`
	TaskCatalogReference *task_runtime.DeviceTaskDefinitionReference `json:"taskCatalogReference,omitempty"`
}

func meshTaskCallerAllowed(actor *auth.ActorContext, coreID string) bool {
	if actor == nil || actor.SpaceID.String() != coreID || actor.DeviceID == "" {
		return false
	}
	return actor.PrincipalType == auth.PrincipalTrustedDevice || actor.PrincipalType == auth.PrincipalLocalUI && actor.IsLocalTrusted && actor.HasPermission(auth.PermSystemAdmin)
}

func registerMeshTaskSubmissionRouter(group *gin.RouterGroup, services *AppServices, coreID string) {
	registerMeshTaskCatalogRouter(group, services, coreID)
	schemas := capability.NewJSONSchemaCache()
	group.GET("/tasks/roles", func(c *gin.Context) {
		actor := security.GetActor(c)
		if !meshTaskCallerAllowed(actor, coreID) {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}
		if services.DeviceMesh == nil || !services.DeviceMesh.BusinessCoordinationReady || services.OwnedBusiness == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"message": "设备任务服务尚未就绪"})
			return
		}
		target := c.Query("targetDeviceId")
		if len(target) > 256 {
			c.JSON(http.StatusBadRequest, gin.H{"message": "目标设备无效"})
			return
		}
		roles, authority, selected, err := services.OwnedBusiness.TaskRoles(c.Request.Context(), business.Request{SpaceID: coreID, CoreID: coreID, DeviceID: actor.DeviceID.String(), TargetDeviceID: target, RequestID: uuid.NewString()})
		if err != nil {
			writeMeshTaskSubmissionError(c, err)
			return
		}
		items := make([]gin.H, 0, len(roles))
		for _, role := range roles {
			var profile struct {
				Name string `json:"name"`
			}
			if json.Unmarshal(role.Profile, &profile) != nil {
				writeMeshTaskSubmissionError(c, coordination.ErrRoleRequired)
				return
			}
			if profile.Name == "" {
				profile.Name = role.ID
			}
			items = append(items, gin.H{"id": role.ID, "name": profile.Name, "revision": role.Revision})
		}
		c.JSON(http.StatusOK, gin.H{"code": 200, "data": gin.H{"roles": items, "selectedRole": selected, "executionScope": authority, "roleOwnerId": authority.RoleOwnerID}})
	})
	group.POST("/tasks", func(c *gin.Context) {
		actor := security.GetActor(c)
		if !meshTaskCallerAllowed(actor, coreID) {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}
		if services.DeviceMesh == nil || !services.DeviceMesh.BusinessCoordinationReady || services.OwnedBusiness == nil || services.KernelContainer == nil || services.KernelContainer.TaskRuntimeService == nil || services.KernelContainer.ExecutionKernel == nil || services.KernelContainer.ExecutionKernel.ScopeStore == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"message": "设备任务服务尚未就绪"})
			return
		}
		var input meshTaskSubmission
		decoder := json.NewDecoder(http.MaxBytesReader(c.Writer, c.Request.Body, (512<<10)+(8<<10)))
		decoder.DisallowUnknownFields()
		if decoder.Decode(&input) != nil || decoder.Decode(&struct{}{}) != io.EOF || input.TaskDefinitionID == "" || len(input.TaskDefinitionID) > 256 || len(input.TargetDeviceID) > 256 || len(input.RoleID) > 256 || input.RequestID == "" || len(input.RequestID) > 128 || len(input.Input) > 512<<10 || !json.Valid(input.Input) || input.ExpectedCoreID != coreID || input.ExpectedModeRevision < 1 || input.ExpectedRoleRevision < 1 || input.ExpectedScope == nil {
			c.JSON(http.StatusBadRequest, gin.H{"message": "设备任务参数或服务提供者版本无效"})
			return
		}
		runtime := services.KernelContainer.TaskRuntimeService
		ctx, authority, finish, err := services.OwnedBusiness.OpenTaskExecution(c.Request.Context(), business.Request{ExpectedScope: input.ExpectedScope, SpaceID: coreID, CoreID: coreID, DeviceID: actor.DeviceID.String(), TargetDeviceID: input.TargetDeviceID, RoleID: input.RoleID, RequestID: input.RequestID})
		if err != nil {
			writeMeshTaskSubmissionError(c, err)
			return
		}
		defer finish()
		if authority.ModeRevision != input.ExpectedModeRevision || authority.RoleRevision != input.ExpectedRoleRevision {
			writeMeshTaskSubmissionError(c, coordination.ErrScopeExpired)
			return
		}
		existing, err := runtime.ExistingOwnedDeviceTaskRequest(ctx, input.TaskDefinitionID, input.Input, input.TaskCatalogReference)
		if err != nil {
			writeMeshTaskSubmissionError(c, err)
			return
		}
		if existing != nil {
			c.JSON(http.StatusOK, gin.H{"code": 200, "data": gin.H{"task": existing, "executionScope": authority}})
			return
		}
		var definition *task_runtime.TaskDefinition
		if reference := input.TaskCatalogReference; reference != nil {
			if reference.CatalogID != input.TaskDefinitionID || reference.CoreID != authority.CoreID || reference.DeviceID != authority.TargetDeviceID {
				writeMeshTaskSubmissionError(c, coordination.ErrWrongOwner)
				return
			}
			entry, readErr := services.DeviceMesh.TargetTaskCatalogEntry(ctx, *reference)
			if readErr != nil {
				writeMeshTaskSubmissionError(c, readErr)
				return
			}
			definition, err = runtime.ImportDeviceTaskDefinition(ctx, entry)
		} else {
			definition, err = runtime.GetTaskDefinition(ctx, input.TaskDefinitionID)
			if err == nil && definition != nil && definition.RemoteSource != nil {
				err = task_runtime.NewTaskError(task_runtime.ErrTaskDefinitionInvalid, "设备目录任务必须携带来源设备与版本引用")
			}
		}
		if err != nil || definition == nil {
			if input.TaskCatalogReference != nil || definition != nil {
				writeMeshTaskSubmissionError(c, err)
			} else {
				c.JSON(http.StatusNotFound, gin.H{"message": "设备任务定义不存在"})
			}
			return
		}
		if len(definition.InputSchema) != 0 && schemas.Validate(definition.InputSchema, input.Input) != nil {
			c.JSON(http.StatusBadRequest, gin.H{"message": "任务输入不符合已安装任务的参数要求"})
			return
		}
		target, err := resolveMeshTaskSubmissionTarget(services, authority, definition)
		var pin task_runtime.TargetTaskDefinitionPin
		if err == nil {
			pin, err = services.DeviceMesh.TargetTaskDefinition(ctx, authority, task_runtime.SourceTaskDefinitionID(definition))
			if err == nil {
				err = task_runtime.ValidateTargetTaskDefinition(authority.TargetDeviceID, definition, pin)
			}
		}
		if err != nil {
			writeMeshTaskSubmissionError(c, err)
			return
		}
		id, err := task_runtime.OwnedRequestTaskRunID(authority)
		if err == nil && (len(definition.PermissionRequirements) != 0 || len(definition.PermissionRequirementStrings) != 0) {
			digest := sha256.Sum256(input.Input)
			permissionRequest := task_runtime.SourceTaskPermissionRequest{Scope: authority, Target: pin, Input: input.Input, Run: task_runtime.TaskRun{TaskRunID: id, TaskDefinitionID: task_runtime.SourceTaskDefinitionID(definition), ExtensionID: definition.ExtensionID, ModuleID: definition.ModuleID, InvocationID: id, ScopeSnapshotID: id, InputHash: hex.EncodeToString(digest[:]), ExecutionPlacement: task_runtime.TaskExecutionPlacementDevice, ExecutionTarget: target}}
			err = services.DeviceMesh.TargetTaskPermissions(ctx, permissionRequest)
		}
		if err == nil {
			err = saveMeshTaskSubmissionSnapshot(ctx, services.KernelContainer.ExecutionKernel.ScopeStore, authority, definition, id)
		}
		if err != nil {
			writeMeshTaskSubmissionError(c, err)
			return
		}
		result, err := runtime.Enqueue(ctx, task_runtime.EnqueueTaskRequest{DeduplicateOwnedRequest: true, TaskDefinitionID: definition.TaskID, ExtensionID: definition.ExtensionID, ModuleID: definition.ModuleID, Input: input.Input, InvocationID: id, ScopeSnapshotID: id, Source: "device-mesh", ExecutionPlacement: task_runtime.TaskExecutionPlacementDevice, TrustedExecutionTarget: &task_runtime.TrustedExecutionTargetRequest{Placement: task_runtime.TaskExecutionPlacementDevice, Target: target, ResolvedBy: "core"}}, definition)
		if err == nil {
			err = coordination.ValidateCurrent(ctx)
		}
		if err != nil {
			writeMeshTaskSubmissionError(c, err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"code": 200, "data": gin.H{"task": result, "executionScope": authority}})
	})
}

func resolveMeshTaskSubmissionTarget(services *AppServices, authority coordination.ExecutionScope, definition *task_runtime.TaskDefinition) (task_runtime.TaskExecutionTarget, error) {
	if services.DeviceMesh.Hub == nil {
		return task_runtime.TaskExecutionTarget{}, task_runtime.NewTaskError(task_runtime.ErrTaskDependencyUnavailable, "目标设备任务服务不可用")
	}
	connection, ok := services.DeviceMesh.Hub.GetByDevice(runtimeidentity.SpaceID(authority.SpaceID), runtimeidentity.DeviceID(authority.TargetDeviceID))
	if !ok || connection.RuntimeID == "" || connection.SessionID == "" || connection.Generation < 1 {
		return task_runtime.TaskExecutionTarget{}, task_runtime.NewTaskError(task_runtime.ErrTaskDependencyUnavailable, "目标设备尚未连接")
	}
	if source := definition.RemoteSource; source != nil {
		if source.Reference.CoreID != authority.CoreID || source.Reference.DeviceID != authority.TargetDeviceID || source.Reference.CatalogID != definition.TaskID {
			return task_runtime.TaskExecutionTarget{}, coordination.ErrWrongOwner
		}
		encoded, err := json.Marshal([]string{definition.TaskID, connection.RuntimeID.String()})
		if err != nil {
			return task_runtime.TaskExecutionTarget{}, err
		}
		digest := sha256.Sum256(encoded)
		return task_runtime.TaskExecutionTarget{SourceTaskDefinitionID: source.Reference.SourceTaskID, ProviderID: capability.ProviderID(definition.TaskID), ProviderInstanceID: capability.ProviderInstanceID("mesh-task-instance-" + hex.EncodeToString(digest[:])), SpaceID: connection.SpaceID, DeviceID: connection.DeviceID, RuntimeID: connection.RuntimeID, RuntimeSessionID: connection.SessionID, ConnectionGeneration: connection.Generation}, nil
	}
	if services.KernelContainer.CapabilityProviders == nil {
		return task_runtime.TaskExecutionTarget{}, task_runtime.NewTaskError(task_runtime.ErrTaskDependencyUnavailable, "目标设备任务服务不可用")
	}
	providers := services.KernelContainer.CapabilityProviders
	var selected *capability.CapabilityProviderInstance
	for _, provider := range providers.ListByExtension(definition.ExtensionID) {
		if provider.ModuleID != definition.ModuleID || provider.Placement != capability.ProviderPlacementDevice || provider.Runtime.RuntimeType != capability.RuntimeTypeTask || provider.Runtime.RuntimeID != definition.TaskID && provider.Runtime.HandlerName != definition.TaskID {
			continue
		}
		for _, instance := range providers.ResolveAvailableInstances(provider.CapabilityID, capability.FilterPlacement(capability.ProviderPlacementDevice), capability.Owner{SpaceID: runtimeidentity.SpaceID(authority.SpaceID), DeviceID: runtimeidentity.DeviceID(authority.TargetDeviceID)}, runtimeidentity.Identity{}) {
			if instance.ProviderID != provider.ID || instance.ModuleID != definition.ModuleID || instance.ExtensionID != definition.ExtensionID || instance.RuntimeID != connection.RuntimeID {
				continue
			}
			if selected != nil {
				return task_runtime.TaskExecutionTarget{}, task_runtime.NewTaskError(task_runtime.ErrTaskProviderBindingInvalid, "目标设备存在多个任务服务，请先完成服务绑定")
			}
			selected = instance
		}
	}
	if selected == nil {
		return task_runtime.TaskExecutionTarget{}, task_runtime.NewTaskError(task_runtime.ErrTaskDependencyUnavailable, "目标设备没有可用的已安装任务服务")
	}
	return task_runtime.TaskExecutionTarget{ProviderID: selected.ProviderID, ProviderInstanceID: selected.ID, SpaceID: connection.SpaceID, DeviceID: connection.DeviceID, RuntimeID: connection.RuntimeID, RuntimeSessionID: connection.SessionID, ConnectionGeneration: connection.Generation}, nil
}

func saveMeshTaskSubmissionSnapshot(ctx context.Context, store scope.ScopeStore, authority coordination.ExecutionScope, definition *task_runtime.TaskDefinition, id string) error {
	encoded, err := json.Marshal(authority)
	if err != nil {
		return err
	}
	check := func(saved scope.ScopeSnapshot) error {
		if saved.SnapshotID != id || saved.InvocationID != id || saved.SpaceID != authority.SpaceID || saved.CharacterID != authority.RoleID || saved.ExtensionID != definition.ExtensionID || saved.ModuleID != definition.ModuleID || !bytes.Equal(saved.OwnedExecutionScope, encoded) {
			return coordination.ErrRequestConflict
		}
		return coordination.ValidateCurrent(ctx)
	}
	saved, err := store.GetSnapshot(ctx, id)
	if err == nil {
		return check(saved)
	}
	if !errors.Is(err, scope.ErrSnapshotNotFound) {
		return err
	}
	candidate := scope.ScopeSnapshot{SnapshotID: id, InvocationID: id, SpaceID: authority.SpaceID, CharacterID: authority.RoleID, ExtensionID: definition.ExtensionID, ModuleID: definition.ModuleID, OwnedExecutionScope: encoded, CreatedAt: time.Now().UTC()}
	if err := coordination.ValidateCurrent(ctx); err != nil {
		return err
	}
	if err := store.SaveSnapshot(ctx, candidate); err != nil {
		saved, readErr := store.GetSnapshot(ctx, id)
		if readErr != nil {
			return err
		}
		return check(saved)
	}
	return coordination.ValidateCurrent(ctx)
}

func writeMeshTaskSubmissionError(c *gin.Context, err error) {
	status := http.StatusConflict
	if errors.Is(err, coordination.ErrWrongOwner) || errors.Is(err, coordination.ErrCapabilityGrant) || task_runtime.IsTaskErrorCode(err, task_runtime.ErrTaskScopeDenied) || task_runtime.IsTaskErrorCode(err, task_runtime.ErrTaskPermissionDenied) {
		status = http.StatusForbidden
	} else if task_runtime.IsTaskErrorCode(err, task_runtime.ErrTaskDependencyUnavailable) {
		status = http.StatusServiceUnavailable
	}
	c.JSON(status, gin.H{"message": err.Error()})
}
