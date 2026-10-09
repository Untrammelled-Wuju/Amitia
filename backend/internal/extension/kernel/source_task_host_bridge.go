package kernel

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/extension/kernel/capability"
	"github.com/u-ai/backend/internal/extension/kernel/event"
	"github.com/u-ai/backend/internal/extension/kernel/execution"
	"github.com/u-ai/backend/internal/extension/kernel/permission"
	"github.com/u-ai/backend/internal/extension/kernel/task_runtime"
)

type sourceTaskHostBridge struct {
	pipeline  *execution.ExecutionPipeline
	tools     *capability.ToolRegistry
	events    *event.RuntimeBridge
	owner     *task_runtime.OwnedRuntimeBinding
	providers *capability.ProviderRegistry
}

func (b *sourceTaskHostBridge) nativeTarget(tool capability.ToolDefinition, target task_runtime.TaskExecutionTarget) (capability.InvocationExecutionTarget, error) {
	if b.providers == nil || tool.CapabilityID == "" {
		return capability.InvocationExecutionTarget{}, errors.New("任务Native设备能力目录未就绪")
	}
	var selected *capability.CapabilityProviderInstance
	for _, instance := range b.providers.ListExecutableInstances(tool.CapabilityID) {
		if instance.Placement != capability.ProviderPlacementDevice || instance.SpaceID != target.SpaceID || instance.DeviceID != target.DeviceID || instance.RuntimeID != target.RuntimeID {
			continue
		}
		provider, exists := b.providers.GetByID(instance.ProviderID)
		if !exists || provider.Placement != capability.ProviderPlacementDevice || provider.CapabilityID != tool.CapabilityID || provider.Runtime.RuntimeType != tool.Runtime.RuntimeType || provider.Runtime.Endpoint != "" {
			continue
		}
		if selected != nil {
			return capability.InvocationExecutionTarget{}, errors.New("原设备Native能力存在多个可执行实例，必须消除歧义后调用")
		}
		selected = instance
	}
	if selected == nil {
		return capability.InvocationExecutionTarget{}, errors.New("原设备没有当前Native工具的可执行能力实例，禁止回退到Core执行")
	}
	return capability.InvocationExecutionTarget{Placement: "device", ProviderID: string(selected.ProviderID), ProviderInstanceID: string(selected.ID), SpaceID: target.SpaceID, DeviceID: target.DeviceID, RuntimeID: target.RuntimeID, RuntimeSessionID: target.RuntimeSessionID}, nil
}

func nativeTaskTool(tool capability.ToolDefinition) bool {
	switch tool.Runtime.RuntimeType {
	case capability.RuntimeTypeAndroid_Native, capability.RuntimeTypeIOS_Native, capability.RuntimeTypeDesktop_Extension:
		return tool.Runtime.HandlerName != "" && tool.Runtime.Endpoint == ""
	default:
		return false
	}
}

func (b *sourceTaskHostBridge) toolRequirements(ctx context.Context, toolID string) ([]permission.PermissionRequirement, error) {
	tool, ok := b.tools.Get(ctx, toolID)
	if !ok || !nativeTaskTool(tool) {
		return nil, errors.New("设备任务只能调用已注册的Native工具，不能调用通用本机或云端接口")
	}
	result := make([]permission.PermissionRequirement, 0, len(tool.Permissions))
	for _, value := range tool.Permissions {
		result = append(result, permission.PermissionRequirement{PermissionID: value.Capability, Scope: permission.ScopeForExtension(tool.ExtensionID)})
	}
	return result, nil
}

func (b *sourceTaskHostBridge) Execute(ctx context.Context, run *task_runtime.TaskRun, definition *task_runtime.TaskDefinition, requestID, method string, call task_runtime.TaskHostNativeCall) (json.RawMessage, error) {
	scope, owned := coordination.FromContext(ctx)
	if !owned || b.owner == nil || run == nil || run.ExecutionTarget.DeviceID.String() != scope.TargetDeviceID || run.ExecutionTarget.SpaceID.String() != scope.SpaceID || run.ExecutionTarget.RuntimeSessionID == "" || run.ExecutionTarget.ConnectionGeneration < 1 {
		return nil, coordination.ErrWrongOwner
	}
	if err := coordination.ValidateCurrent(ctx); err != nil {
		return nil, err
	}
	if method == "task.host.emitEvent" {
		if !scope.Coordinated {
			return b.owner.PublishOwnedSourceTaskEvent(ctx, run, definition, requestID, call)
		}
		return b.publish(ctx, run, definition, requestID, call)
	}
	if method != "task.host.executeTool" || b.pipeline == nil || b.tools == nil {
		return nil, errors.New("任务Native工具执行依赖未就绪")
	}
	tool, ok := b.tools.Get(ctx, call.ToolID)
	if !ok || !nativeTaskTool(tool) {
		return nil, errors.New("任务Native工具不存在或执行边界不支持")
	}
	declared := map[string]bool{}
	for _, value := range definition.PermissionRequirements {
		declared[value.PermissionID] = true
	}
	for _, value := range definition.PermissionRequirementStrings {
		declared[value] = true
	}
	if !declared["service.tool.execute"] {
		return nil, errors.New("原任务未声明Native工具执行权限")
	}
	for _, value := range tool.Permissions {
		if !declared[value.Capability] {
			return nil, errors.New("Native工具实际权限超过原任务声明")
		}
	}
	target, err := b.nativeTarget(tool, run.ExecutionTarget)
	if err != nil {
		return nil, err
	}
	pin, err := b.owner.TargetTaskDefinition(ctx, scope, task_runtime.SourceTaskDefinitionID(definition))
	if err != nil {
		return nil, err
	}
	if err := task_runtime.ValidateTargetTaskDefinition(scope.TargetDeviceID, definition, pin); err != nil {
		return nil, err
	}
	deadline := 5 * time.Second
	if call.TimeoutMS > 0 {
		deadline = time.Duration(call.TimeoutMS) * time.Millisecond
	}
	current, cancel := context.WithTimeout(ctx, deadline)
	defer cancel()
	idHash := sha256.Sum256([]byte(run.TaskRunID + "\x00" + run.ExecutionAttemptID.String() + "\x00" + requestID))
	id := "task-native-" + hex.EncodeToString(idHash[:16])
	target.ExtensionID, target.ModuleID = definition.ExtensionID, definition.ModuleID
	invocation := capability.ToolInvocationContext{InvocationID: id, ParentID: run.InvocationID, RootID: run.InvocationID, ExternalCallID: id, SpaceID: scope.SpaceID, CharacterID: scope.RoleID, ExtensionID: definition.ExtensionID, ModuleID: definition.ModuleID, Generation: pin.InstalledGeneration, Source: capability.InvocationSourcePlugin, ApprovalMode: capability.ApprovalModeManual, TraceID: id, OperationID: scope.RequestID, IsBackground: true, DeadlineDuration: deadline, IdempotencyKey: id, ExecutionTarget: target}
	if definition.RemoteSource == nil {
		invocation.Metadata = map[string]any{"parentScopeSnapshotId": run.ScopeSnapshotID}
	}
	pipeline := *b.pipeline
	if !scope.Coordinated {
		pipeline.AuditSink, pipeline.SideEffectRec = nil, nil
	}
	result := pipeline.Execute(current, execution.ToolExecutionRequest{ToolID: capability.CapabilityID(tool.ID), Input: call.Input, Invocation: invocation})
	if err := coordination.ValidateCurrent(current); err != nil {
		return nil, err
	}
	if result.InvocationID != id || result.Status != capability.ToolResultStatusSuccess || result.DeviceID != scope.TargetDeviceID || result.RuntimeID != run.ExecutionTarget.RuntimeID.String() || result.RuntimeSessionID != run.ExecutionTarget.RuntimeSessionID.String() {
		if result.Error != nil {
			return nil, result.Error
		}
		return nil, errors.New("Native工具未获得原设备执行连接的成功确认")
	}
	return json.Marshal(map[string]any{"result": result})
}

func (b *sourceTaskHostBridge) publish(ctx context.Context, run *task_runtime.TaskRun, definition *task_runtime.TaskDefinition, requestID string, call task_runtime.TaskHostNativeCall) (json.RawMessage, error) {
	scope, owned := coordination.FromContext(ctx)
	if !owned || b.events == nil || !strings.HasPrefix(call.Type, "extension."+definition.ExtensionID+".") {
		return nil, errors.New("任务事件发布依赖未就绪或命名空间越权")
	}
	generation := definition.InstalledGeneration
	if definition.RemoteSource != nil {
		pin, err := b.owner.TargetTaskDefinition(ctx, scope, task_runtime.SourceTaskDefinitionID(definition))
		if err != nil {
			return nil, err
		}
		generation = pin.InstalledGeneration
	}
	var result event.PublishResult
	err := coordination.CommitCurrent(ctx, func() error {
		var err error
		result, err = b.events.PublishFromRuntime(ctx, definition.ExtensionID, event.EventTypeID(call.Type), 1, call.Payload, event.PublishOptions{ProducerGeneration: generation, ProducerModuleID: definition.ModuleID, AggregateType: "source-task", AggregateID: run.TaskRunID, PartitionKey: scope.SpaceID, OrderingKey: run.TaskRunID, ScopeSnapshotID: run.ScopeSnapshotID, TraceID: requestID, OperationID: scope.RequestID})
		return err
	})
	if err != nil {
		return nil, err
	}
	if !result.Accepted || result.EventID == "" || result.OutboxID == "" {
		return nil, errors.New("任务事件尚未取得真实持久化发布确认")
	}
	if err := coordination.ValidateCurrent(ctx); err != nil {
		return nil, err
	}
	return json.Marshal(map[string]any{"confirmed": true, "eventId": result.EventID, "outboxId": result.OutboxID})
}
