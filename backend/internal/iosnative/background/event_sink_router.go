package background

import (
	"context"

	"github.com/u-ai/backend/internal/extension/kernel/task_runtime"
	"github.com/u-ai/backend/internal/nativebridge"
)

type taskRuntimeEventSinkRouter struct {
	svc *task_runtime.TaskRuntimeService
}

func NewTaskRuntimeEventSinkRouter(svc *task_runtime.TaskRuntimeService) nativebridge.NativeEventSinkRouter {
	return &taskRuntimeEventSinkRouter{svc: svc}
}

func (r *taskRuntimeEventSinkRouter) ResumeBackgroundTask(ctx context.Context, taskRunID string) error {
	if r.svc == nil {
		return nil
	}
	return r.svc.ResumeTask(ctx, task_runtime.ResumeTaskRequest{TaskRunID: taskRunID})
}

func (r *taskRuntimeEventSinkRouter) SignalBackgroundExpiration(ctx context.Context, taskRunID string) error {
	if r.svc == nil {
		return nil
	}
	return r.svc.Cancel(ctx, taskRunID, "execution_window_expired")
}
