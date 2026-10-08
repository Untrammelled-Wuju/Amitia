package task_runtime

import (
	"context"
	"encoding/json"
	"time"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/extension/kernel/permission"
)

type SourceTaskApprovalRecorder interface {
	RecordApproval(context.Context, permission.PermissionApprovalRecordRequest) (permission.PermissionApprovalRecord, error)
}

type SourceTaskApprovalDetails struct {
	SourceTaskApproval
	Permissions []permission.PermissionRequirement `json:"permissions"`
}

func (s *TaskRuntimeService) DescribeSourceTaskApproval(ctx context.Context, approval SourceTaskApproval) (SourceTaskApprovalDetails, error) {
	if err := s.ValidateSourceTaskApproval(ctx, approval); err != nil {
		return SourceTaskApprovalDetails{}, err
	}
	definition, err := s.store.GetTaskDefinition(ctx, approval.Binding.Target.TaskID)
	if err != nil {
		return SourceTaskApprovalDetails{}, err
	}
	requirements, err := sourceTaskPermissionRequirements(definition)
	if err != nil {
		return SourceTaskApprovalDetails{}, err
	}
	if err := s.ValidateSourceTaskApproval(ctx, approval); err != nil {
		return SourceTaskApprovalDetails{}, err
	}
	return SourceTaskApprovalDetails{SourceTaskApproval: approval, Permissions: requirements}, nil
}

func sourceTaskApprovalExecutionContext(binding SourceTaskApprovalBinding) permission.PermissionExecutionContext {
	target := binding.ExecutionTarget
	return permission.PermissionExecutionContext{Placement: permission.ExecutionPlacementDevice, SpaceID: target.SpaceID, DeviceID: target.DeviceID, RuntimeID: target.RuntimeID, RuntimeSessionID: target.RuntimeSessionID, ProviderID: target.ProviderID.String(), ProviderInstanceID: target.ProviderInstanceID.String(), ExtensionID: binding.Target.ExtensionID, ModuleID: binding.Target.ModuleID, Source: "device-mesh-task"}
}

func (s *TaskRuntimeService) RequestSourceTaskApproval(ctx context.Context, request SourceTaskPermissionRequest) (SourceTaskApproval, error) {
	if s.config.SourceApprovalRecorder == nil || s.sourceApprovals == nil {
		return SourceTaskApproval{}, NewTaskError(ErrTaskDependencyUnavailable, "目标设备没有单次资源审批端口")
	}
	if !json.Valid(request.Input) || len(request.Input) > 512<<10 || request.Run.InputHash != hashBytes(request.Input) || request.Run.TaskDefinitionID != request.Target.TaskID || request.Run.InvocationID != request.Run.TaskRunID || request.Run.ScopeSnapshotID != request.Run.TaskRunID {
		return SourceTaskApproval{}, coordination.ErrWrongOwner
	}
	binding := SourceTaskApprovalBinding{Scope: request.Scope, TaskRunID: request.Run.TaskRunID, TaskGeneration: sourceTaskApprovalGeneration(request.Run.Generation), InputHash: request.Run.InputHash, Target: request.Target, ExecutionTarget: request.Run.ExecutionTarget}
	if err := s.ValidateSourceTaskApproval(ctx, SourceTaskApproval{Binding: binding}); err != nil {
		return SourceTaskApproval{}, err
	}
	definition, err := s.store.GetTaskDefinition(ctx, binding.Target.TaskID)
	if err != nil {
		return SourceTaskApproval{}, err
	}
	requirements, err := sourceTaskPermissionRequirements(definition)
	if err != nil || len(requirements) == 0 {
		return SourceTaskApproval{}, NewTaskError(ErrTaskPermissionDenied, "设备任务没有可批准的资源声明")
	}
	if err := s.validateSourceApprovalEligibility(ctx, binding, definition, requirements); err != nil {
		return SourceTaskApproval{}, err
	}
	return s.sourceApprovals.Request(ctx, binding)
}

func (s *TaskRuntimeService) DecideSourceTaskApproval(ctx context.Context, id string, revision int64, approved bool) (SourceTaskApproval, error) {
	value, err := s.SourceTaskApproval(id)
	if err != nil {
		return SourceTaskApproval{}, err
	}
	if value.Revision != revision || value.Status != "pending" {
		return SourceTaskApproval{}, coordination.ErrRequestConflict
	}
	if err := s.ValidateSourceTaskApproval(ctx, value); err != nil {
		return SourceTaskApproval{}, err
	}
	if !approved {
		return s.sourceApprovals.Decide(ctx, id, revision, value.Binding, false)
	}
	if s.config.SourceApprovalRecorder == nil {
		return SourceTaskApproval{}, NewTaskError(ErrTaskDependencyUnavailable, "目标设备没有实际权限审批记录端口")
	}
	definition, err := s.store.GetTaskDefinition(ctx, value.Binding.Target.TaskID)
	if err != nil {
		return SourceTaskApproval{}, err
	}
	requirements, err := sourceTaskPermissionRequirements(definition)
	if err != nil || len(requirements) == 0 {
		return SourceTaskApproval{}, NewTaskError(ErrTaskPermissionDenied, "设备任务资源声明已失效")
	}
	if err := s.validateSourceApprovalEligibility(ctx, value.Binding, definition, requirements); err != nil {
		return SourceTaskApproval{}, err
	}
	ids := make([]string, 0, len(requirements))
	for _, requirement := range requirements {
		ids = append(ids, requirement.PermissionID)
	}
	execution := sourceTaskApprovalExecutionContext(value.Binding)
	expires := time.Now().UTC().Add(30 * time.Minute)
	record, err := s.config.SourceApprovalRecorder.RecordApproval(ctx, permission.PermissionApprovalRecordRequest{InvocationID: value.Binding.TaskRunID, PermissionIDs: ids, ScopeSnapshotID: value.Binding.TaskRunID, Decision: permission.ApprovalDecisionApproved, ApprovedBy: "local-device:" + value.Binding.Scope.TargetDeviceID, ExpiresAt: &expires, ExecutionContext: execution, ExecutionBindingKey: execution.BindingKey()})
	if err != nil {
		return SourceTaskApproval{}, err
	}
	if record.RecordID == "" || record.Decision != permission.ApprovalDecisionApproved || record.InvocationID != value.Binding.TaskRunID || record.ScopeSnapshotID != value.Binding.TaskRunID || record.ExecutionBindingKey != execution.BindingKey() || record.ExpiresAt == nil || len(record.PermissionIDs) != len(ids) {
		return SourceTaskApproval{}, NewTaskError(ErrTaskPermissionDenied, "目标设备未返回一致的实际权限审批记录")
	}
	for index, id := range ids {
		if record.PermissionIDs[index] != id {
			return SourceTaskApproval{}, NewTaskError(ErrTaskPermissionDenied, "目标设备权限记录与声明不一致")
		}
	}
	if err := s.ValidateSourceTaskApproval(ctx, value); err != nil {
		return SourceTaskApproval{}, err
	}
	decided, err := s.sourceApprovals.Decide(ctx, id, revision, value.Binding, true)
	if err != nil {
		return SourceTaskApproval{}, err
	}
	if err := s.sourceApprovals.SetApprovalRecord(ctx, id, decided.Revision, value.Binding, record.RecordID, *record.ExpiresAt); err != nil {
		s.sourceApprovals.Revoke(id)
		return SourceTaskApproval{}, err
	}
	if err := coordination.ValidateCurrent(ctx); err != nil {
		s.sourceApprovals.Revoke(id)
		return SourceTaskApproval{}, err
	}
	return decided, nil
}

func (s *TaskRuntimeService) ListSourceTaskApprovals() []SourceTaskApproval {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.closed || s.sourceApprovals == nil {
		return []SourceTaskApproval{}
	}
	return s.sourceApprovals.List()
}

func (s *TaskRuntimeService) RevokeSourceTaskApproval(ctx context.Context, id string, revision int64) (SourceTaskApproval, error) {
	value, err := s.SourceTaskApproval(id)
	if err != nil {
		return SourceTaskApproval{}, err
	}
	if err := s.ValidateSourceTaskApproval(ctx, value); err != nil {
		return SourceTaskApproval{}, err
	}
	revoked, err := s.sourceApprovals.RevokeCurrent(ctx, id, revision, value.Binding)
	if err != nil {
		return SourceTaskApproval{}, err
	}
	return revoked, coordination.ValidateCurrent(ctx)
}

func (s *TaskRuntimeService) SourceTaskApproval(id string) (SourceTaskApproval, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.closed || s.sourceApprovals == nil {
		return SourceTaskApproval{}, NewTaskError(ErrTaskDependencyUnavailable, "设备任务审批服务不可用")
	}
	approval, found := s.sourceApprovals.Get(id)
	if !found {
		return SourceTaskApproval{}, NewTaskError(ErrTaskPermissionDenied, "设备任务审批不存在或已经到期")
	}
	return approval, nil
}

func (s *TaskRuntimeService) ValidateSourceTaskApproval(ctx context.Context, approval SourceTaskApproval) error {
	if err := validateSourceTaskApprovalBinding(ctx, approval.Binding); err != nil {
		return err
	}
	actual, err := s.DescribeInstalledTask(ctx, approval.Binding.Target.TaskID, approval.Binding.Scope.TargetDeviceID)
	if err != nil {
		return err
	}
	if actual != approval.Binding.Target {
		return NewTaskError(ErrTaskDefinitionInvalid, "审批对应的设备任务安装版本已经变化")
	}
	return coordination.ValidateCurrent(ctx)
}

func (s *TaskRuntimeService) validateSourceApprovalEligibility(ctx context.Context, binding SourceTaskApprovalBinding, definition *TaskDefinition, requirements []permission.PermissionRequirement) error {
	policy, ok := s.config.SourceApprovalRecorder.(interface {
		CanApproveRemoteTask(context.Context, permission.PermissionEvaluationRequest) bool
	})
	request := permission.PermissionEvaluationRequest{Subject: permission.PermissionSubject{Type: permission.SubjectModule, ID: definition.ModuleID, ExtensionID: definition.ExtensionID, ModuleID: definition.ModuleID}, Requirements: requirements, InvocationID: binding.TaskRunID, ScopeSnapshotID: binding.TaskRunID, Target: permission.PermissionTarget{Type: "task", ID: definition.TaskID}, IsBackground: true, Generation: definition.InstalledGeneration, ExecutionContext: sourceTaskApprovalExecutionContext(binding)}
	if !ok || !policy.CanApproveRemoteTask(ctx, request) {
		return NewTaskError(ErrTaskPermissionDenied, "目标设备资源策略禁止批准此任务")
	}
	return nil
}
