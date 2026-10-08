package task_runtime

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/extension/kernel/permission"
)

type SourceTaskPermissionGuard func(context.Context, *TaskRun, *TaskDefinition, json.RawMessage) error

type TaskPermissionEvaluator interface {
	Evaluate(context.Context, permission.PermissionEvaluationRequest) permission.PermissionEvaluationResult
}

type SourceTaskPermissionRequest struct {
	Scope  coordination.ExecutionScope `json:"executionScope"`
	Run    TaskRun                     `json:"task"`
	Target TargetTaskDefinitionPin     `json:"target"`
	Input  json.RawMessage             `json:"input"`
}

type SourceTaskPermissionAcknowledgement struct {
	Scope           coordination.ExecutionScope `json:"executionScope"`
	TaskRunID       string                      `json:"taskRunId"`
	TaskGeneration  int64                       `json:"taskGeneration"`
	ExecutionTarget TaskExecutionTarget         `json:"executionTarget"`
	InputHash       string                      `json:"inputHash"`
	Target          TargetTaskDefinitionPin     `json:"target"`
	Allowed         bool                        `json:"allowed"`
	ApprovalID      string                      `json:"approvalId,omitempty"`
	ApprovalStatus  string                      `json:"approvalStatus,omitempty"`
}

func (s *TaskRuntimeService) CheckInstalledTaskPermissions(ctx context.Context, request SourceTaskPermissionRequest) (SourceTaskPermissionAcknowledgement, error) {
	ack := SourceTaskPermissionAcknowledgement{}
	authority, owned := coordination.FromContext(ctx)
	if !owned || authority != request.Scope || len(request.Run.Input) != 0 && string(request.Run.Input) != "null" || request.Run.TaskRunID == "" || len(request.Run.TaskRunID) > 256 || request.Run.InvocationID != request.Run.TaskRunID || request.Run.ScopeSnapshotID != request.Run.TaskRunID || request.Run.InputHash != hashBytes(request.Input) || !json.Valid(request.Input) || len(request.Input) > 512<<10 || request.Target.TaskID != request.Run.TaskDefinitionID || request.Target.DeviceID != authority.TargetDeviceID || request.Target.ExtensionID != request.Run.ExtensionID || request.Target.ModuleID != request.Run.ModuleID || request.Run.ExecutionTarget.SpaceID.String() != authority.CoreID || request.Run.ExecutionTarget.DeviceID.String() != authority.TargetDeviceID || request.Run.ExecutionTarget.RuntimeID == "" || request.Run.ExecutionTarget.RuntimeSessionID == "" || request.Run.ExecutionTarget.ConnectionGeneration < 1 {
		return ack, NewTaskError(ErrTaskPermissionDenied, "设备任务资源预授权身份不一致")
	}
	actual, err := s.DescribeInstalledTask(ctx, request.Run.TaskDefinitionID, authority.TargetDeviceID)
	if err != nil || actual != request.Target {
		return ack, NewTaskError(ErrTaskDefinitionInvalid, "设备任务在权限准备期间已更新")
	}
	definition, err := s.store.GetTaskDefinition(ctx, request.Run.TaskDefinitionID)
	if err != nil {
		return ack, err
	}
	allowed := true
	var approval SourceTaskApproval
	binding := SourceTaskApprovalBinding{Scope: authority, TaskRunID: request.Run.TaskRunID, TaskGeneration: sourceTaskApprovalGeneration(request.Run.Generation), InputHash: request.Run.InputHash, Target: actual, ExecutionTarget: request.Run.ExecutionTarget}
	if s.sourceApprovals != nil {
		for _, value := range s.sourceApprovals.List() {
			if value.Binding == binding {
				approval = value
				if value.Status == "approved" {
					ctx = context.WithValue(ctx, sourceTaskApprovalProofKey{}, sourceTaskApprovalProof{ledger: s.sourceApprovals, id: value.ID, binding: binding})
				}
				break
			}
		}
	}
	if err := s.validateSourceTaskPermissions(ctx, &request.Run, definition, request.Input); err != nil {
		if !IsTaskErrorCode(err, ErrTaskPermissionDenied) {
			return ack, err
		}
		allowed = false
		if approval.ID == "" {
			approval, err = s.RequestSourceTaskApproval(ctx, request)
			if err != nil && !IsTaskErrorCode(err, ErrTaskPermissionDenied) && !IsTaskErrorCode(err, ErrTaskDependencyUnavailable) {
				return ack, err
			}
		}
	}
	current, err := s.DescribeInstalledTask(ctx, request.Run.TaskDefinitionID, authority.TargetDeviceID)
	if err != nil || current != actual {
		return ack, NewTaskError(ErrTaskDefinitionInvalid, "设备插件权限准备期间安装版本已变化")
	}
	return SourceTaskPermissionAcknowledgement{Scope: authority, TaskRunID: request.Run.TaskRunID, TaskGeneration: sourceTaskApprovalGeneration(request.Run.Generation), ExecutionTarget: request.Run.ExecutionTarget, InputHash: request.Run.InputHash, Target: actual, Allowed: allowed, ApprovalID: approval.ID, ApprovalStatus: approval.Status}, nil
}

func ValidateSourceTaskPermissionAcknowledgement(request SourceTaskPermissionRequest, ack SourceTaskPermissionAcknowledgement) error {
	if ack.Scope != request.Scope || ack.TaskRunID != request.Run.TaskRunID || ack.TaskGeneration != sourceTaskApprovalGeneration(request.Run.Generation) || ack.ExecutionTarget != request.Run.ExecutionTarget || ack.InputHash != request.Run.InputHash || ack.Target != request.Target {
		return NewTaskError(ErrTaskPermissionDenied, "目标设备资源权限确认与当前执行不一致")
	}
	if !ack.Allowed {
		return NewTaskError(ErrTaskPermissionDenied, "请在目标设备批准当前插件的资源权限后重试")
	}
	return nil
}

func sourceTaskPermissionRequirements(definition *TaskDefinition) ([]permission.PermissionRequirement, error) {
	if definition == nil || len(definition.PermissionRequirements)+len(definition.PermissionRequirementStrings) > 64 {
		return nil, NewTaskError(ErrTaskPermissionDenied, "设备任务权限声明无效或超过上限")
	}
	requirements := append([]permission.PermissionRequirement(nil), definition.PermissionRequirements...)
	for _, id := range definition.PermissionRequirementStrings {
		requirements = append(requirements, permission.PermissionRequirement{PermissionID: id, Scope: permission.ScopeForExtension(definition.ExtensionID)})
	}
	for index := range requirements {
		item := &requirements[index]
		if item.PermissionID == "" || len(item.PermissionID) > 256 || strings.TrimSpace(item.PermissionID) != item.PermissionID || len(item.Conditions) > 4096 || len(item.Conditions) != 0 && !json.Valid(item.Conditions) {
			return nil, NewTaskError(ErrTaskPermissionDenied, "设备任务权限声明无效")
		}
		if !item.Scope.IsValid() {
			item.Scope = permission.ScopeForExtension(definition.ExtensionID)
		}
	}
	encoded, err := json.Marshal(requirements)
	if err != nil || len(encoded) > 64<<10 {
		return nil, NewTaskError(ErrTaskPermissionDenied, "设备任务权限声明超过上限")
	}
	return requirements, nil
}

func NewSourceTaskPermissionGuard(evaluator TaskPermissionEvaluator) SourceTaskPermissionGuard {
	return func(ctx context.Context, run *TaskRun, definition *TaskDefinition, input json.RawMessage) error {
		requirements, err := sourceTaskPermissionRequirements(definition)
		if err != nil || len(requirements) == 0 {
			return err
		}
		authority, owned := coordination.FromContext(ctx)
		if evaluator == nil || !owned || run == nil || definition.InstalledGeneration < 1 || run.TaskRunID == "" || run.InvocationID == "" || run.ScopeSnapshotID == "" || run.TaskDefinitionID != definition.TaskID || run.ExtensionID != definition.ExtensionID || run.ModuleID != definition.ModuleID || run.InputHash != hashBytes(input) || !json.Valid(input) || len(input) > 1<<20 || run.ExecutionTarget.SpaceID.String() != authority.CoreID || run.ExecutionTarget.DeviceID.String() != authority.TargetDeviceID || run.ExecutionTarget.RuntimeID == "" || run.ExecutionTarget.RuntimeSessionID == "" || run.ExecutionTarget.ConnectionGeneration < 1 {
			return NewTaskError(ErrTaskPermissionDenied, "目标设备缺少一致的插件资源授权身份")
		}
		recordID, err := sourceTaskPermissionRecord(ctx, run, definition)
		if err != nil {
			return err
		}
		if policy, ok := evaluator.(interface{ RequiresRemoteApproval(string) bool }); ok && recordID == "" {
			for _, requirement := range requirements {
				if policy.RequiresRemoteApproval(requirement.PermissionID) {
					return NewTaskError(ErrTaskPermissionDenied, "目标设备权限需要独立审批，当前任务尚未绑定本次审批")
				}
			}
		}
		target := run.ExecutionTarget
		request := permission.PermissionEvaluationRequest{
			ApprovalRecordID: recordID,
			Subject:          permission.PermissionSubject{Type: permission.SubjectModule, ID: definition.ModuleID, ExtensionID: definition.ExtensionID, ModuleID: definition.ModuleID},
			Requirements:     requirements,
			InvocationID:     run.InvocationID,
			Input:            append(json.RawMessage(nil), input...),
			Target:           permission.PermissionTarget{Type: "task", ID: definition.TaskID},
			IsBackground:     true,
			ScopeSnapshotID:  run.ScopeSnapshotID,
			Generation:       definition.InstalledGeneration,
			ExecutionContext: permission.PermissionExecutionContext{Placement: permission.ExecutionPlacementDevice, SpaceID: target.SpaceID, DeviceID: target.DeviceID, RuntimeID: target.RuntimeID, RuntimeSessionID: target.RuntimeSessionID, ProviderID: string(target.ProviderID), ProviderInstanceID: string(target.ProviderInstanceID), ExtensionID: definition.ExtensionID, ModuleID: definition.ModuleID, Source: "device-mesh-task"},
		}
		decision := evaluator.Evaluate(ctx, request)
		if decision.Decision != permission.DecisionAllow && decision.Decision != permission.DecisionAllowPersistent && decision.Decision != permission.DecisionAllowSession {
			return NewTaskError(ErrTaskPermissionDenied, "目标设备尚未批准当前任务的插件资源权限")
		}
		for _, grant := range decision.MatchedGrants {
			if grant.IsOneTime() || !grant.IsValid() {
				return NewTaskError(ErrTaskPermissionDenied, "设备任务的一次性资源权限尚未绑定本次执行")
			}
		}
		return nil
	}
}

func (s *TaskRuntimeService) validateSourceTaskPermissions(ctx context.Context, run *TaskRun, definition *TaskDefinition, input json.RawMessage) error {
	requirements, err := sourceTaskPermissionRequirements(definition)
	if err != nil || len(requirements) == 0 {
		return err
	}
	if s.config.SourcePermissionGuard == nil {
		return NewTaskError(ErrTaskPermissionDenied, "设备缺少插件资源权限检查端口")
	}
	if run != nil && run.ExecutionTarget.SourceTaskDefinitionID != "" {
		copy := CloneTaskRun(run)
		copy.TaskDefinitionID = run.ExecutionTarget.SourceTaskDefinitionID
		run = copy
	}
	return s.config.SourcePermissionGuard(ctx, run, definition, input)
}
