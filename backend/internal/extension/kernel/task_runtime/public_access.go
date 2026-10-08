package task_runtime

import (
	"context"

	"github.com/u-ai/backend/internal/auth"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

type PublicTaskRequestGuard func(context.Context, *coordination.ExecutionScope) (context.Context, func(), error)

func (s *TaskRuntimeService) withManagementTaskTx(ctx context.Context, operation func(context.Context) error) error {
	return coordination.CommitRequestCurrent(ctx, func() error { return s.store.WithinTaskTx(ctx, operation) })
}

func (s *TaskRuntimeService) OpenPublicTaskRequest(ctx context.Context, taskID string) (context.Context, func(), error) {
	return s.openPublicTaskRequest(ctx, taskID, false)
}

func (s *TaskRuntimeService) OpenPublicTaskRead(ctx context.Context, taskID string) (context.Context, func(), error) {
	return s.openPublicTaskRequest(ctx, taskID, true)
}

func (s *TaskRuntimeService) openPublicTaskRequest(ctx context.Context, taskID string, readOnly bool) (context.Context, func(), error) {
	actor, ok := auth.FromContext(ctx)
	if !ok || actor == nil || actor.SpaceID == "" {
		return ctx, nil, NewTaskError(ErrTaskScopeDenied, "任务管理缺少已认证设备身份")
	}
	if s.config.PublicRequestGuard == nil {
		if actor.PrincipalType == auth.PrincipalLocalUI && actor.HasPermission(auth.PermSystemAdmin) {
			return ctx, func() {}, nil
		}
		return ctx, nil, NewTaskError(ErrTaskScopeDenied, "任务管理授权端口尚未就绪")
	}
	current, finish, err := s.config.PublicRequestGuard(ctx, nil)
	if err != nil {
		if finish != nil {
			finish()
		}
		return ctx, nil, err
	}
	if current == nil || finish == nil {
		if finish != nil {
			finish()
		}
		return ctx, nil, NewTaskError(ErrTaskScopeDenied, "任务管理未建立有效授权上下文")
	}
	if err := coordination.ValidateCurrent(current); err != nil {
		finish()
		return ctx, nil, err
	}
	if taskID == "" {
		return current, finish, nil
	}
	run, err := s.store.GetTaskRun(current, taskID)
	if err != nil || run == nil {
		finish()
		return ctx, nil, NewTaskError(ErrTaskNotFound, "任务不存在")
	}
	authority, owned, err := s.taskAuthoritySnapshot(current, run.ScopeSnapshotID, run.InvocationID, run.ExtensionID, run.ModuleID, readOnly)
	if err != nil {
		finish()
		return ctx, nil, err
	}
	if !owned {
		if !actor.HasPermission(auth.PermSystemAdmin) {
			finish()
			return ctx, nil, NewTaskError(ErrTaskScopeDenied, "普通设备不能读取或控制 Core 全局任务")
		}
		return current, finish, nil
	}
	if authority.SpaceID != actor.SpaceID.String() || !actor.HasPermission(auth.PermSystemAdmin) && authority.InitiatorDeviceID != actor.DeviceID.String() && authority.TargetDeviceID != actor.DeviceID.String() {
		finish()
		return ctx, nil, NewTaskError(ErrTaskScopeDenied, "当前设备没有此任务的读取或控制权限")
	}
	var guarded context.Context
	var taskFinish func()
	if readOnly {
		if s.config.OwnedReadGuard == nil {
			finish()
			return ctx, nil, NewTaskError(ErrTaskScopeDenied, "任务历史只读授权端口尚未就绪")
		}
		guarded, taskFinish, err = s.config.OwnedReadGuard(current, authority, run)
		if err == nil && guarded != nil {
			actual, inherited := coordination.FromContext(guarded)
			_, proof, historical := coordination.TaskReadAuthority(guarded)
			if taskFinish == nil || !inherited || actual != authority || !historical || proof.Scope != authority || proof.TaskRunID != run.TaskRunID || proof.DefinitionFingerprint != run.DefinitionFingerprint {
				err = NewTaskError(ErrTaskScopeDenied, "任务历史读取未保留原始归属和只读约束")
			}
		}
		if err == nil && guarded == nil {
			err = NewTaskError(ErrTaskScopeDenied, "任务历史读取缺少授权上下文")
		}
	} else {
		guarded, taskFinish, err = s.restoreTaskAuthority(current, run)
	}
	if err == nil {
		err = coordination.ValidateCurrent(guarded)
	}
	if err != nil {
		if taskFinish != nil {
			taskFinish()
		}
		finish()
		return ctx, nil, err
	}
	return guarded, func() { taskFinish(); finish() }, nil
}
