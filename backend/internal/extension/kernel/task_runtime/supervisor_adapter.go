package task_runtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

type SupervisorTaskObserver interface {
	OnTaskStatusChanged(ctx context.Context, taskRunID string, status TaskRunStatus, generation int64) error
}

type SupervisorAdapter struct {
	service *TaskRuntimeService
}

func NewSupervisorAdapter(service *TaskRuntimeService) *SupervisorAdapter {
	return &SupervisorAdapter{service: service}
}

func (a *SupervisorAdapter) ActivateRun(ctx context.Context, taskRunID string) error {
	run, err := a.service.GetTaskRun(ctx, taskRunID)
	if err != nil {
		return err
	}
	if run.Status == RunStatusQueued {
		go a.service.tryDispatch()
	}
	return nil
}

func (a *SupervisorAdapter) PauseRun(ctx context.Context, taskRunID, reason string, generation int64) error {
	return a.service.PauseTask(ctx, PauseTaskRequest{
		TaskRunID:  taskRunID,
		Reason:     reason,
		Generation: generation,
	})
}

func (a *SupervisorAdapter) ResumeRun(ctx context.Context, taskRunID string, generation int64, kind string) error {
	return a.service.ResumeTask(ctx, ResumeTaskRequest{
		TaskRunID:  taskRunID,
		Generation: generation,
		ResumeKind: kind,
	})
}

func (a *SupervisorAdapter) CancelRun(ctx context.Context, taskRunID, reason string) error {
	return a.service.Cancel(ctx, taskRunID, reason)
}

func (a *SupervisorAdapter) HandleDeviceConnectionChanged(
	ctx context.Context,
	taskRunID string,
	activeSessionID interface{ String() string },
	generation int64,
) error {
	run, err := a.service.GetTaskRun(ctx, taskRunID)
	if err != nil {
		return err
	}

	if !run.HasStaleDeviceConnection(activeSessionID, generation) {
		return nil
	}

	if err := a.service.ClearExecutionConnectionBinding(ctx, taskRunID); err != nil {
		var taskErr *TaskError
		if errors.As(err, &taskErr) && taskErr.Code == ErrTaskExecutionPlacementInvalid {
			return nil
		}
		return err
	}

	return nil
}

func (a *SupervisorAdapter) BindTrustedExecutionTarget(
	ctx context.Context,
	taskRunID string,
	request TrustedExecutionTargetRequest,
) error {
	_, err := a.service.BindExecutionTarget(ctx, taskRunID, request)
	return err
}

func (a *SupervisorAdapter) RefreshProgress(
	ctx context.Context,
	taskRunID string,
	progressJSON []byte,
	startedAt time.Time,
	generation int64,
) error {
	run, err := a.service.GetTaskRun(ctx, taskRunID)
	if err != nil {
		return err
	}
	if run.Generation != generation {
		return fmt.Errorf("stale progress: generation mismatch")
	}
	guarded, finish, owned, err := a.service.callbackAuthority(ctx, run, run.ExecutionAttemptID.String(), generation)
	if err != nil {
		return err
	}
	defer finish()
	if owned {
		var progress TaskRunProgress
		if len(progressJSON) > 64<<10 || json.Unmarshal(progressJSON, &progress) != nil || progress.TaskRunID != taskRunID || run.StartedAt == nil || !run.StartedAt.Equal(startedAt) {
			return NewTaskError(ErrTaskExecutionAttemptInvalid, "任务监管进度格式或执行起点不一致")
		}
		return a.service.handleProgress(guarded, taskRunID, progress.Sequence, progress.Current, progress.Total, progress.Percentage, progress.Stage, progress.Message, run)
	}
	return a.service.store.PutProgress(ctx, taskRunID, time.Now().UnixNano(), progressJSON)
}

func (a *SupervisorAdapter) UpdateProgress(
	ctx context.Context,
	taskRunID string,
	seq int64,
	current, total *float64,
	stage, message string,
	generation int64,
) error {
	run, err := a.service.GetTaskRun(ctx, taskRunID)
	if err != nil {
		return err
	}
	if run.Generation != generation {
		return fmt.Errorf("stale progress: generation mismatch")
	}
	percentage := computePercentage(current, total)
	guarded, finish, _, err := a.service.callbackAuthority(ctx, run, run.ExecutionAttemptID.String(), generation)
	if err != nil {
		return err
	}
	defer finish()
	return a.service.handleProgress(guarded, taskRunID, seq, current, total, percentage, stage, message, run)
}

func computePercentage(current, total *float64) *float64 {
	if current == nil || total == nil || *total <= 0 {
		return nil
	}
	p := (*current / *total) * 100
	return &p
}

func (a *SupervisorAdapter) ensureSession(taskRunID string) error {
	if taskRunID == "" {
		return fmt.Errorf("taskRunId is required")
	}
	return nil
}
