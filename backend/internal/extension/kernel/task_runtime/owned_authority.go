package task_runtime

import (
	"context"
	"encoding/json"
	"time"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/extension/kernel/scope"
)

type TaskAuthoritySnapshotStore interface {
	GetSnapshot(context.Context, string) (scope.ScopeSnapshot, error)
}

type OwnedTaskExecutionGuard func(context.Context, coordination.ExecutionScope, *TaskRun) (context.Context, func(), error)

func (s *TaskRuntimeService) validateTaskReadScope(ctx context.Context, run *TaskRun) error {
	current, inherited := coordination.FromContext(ctx)
	if !inherited {
		return nil
	}
	if run == nil {
		return NewTaskError(ErrTaskNotFound, "任务不存在")
	}
	_, _, historical := coordination.TaskReadAuthority(ctx)
	authority, owned, err := s.taskAuthoritySnapshot(ctx, run.ScopeSnapshotID, run.InvocationID, run.ExtensionID, run.ModuleID, historical)
	if err != nil {
		return err
	}
	if !owned || authority != current {
		return NewTaskError(ErrTaskScopeDenied, "读取范围与任务持久化授权不一致")
	}
	return coordination.ValidateCurrent(ctx)
}

func (s *TaskRuntimeService) taskAuthority(ctx context.Context, snapshotID, invocationID, extensionID, moduleID string) (coordination.ExecutionScope, bool, error) {
	return s.taskAuthoritySnapshot(ctx, snapshotID, invocationID, extensionID, moduleID, false)
}

func (s *TaskRuntimeService) taskAuthoritySnapshot(ctx context.Context, snapshotID, invocationID, extensionID, moduleID string, historical bool) (coordination.ExecutionScope, bool, error) {
	if snapshotID == "" {
		return coordination.ExecutionScope{}, false, nil
	}
	if s.config.AuthoritySnapshots == nil {
		return coordination.ExecutionScope{}, false, NewTaskError(ErrTaskScopeDenied, "任务授权快照存储不可用")
	}
	snapshot, err := s.config.AuthoritySnapshots.GetSnapshot(ctx, snapshotID)
	if err != nil {
		return coordination.ExecutionScope{}, false, WrapTaskError(ErrTaskScopeDenied, "任务授权快照不存在", err)
	}
	if snapshot.SnapshotID != snapshotID || snapshot.InvocationID != invocationID || snapshot.ExtensionID != extensionID || snapshot.ModuleID != moduleID || (!historical && snapshot.ExpiresAt != nil && !snapshot.ExpiresAt.After(time.Now())) {
		return coordination.ExecutionScope{}, false, NewTaskError(ErrTaskScopeDenied, "任务授权快照已过期或与调用不一致")
	}
	if len(snapshot.OwnedExecutionScope) == 0 {
		return coordination.ExecutionScope{}, false, nil
	}
	var authority coordination.ExecutionScope
	if len(snapshot.OwnedExecutionScope) > 64<<10 || json.Unmarshal(snapshot.OwnedExecutionScope, &authority) != nil || authority.SpaceID != snapshot.SpaceID || authority.RoleID != snapshot.CharacterID || authority.CoreID == "" || authority.AuthorizationRealm != authority.CoreID || authority.InitiatorDeviceID == "" || authority.TargetDeviceID == "" || authority.ResourceOwnerID == "" || authority.RoleOwnerID != authority.ResourceOwnerID || authority.RequestID == "" || authority.ExecutionID == "" || authority.TurnID == "" || authority.RoleRevision <= 0 || authority.ProviderEpoch <= 0 || authority.TargetProviderEpoch <= 0 || authority.PermissionRevision <= 0 || authority.TargetPermissionRevision <= 0 || authority.ModeRevision <= 0 {
		return coordination.ExecutionScope{}, false, NewTaskError(ErrTaskScopeDenied, "任务设备授权范围不完整")
	}
	return authority, true, nil
}

func (s *TaskRuntimeService) validateEnqueueAuthority(ctx context.Context, req EnqueueTaskRequest, def *TaskDefinition) error {
	authority, owned, err := s.taskAuthority(ctx, req.ScopeSnapshotID, req.InvocationID, def.ExtensionID, def.ModuleID)
	if err != nil {
		return err
	}
	current, currentOwned := coordination.FromContext(ctx)
	if def.RemoteSource != nil {
		if !owned || req.TrustedExecutionTarget == nil || req.TrustedExecutionTarget.Target.SourceTaskDefinitionID != SourceTaskDefinitionID(def) {
			return NewTaskError(ErrTaskScopeDenied, "设备任务目录引用缺少固定目标与持久化执行授权")
		}
		if err := validateDeviceTaskSource(authority, def); err != nil {
			return err
		}
	}
	if currentOwned != owned || (owned && (current != authority || s.config.OwnedExecutionGuard == nil)) {
		return NewTaskError(ErrTaskScopeDenied, "任务缺少一致的持久化设备授权或数据归属执行器")
	}
	return coordination.ValidateCurrent(ctx)
}

func (s *TaskRuntimeService) restoreTaskAuthority(ctx context.Context, run *TaskRun) (context.Context, func(), error) {
	if _, _, readOnly := coordination.TaskReadAuthority(ctx); readOnly {
		return ctx, nil, NewTaskError(ErrTaskScopeDenied, "历史只读授权不能恢复执行或控制任务")
	}
	authority, owned, err := s.taskAuthority(ctx, run.ScopeSnapshotID, run.InvocationID, run.ExtensionID, run.ModuleID)
	if err != nil {
		return ctx, nil, err
	}
	if !owned {
		if _, inherited := coordination.FromContext(ctx); inherited {
			return ctx, nil, NewTaskError(ErrTaskScopeDenied, "任务授权范围不能降级为普通执行")
		}
		return ctx, func() {}, nil
	}
	if s.config.OwnedExecutionGuard == nil {
		return ctx, nil, NewTaskError(ErrTaskScopeDenied, "任务设备授权和数据归属执行器不可用")
	}
	guarded, finish, err := s.config.OwnedExecutionGuard(ctx, authority, run)
	if err != nil {
		if finish != nil {
			finish()
		}
		return ctx, nil, WrapTaskError(ErrTaskScopeDenied, "任务设备授权恢复失败", err)
	}
	if guarded == nil {
		if finish != nil {
			finish()
		}
		return ctx, nil, NewTaskError(ErrTaskScopeDenied, "任务执行器未恢复授权上下文")
	}
	restored, ok := coordination.FromContext(guarded)
	if finish == nil || !ok || restored != authority {
		if finish != nil {
			finish()
		}
		return ctx, nil, NewTaskError(ErrTaskScopeDenied, "任务执行器改变了持久化授权范围")
	}
	if err := coordination.ValidateCurrent(guarded); err != nil {
		finish()
		return ctx, nil, WrapTaskError(ErrTaskScopeDenied, "任务设备授权已失效", err)
	}
	return guarded, finish, nil
}
