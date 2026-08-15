package task_runtime

import (
	"context"
	"time"
)

type TaskStore interface {
	PutTaskDefinition(ctx context.Context, def *TaskDefinition) error

	DeleteTaskDefinition(ctx context.Context, defID string) error
	DeleteByExtension(ctx context.Context, extensionID string) error
	GetTaskDefinition(ctx context.Context, defID string) (*TaskDefinition, error)
	ListTaskDefinitions(ctx context.Context, extensionID string) ([]*TaskDefinition, error)

	PutTaskRun(ctx context.Context, run *TaskRun) error
	UpdateTaskRunCAS(ctx context.Context, run *TaskRun, expectedStatus TaskRunStatus, expectedGeneration int64) (bool, error)
	GetTaskRun(ctx context.Context, runID string) (*TaskRun, error)
	ListTaskRuns(ctx context.Context, filter ListTasksFilter) ([]*TaskRun, error)
	ListTaskRunsByStatus(ctx context.Context, status string) ([]*TaskRun, error)

	UpdateExecutionTarget(
		ctx context.Context,
		taskRunID string,
		placement TaskExecutionPlacement,
		target TaskExecutionTarget,
		resolvedAt time.Time,
		resolvedBy string,
	) error

	UpdateExecutionConnectionBinding(
		ctx context.Context,
		taskRunID string,
		runtimeSessionID interface{ String() string },
		generation int64,
		at time.Time,
	) error

	UpdateExecutionAttempt(
		ctx context.Context,
		taskRunID string,
		attemptID TaskExecutionAttemptID,
		runtimeInstanceID string,
		at time.Time,
	) error

	EnqueueTask(ctx context.Context, entry *TaskQueueEntry) error
	DequeueTask(ctx context.Context, leaseOwner string, leaseDuration time.Duration) (*TaskQueueEntry, error)
	RemoveFromQueue(ctx context.Context, taskRunID string) error
	ReclaimExpiredLeases(ctx context.Context) (int, error)
	GetQueueEntry(ctx context.Context, taskRunID string) (*TaskQueueEntry, error)

	PutCheckpoint(ctx context.Context, cp *TaskCheckpoint) error
	GetLatestCheckpoint(ctx context.Context, taskRunID string) (*TaskCheckpoint, error)

	PutProgress(ctx context.Context, taskRunID string, seq int64, progressJSON []byte) error
	GetProgress(ctx context.Context, taskRunID string) (*TaskRunProgress, error)

	PutResult(ctx context.Context, result *TaskRunResult) error
	GetResult(ctx context.Context, taskRunID string) (*TaskRunResult, error)

	CountActive(ctx context.Context) (int, error)
	CountActiveByExtension(ctx context.Context, extensionID string) (int, error)
	CountActiveByDefinition(ctx context.Context, defID string) (int, error)

	WithinTaskTx(ctx context.Context, fn func(ctx context.Context) error) error
}
