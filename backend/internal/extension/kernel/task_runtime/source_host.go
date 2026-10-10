package task_runtime

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/extension/kernel/permission"
)

type TaskHostToolRequirements func(context.Context, string) ([]permission.PermissionRequirement, error)

type TaskHostSourceEventRequest struct {
	Scope        coordination.ExecutionScope `json:"executionScope"`
	TaskRunID    string                      `json:"taskRunId"`
	Generation   int64                       `json:"generation"`
	AttemptID    string                      `json:"attemptId"`
	RequestID    string                      `json:"requestId"`
	Call         TaskHostNativeCall          `json:"call"`
	ContractOnly bool                        `json:"contractOnly,omitempty"`
	PayloadBytes []byte                      `json:"payloadBytes,omitempty"`
}

type TaskHostSourceEventPort interface {
	PublishOwnedSourceTaskEvent(context.Context, *TaskRun, *TaskDefinition, string, TaskHostNativeCall) (json.RawMessage, error)
}

func NewSourceTaskHostPermissionGuard(evaluator TaskPermissionEvaluator, resolve TaskHostToolRequirements) func(context.Context, *TaskRun, *TaskDefinition, string, TaskHostNativeCall) error {
	return func(ctx context.Context, run *TaskRun, definition *TaskDefinition, method string, call TaskHostNativeCall) error {
		if err := coordination.ValidateCurrent(ctx); err != nil {
			return err
		}
		if evaluator == nil || run == nil || definition == nil || run.TaskRunID != call.TaskRunID || definition.InstalledGeneration < 1 || run.ModuleID != definition.ModuleID || run.ExtensionID != definition.ExtensionID || run.ExecutionTarget.RuntimeID == "" || run.ExecutionTarget.RuntimeSessionID == "" || run.ExecutionTarget.ConnectionGeneration < 1 {
			return NewTaskError(ErrTaskPermissionDenied, "目标设备Native调用的独立权限端口或安装代次无效")
		}
		scope, owned := coordination.FromContext(ctx)
		if !owned || run.ExecutionTarget.DeviceID.String() != scope.TargetDeviceID || run.ExecutionTarget.SpaceID.String() != scope.CoreID {
			return coordination.ErrWrongOwner
		}
		required := []permission.PermissionRequirement{}
		input := call.Input
		targetID := call.ToolID
		if method == "task.host.executeTool" {
			if resolve == nil {
				return NewTaskError(ErrTaskDependencyUnavailable, "目标设备Native工具目录未就绪")
			}
			var err error
			required, err = resolve(ctx, call.ToolID)
			if err != nil {
				return err
			}
			required = append(required, permission.PermissionRequirement{PermissionID: "service.tool.execute", Scope: permission.ScopeForExtension(definition.ExtensionID)})
		} else if method == "task.host.emitEvent" {
			if !strings.HasPrefix(call.Type, "extension."+definition.ExtensionID+".") {
				return NewTaskError(ErrTaskPermissionDenied, "设备任务只能发布原插件命名空间中的事件")
			}
			input, targetID = call.Payload, call.Type
			required = append(required, permission.PermissionRequirement{PermissionID: "event.emit", Scope: permission.ScopeForExtension(definition.ExtensionID)})
		} else {
			return NewTaskError(ErrTaskPermissionDenied, "设备任务Native方法未授权")
		}
		declared, err := sourceTaskPermissionRequirements(definition)
		if err != nil {
			return err
		}
		declaredIDs := map[string]bool{}
		for _, value := range declared {
			declaredIDs[value.PermissionID] = true
		}
		for _, value := range required {
			if !declaredIDs[value.PermissionID] {
				return NewTaskError(ErrTaskPermissionDenied, "Native实际操作权限超出原任务已声明并批准的权限")
			}
		}
		for index := range required {
			required[index].Scope = permission.ScopeForExtension(definition.ExtensionID)
		}
		actualIDs := make(map[string]bool, len(required))
		for _, value := range required {
			actualIDs[value.PermissionID] = true
			if err := permission.ValidateInputConditions(value.Conditions, input); err != nil {
				return NewTaskError(ErrTaskPermissionDenied, "Native实际输入不符合工具权限条件："+err.Error())
			}
		}
		for _, value := range declared {
			if !actualIDs[value.PermissionID] {
				continue
			}
			if err := permission.ValidateInputConditions(value.Conditions, input); err != nil {
				return NewTaskError(ErrTaskPermissionDenied, "Native实际输入不符合原任务批准的权限条件："+err.Error())
			}
			value.Optional = false
			required = append(required, value)
		}
		copy := CloneTaskRun(run)
		if copy.ExecutionTarget.SourceTaskDefinitionID != "" {
			copy.TaskDefinitionID = copy.ExecutionTarget.SourceTaskDefinitionID
		}
		recordID, err := sourceTaskPermissionRecord(ctx, copy, definition)
		if err != nil {
			return err
		}
		target := run.ExecutionTarget
		request := permission.PermissionEvaluationRequest{ApprovalRecordID: recordID, Subject: permission.PermissionSubject{Type: permission.SubjectModule, ID: definition.ModuleID, ExtensionID: definition.ExtensionID, ModuleID: definition.ModuleID}, Requirements: required, InvocationID: run.InvocationID, Input: input, Target: permission.PermissionTarget{Type: "task-native", ID: targetID}, IsBackground: true, ScopeSnapshotID: run.ScopeSnapshotID, Generation: definition.InstalledGeneration, ExecutionContext: permission.PermissionExecutionContext{Placement: permission.ExecutionPlacementDevice, SpaceID: target.SpaceID, DeviceID: target.DeviceID, RuntimeID: target.RuntimeID, RuntimeSessionID: target.RuntimeSessionID, ProviderID: string(target.ProviderID), ProviderInstanceID: string(target.ProviderInstanceID), ExtensionID: definition.ExtensionID, ModuleID: definition.ModuleID, Source: "device-mesh-task-native"}}
		result := evaluator.Evaluate(ctx, request)
		if result.Decision != permission.DecisionAllow && result.Decision != permission.DecisionAllowPersistent && result.Decision != permission.DecisionAllowSession {
			return NewTaskError(ErrTaskPermissionDenied, "目标设备尚未批准当前Native操作或权限已经失效")
		}
		return coordination.ValidateCurrent(ctx)
	}
}

func (s *TaskRuntimeService) BindNativeHost(data coordination.DataPort, executor TaskHostNativeExecutor, guard func(context.Context, *TaskRun, *TaskDefinition, string, TaskHostNativeCall) error, publish func(context.Context, *TaskRun, *TaskDefinition, string, TaskHostNativeCall) (json.RawMessage, error), contracts ...TaskHostEventContractsResolver) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed || s.dispatchCtx != nil || s.config.OwnedHost != nil || guard == nil || executor == nil || data == nil {
		return NewTaskError(ErrTaskDependencyUnavailable, "任务Native授信依赖不完整或已经绑定")
	}
	s.config.OwnedHost = AcknowledgedTaskHostPort{Data: data, Executor: executor}
	s.config.SourceHostPermissionGuard = guard
	s.config.SourceHostCapabilities = SourceTaskCapabilities{ExecuteTool: true, EmitEvent: publish != nil}
	s.sourceEventPublisher = publish
	if len(contracts) == 1 {
		s.config.SourceEventContracts = contracts[0]
	}
	return nil
}

func (s *TaskRuntimeService) PublishSourceTaskNativeEvent(ctx context.Context, request TaskHostSourceEventRequest) (json.RawMessage, error) {
	if request.ContractOnly || len(request.PayloadBytes) > 0 {
		if len(request.PayloadBytes) == 0 || len(request.PayloadBytes) > 64<<10 || !json.Valid(request.PayloadBytes) {
			return nil, NewTaskError(ErrTaskInputInvalid, "设备事件契约缺少原始正文或正文超过上限")
		}
		request.Call.Payload = append(json.RawMessage(nil), request.PayloadBytes...)
	}
	value, exists := s.sourceHosts.Load(request.TaskRunID)
	if !exists || s.sourceEventPublisher == nil || request.RequestID == "" || len(request.RequestID) > 256 {
		return nil, NewTaskError(ErrTaskExecutionAttemptInvalid, "设备任务事件缺少当前实际执行进程")
	}
	binding := value.(*sourceTaskProcess)
	dispatch := binding.dispatch
	scope, inherited := coordination.FromContext(ctx)
	expected, active := coordination.FromContext(binding.ctx)
	if !inherited || !active || scope != expected || scope != request.Scope || !request.ContractOnly && (scope.Coordinated || scope.ResourceOwnerID != scope.TargetDeviceID) || dispatch.TaskGeneration != request.Generation || dispatch.AttemptID != request.AttemptID || request.Call.TaskRunID != dispatch.TaskRunID {
		return nil, NewTaskError(ErrTaskScopeDenied, "设备事件与原执行范围或数据所有者不一致")
	}
	var run TaskRun
	if json.Unmarshal(dispatch.RootTaskMetadata, &run) != nil {
		return nil, NewTaskError(ErrTaskScopeDenied, "设备事件缺少原任务元数据")
	}
	definition, err := s.store.GetTaskDefinition(ctx, dispatch.TaskDefinitionID)
	if err != nil {
		return nil, err
	}
	merged, cancel := context.WithCancel(binding.ctx)
	stop := context.AfterFunc(ctx, cancel)
	defer cancel()
	defer stop()
	guarded := coordination.WithAdditionalGuard(merged, func(context.Context) error { return coordination.ValidateCurrent(ctx) })
	if err := s.config.SourceHostPermissionGuard(guarded, &run, definition, "task.host.emitEvent", request.Call); err != nil {
		return nil, err
	}
	if request.ContractOnly {
		return s.sourceTaskEventContract(guarded, &run, definition, request)
	}
	return s.sourceEventPublisher(guarded, &run, definition, request.RequestID, request.Call)
}
