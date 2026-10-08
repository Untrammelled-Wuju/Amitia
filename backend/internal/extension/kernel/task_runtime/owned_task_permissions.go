package task_runtime

import (
	"context"
	"encoding/json"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

type TargetTaskPermissionProvider interface {
	TargetTaskPermissions(context.Context, SourceTaskPermissionRequest) error
}

type OwnedTaskPermissionPort interface {
	Prepare(context.Context, *TaskRun, *TaskDefinition, TargetTaskDefinitionPin, json.RawMessage) error
}

type AcknowledgedTaskPermissionPort struct {
	Provider TargetTaskPermissionProvider
}

func (p AcknowledgedTaskPermissionPort) Prepare(ctx context.Context, run *TaskRun, definition *TaskDefinition, pin TargetTaskDefinitionPin, input json.RawMessage) error {
	if definition == nil || run == nil {
		return NewTaskError(ErrTaskPermissionDenied, "设备任务权限准备缺少原任务身份")
	}
	if len(definition.PermissionRequirements)+len(definition.PermissionRequirementStrings) == 0 {
		return nil
	}
	scope, owned := coordination.FromContext(ctx)
	if !owned || p.Provider == nil || !json.Valid(input) || len(input) > 512<<10 || run.InputHash != hashBytes(input) || run.Generation < 1 || run.InvocationID != run.TaskRunID || run.ScopeSnapshotID != run.TaskRunID {
		return NewTaskError(ErrTaskPermissionDenied, "设备任务资源审批缺少完整所有者身份")
	}
	if err := ValidateTargetTaskDefinition(scope.TargetDeviceID, definition, pin); err != nil {
		return err
	}
	requestRun := CloneTaskRun(run)
	requestRun.Input = nil
	requestRun.TaskDefinitionID = SourceTaskDefinitionID(definition)
	if err := p.Provider.TargetTaskPermissions(ctx, SourceTaskPermissionRequest{Scope: scope, Run: *requestRun, Target: pin, Input: append(json.RawMessage(nil), input...)}); err != nil {
		return err
	}
	return coordination.ValidateCurrent(ctx)
}

func (s *TaskRuntimeService) prepareOwnedTargetPermissions(ctx context.Context, run *TaskRun, definition *TaskDefinition, pin TargetTaskDefinitionPin, input json.RawMessage) error {
	if definition == nil || len(definition.PermissionRequirements)+len(definition.PermissionRequirementStrings) == 0 {
		return nil
	}
	if s.config.OwnedTargetPermissions == nil {
		return NewTaskError(ErrTaskPermissionDenied, "目标设备资源权限准备端口不可用")
	}
	return s.config.OwnedTargetPermissions.Prepare(ctx, run, definition, pin, input)
}
