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
	UpdateTaskRunCAS(ctx context.Context, run *TaskRun, expectedStatus TaskRunStatus, expectedGeneration int64, expectedRevision int64) (bool, error)
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
		nextRevision int64,
		expectedRevision int64,
	) error

	UpdateExecutionConnectionBinding(
		ctx context.Context,
		taskRunID string,
		runtimeSessionID interface{ String() string },
		generation int64,
		at time.Time,
		nextRevision int64,
		expectedRevision int64,
	) error

	UpdateExecutionAttempt(
		ctx context.Context,
		taskRunID string,
		attemptID TaskExecutionAttemptID,
		runtimeInstanceID string,
		at time.Time,
		nextRevision int64,
		expectedRevision int64,
	) error

	EnqueueTask(ctx context.Context, entry *TaskQueueEntry) error
	DequeueTask(ctx context.Context, leaseOwner string, leaseDuration time.Duration) (*TaskQueueEntry, error)
	RemoveFromQueue(ctx context.Context, taskRunID string) error
	ReclaimExpiredLeases(ctx context.Context) (int, error)
	GetQueueEntry(ctx context.Context, taskRunID string) (*TaskQueueEntry, error)

	PutCheckpoint(ctx context.Context, cp *TaskCheckpoint) error
	GetLatestCheckpoint(ctx context.Context, taskRunID string) (*TaskCheckpoint, error)
	RemoveCheckpoint(ctx context.Context, checkpointID string) error

	PutProgress(ctx context.Context, taskRunID string, seq int64, progressJSON []byte) error
	GetProgress(ctx context.Context, taskRunID string) (*TaskRunProgress, error)

	PutResult(ctx context.Context, result *TaskRunResult) error
	GetResult(ctx context.Context, taskRunID string) (*TaskRunResult, error)

	CountActive(ctx context.Context) (int, error)
	CountActiveByExtension(ctx context.Context, extensionID string) (int, error)
	CountActiveByDefinition(ctx context.Context, defID string) (int, error)

	WithinTaskTx(ctx context.Context, fn func(ctx context.Context) error) error
}

func CloneTaskRun(run *TaskRun) *TaskRun {
	return cloneTaskRun(run)
}

func PrepareTaskMutation(run *TaskRun, nextRevision int64) *TaskRun {
	clone := cloneTaskRun(run)
	clone.Revision = nextRevision
	return clone
}

func CopyCommittedTaskRun(dst *TaskRun, src *TaskRun) {
	dst.Revision = src.Revision
	dst.Generation = src.Generation
	dst.Status = src.Status
	dst.ExecutionTarget = src.ExecutionTarget
	dst.ExecutionAttemptID = src.ExecutionAttemptID
	dst.ExecutionResolvedAt = src.ExecutionResolvedAt
	dst.ExecutionResolvedBy = src.ExecutionResolvedBy
	dst.RuntimeInstanceID = src.RuntimeInstanceID
	dst.CheckpointID = src.CheckpointID
	dst.ResultArtifactID = src.ResultArtifactID
	dst.ErrorCode = src.ErrorCode
	dst.ErrorMessage = src.ErrorMessage
	dst.StartedAt = src.StartedAt
	dst.FinishedAt = src.FinishedAt
	dst.CancelRequestedAt = src.CancelRequestedAt
	dst.PauseRequestedAt = src.PauseRequestedAt
	dst.PausedAt = src.PausedAt
	dst.ResumedAt = src.ResumedAt
	dst.PauseReason = src.PauseReason
	dst.Attempt = src.Attempt
	dst.QueuedAt = src.QueuedAt
	dst.DeadlineAt = src.DeadlineAt
}
