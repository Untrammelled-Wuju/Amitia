package task_runtime

import (
	"context"
)

type UnavailableRemoteTaskExecutor struct{}

func (UnavailableRemoteTaskExecutor) Kind() TaskExecutorKind {
	return TaskExecutorKindRemote
}

func (UnavailableRemoteTaskExecutor) SupportsPlacement(placement TaskExecutionPlacement) bool {
	return placement == TaskExecutionPlacementCloud || placement == TaskExecutionPlacementDevice
}

func (UnavailableRemoteTaskExecutor) Execute(
	ctx context.Context,
	request TaskExecutionRequest,
) (TaskExecutionOutcome, error) {
	return TaskExecutionOutcome{
		Status:       RunStatusRecoveryRequired,
		ErrorCode:    string(ErrRemoteTaskExecutorUnavailable),
		ErrorMessage: "remote task executor is not available",
	}, NewTaskError(ErrRemoteTaskExecutorUnavailable, "remote task executor is not available")
}

func (UnavailableRemoteTaskExecutor) ValidateTarget(
	ctx context.Context,
	target TaskExecutionTarget,
) error {
	return NewTaskError(ErrRemoteTaskExecutorUnavailable, "remote task executor is not available")
}

func (UnavailableRemoteTaskExecutor) Cancel(
	ctx context.Context,
	run *TaskRun,
) error {
	return NewTaskError(ErrRemoteTaskExecutorUnavailable, "remote task executor is not available")
}
