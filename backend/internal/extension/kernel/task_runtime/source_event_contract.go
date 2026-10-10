package task_runtime

import (
	"context"
	"encoding/json"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/extension/kernel/event"
)

type TaskHostEventContractsResolver func(context.Context, *TaskDefinition) ([]event.EventTypeDefinition, error)

type TaskHostEventContract struct {
	Scope       coordination.ExecutionScope `json:"executionScope"`
	TaskRunID   string                      `json:"taskRunId"`
	Generation  int64                       `json:"generation"`
	AttemptID   string                      `json:"attemptId"`
	RequestID   string                      `json:"requestId"`
	PayloadHash string                      `json:"payloadHash"`
	Target      TargetTaskDefinitionPin     `json:"target"`
	Definition  event.EventTypeDefinition   `json:"definition"`
}

type TaskHostSourceEventContractPort interface {
	SourceTaskEventContract(context.Context, *TaskRun, *TaskDefinition, string, TaskHostNativeCall) (TaskHostEventContract, error)
}

func (s *TaskRuntimeService) validateSourceHostCapabilities(ctx context.Context, definition *TaskDefinition) error {
	if err := validateSourceTaskDeclaredCapabilitiesWith(definition, s.declaredHostCapabilities()); err != nil {
		return err
	}
	if s.config.SourceEventContracts == nil {
		return nil
	}
	requirements, err := sourceTaskPermissionRequirements(definition)
	if err != nil {
		return err
	}
	for _, requirement := range requirements {
		if requirement.PermissionID != "event.emit" {
			continue
		}
		contracts, err := s.config.SourceEventContracts(ctx, definition)
		if err != nil || len(contracts) == 0 {
			return NewTaskError(ErrTaskDependencyUnavailable, "当前设备插件缺少固定安装代次的事件契约，尚不支持事件任务")
		}
		usable := false
		for _, contract := range contracts {
			if contract.Version == 1 && !contract.EventTypeID.IsReservedNamespace() && contract.EventTypeID.IsExtensionNamespace(definition.ExtensionID) && contract.DefinitionHash == contract.Hash() {
				usable = true
				break
			}
		}
		if !usable {
			return NewTaskError(ErrTaskDependencyUnavailable, "当前设备插件没有有效的原命名空间事件契约")
		}
	}
	return nil
}

func (s *TaskRuntimeService) sourceTaskEventContract(ctx context.Context, run *TaskRun, definition *TaskDefinition, request TaskHostSourceEventRequest) (json.RawMessage, error) {
	if s.config.SourceEventContracts == nil || !request.Scope.Coordinated {
		return nil, NewTaskError(ErrTaskDependencyUnavailable, "原设备固定事件契约读取端口未就绪")
	}
	contracts, err := s.config.SourceEventContracts(ctx, definition)
	if err != nil {
		return nil, err
	}
	for _, contract := range contracts {
		if string(contract.EventTypeID) != request.Call.Type || contract.Version != 1 || contract.EventTypeID.IsReservedNamespace() || !contract.EventTypeID.IsExtensionNamespace(definition.ExtensionID) || contract.DefinitionHash != contract.Hash() {
			continue
		}
		target, err := s.DescribeInstalledTask(ctx, definition.TaskID, request.Scope.TargetDeviceID)
		if err != nil {
			return nil, err
		}
		if err := coordination.ValidateCurrent(ctx); err != nil {
			return nil, err
		}
		if err := s.config.SourceHostPermissionGuard(ctx, run, definition, "task.host.emitEvent", request.Call); err != nil {
			return nil, err
		}
		encoded, err := json.Marshal(TaskHostEventContract{Scope: request.Scope, TaskRunID: run.TaskRunID, Generation: run.Generation, AttemptID: run.ExecutionAttemptID.String(), RequestID: request.RequestID, PayloadHash: hashBytes(request.Call.Payload), Target: target, Definition: contract})
		if err != nil || len(encoded) > 64<<10 {
			return nil, NewTaskError(ErrTaskDependencyUnavailable, "原设备固定事件契约响应超过上限")
		}
		return encoded, nil
	}
	return nil, NewTaskError(ErrTaskDependencyUnavailable, "原设备未安装当前事件类型的固定契约")
}

func (b *OwnedRuntimeBinding) SourceTaskEventContract(ctx context.Context, run *TaskRun, definition *TaskDefinition, requestID string, call TaskHostNativeCall) (TaskHostEventContract, error) {
	dependencies, err := b.load()
	if err != nil {
		return TaskHostEventContract{}, err
	}
	port, ok := dependencies.data.(TaskHostSourceEventContractPort)
	if !ok {
		return TaskHostEventContract{}, NewTaskError(ErrTaskDependencyUnavailable, "原设备事件契约授信端口未就绪")
	}
	return port.SourceTaskEventContract(ctx, run, definition, requestID, call)
}
