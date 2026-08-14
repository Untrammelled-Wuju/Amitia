package task_runtime

import (
	"context"
)

type LocalTaskExecutor struct {
	service *TaskRuntimeService
}

func NewLocalTaskExecutor(service *TaskRuntimeService) *LocalTaskExecutor {
	return &LocalTaskExecutor{service: service}
}

func (e *LocalTaskExecutor) Kind() TaskExecutorKind {
	return TaskExecutorKindLocal
}

func (e *LocalTaskExecutor) SupportsPlacement(placement TaskExecutionPlacement) bool {
	return placement == TaskExecutionPlacementLocal
}

func (e *LocalTaskExecutor) Execute(
	ctx context.Context,
	request TaskExecutionRequest,
) (TaskExecutionOutcome, error) {
	run := request.Run
	if err := e.service.persistExecutionAttempt(ctx, run, request.AttemptID, ""); err != nil {
		return TaskExecutionOutcome{
			Status:       RunStatusFailed,
			ErrorCode:    string(ErrTaskExecutionAttemptInvalid),
			ErrorMessage: err.Error(),
		}, err
	}
	return TaskExecutionOutcome{
		Status: run.Status,
	}, nil
}
