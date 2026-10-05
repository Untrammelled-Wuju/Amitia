package task_runtime

import (
	"context"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

func (s *TaskRuntimeService) callbackAuthority(ctx context.Context, run *TaskRun, attemptID string, generation int64) (context.Context, func(), bool, error) {
	if run == nil {
		return ctx, nil, false, NewTaskError(ErrTaskNotFound, "任务不存在")
	}
	authority, owned, err := s.taskAuthority(ctx, run.ScopeSnapshotID, run.InvocationID, run.ExtensionID, run.ModuleID)
	if err != nil {
		return ctx, nil, false, err
	}
	current, inherited := coordination.FromContext(ctx)
	if inherited && (!owned || current != authority) {
		return ctx, nil, false, NewTaskError(ErrTaskScopeDenied, "回调授权与任务持久化授权不一致")
	}
	if !owned {
		return ctx, func() {}, false, nil
	}
	if attemptID == "" || attemptID != run.ExecutionAttemptID.String() || generation < 1 || generation != run.Generation {
		return ctx, nil, true, NewTaskError(ErrTaskExecutionAttemptInvalid, "回调缺少有效的任务执行身份")
	}
	guarded, finish, err := s.restoreTaskAuthority(ctx, run)
	if err != nil {
		return ctx, nil, true, err
	}
	def, err := s.store.GetTaskDefinition(guarded, run.TaskDefinitionID)
	if err == nil {
		err = validateTaskDefinition(true, run, def)
	}
	if err != nil {
		finish()
		return ctx, nil, true, err
	}
	guarded = s.guardOwnedTaskDefinition(guarded, run)
	if err := coordination.ValidateCurrent(guarded); err != nil {
		finish()
		return ctx, nil, true, err
	}
	return guarded, finish, true, nil
}
