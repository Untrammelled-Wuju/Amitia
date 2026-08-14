package task_runtime

import (
	"context"
	"encoding/json"
	"time"
)

type TaskDTO struct {
	TaskRunID         string            `json:"taskRunId"`
	TaskDefinitionID  string            `json:"taskDefinitionId"`
	ExtensionID       string            `json:"extensionId"`
	ModuleID          string            `json:"moduleId"`
	Status            string            `json:"status"`
	Priority          int               `json:"priority"`
	Input             json.RawMessage   `json:"input"`
	InputHash         string            `json:"inputHash"`
	InputArtifactID   *string           `json:"inputArtifactId,omitempty"`
	RuntimeInstanceID *string           `json:"runtimeInstanceId,omitempty"`
	CheckpointID      *string           `json:"checkpointId,omitempty"`
	ResultArtifactID  *string           `json:"resultArtifactId,omitempty"`
	Attempt           int               `json:"attempt"`
	MaxAttempts       int               `json:"maxAttempts"`
	CreatedAt         string            `json:"createdAt"`
	QueuedAt          *string           `json:"queuedAt,omitempty"`
	StartedAt         *string           `json:"startedAt,omitempty"`
	FinishedAt        *string           `json:"finishedAt,omitempty"`
	DeadlineAt        *string           `json:"deadlineAt,omitempty"`
	CancelRequestedAt *string           `json:"cancelRequestedAt,omitempty"`
	PauseReason       *string           `json:"pauseReason,omitempty"`
	PauseRequestedAt  *string           `json:"pauseRequestedAt,omitempty"`
	PausedAt          *string           `json:"pausedAt,omitempty"`
	ResumedAt         *string           `json:"resumedAt,omitempty"`
	ErrorCode         *string           `json:"errorCode,omitempty"`
	ErrorMessage      *string           `json:"errorMessage,omitempty"`
	Generation        int64             `json:"generation"`
	HasResolvedTarget interface{}       `json:"hasResolvedTarget,omitempty"`
	Execution         *TaskExecutionDTO `json:"execution,omitempty"`
}

type TaskExecutionDTO struct {
	Placement  string              `json:"placement"`
	Target     TaskExecutionTarget `json:"target,omitempty"`
	AttemptID  string              `json:"attemptId,omitempty"`
	ResolvedAt string              `json:"resolvedAt,omitempty"`
	ResolvedBy string              `json:"resolvedBy,omitempty"`
}

func TaskRunToDTO(run *TaskRun) *TaskDTO {
	if run == nil {
		return nil
	}
	dto := &TaskDTO{
		TaskRunID:         run.TaskRunID,
		TaskDefinitionID:  run.TaskDefinitionID,
		ExtensionID:       run.ExtensionID,
		ModuleID:          run.ModuleID,
		Status:            string(run.Status),
		Priority:          run.Priority,
		Input:             run.Input,
		InputHash:         run.InputHash,
		InputArtifactID:   run.InputArtifactID,
		RuntimeInstanceID: run.RuntimeInstanceID,
		CheckpointID:      run.CheckpointID,
		ResultArtifactID:  run.ResultArtifactID,
		Attempt:           run.Attempt,
		MaxAttempts:       run.MaxAttempts,
		CreatedAt:         run.CreatedAt.Format("2006-01-02T15:04:05.000Z07:00"),
		PauseReason:       run.PauseReason,
		ErrorCode:         run.ErrorCode,
		ErrorMessage:      run.ErrorMessage,
	}
	if run.QueuedAt != nil {
		s := run.QueuedAt.Format("2006-01-02T15:04:05.000Z07:00")
		dto.QueuedAt = &s
	}
	if run.StartedAt != nil {
		s := run.StartedAt.Format("2006-01-02T15:04:05.000Z07:00")
		dto.StartedAt = &s
	}
	if run.FinishedAt != nil {
		s := run.FinishedAt.Format("2006-01-02T15:04:05.000Z07:00")
		dto.FinishedAt = &s
	}
	if run.DeadlineAt != nil {
		s := run.DeadlineAt.Format("2006-01-02T15:04:05.000Z07:00")
		dto.DeadlineAt = &s
	}
	if run.CancelRequestedAt != nil {
		s := run.CancelRequestedAt.Format("2006-01-02T15:04:05.000Z07:00")
		dto.CancelRequestedAt = &s
	}
	if run.PauseRequestedAt != nil {
		s := run.PauseRequestedAt.Format("2006-01-02T15:04:05.000Z07:00")
		dto.PauseRequestedAt = &s
	}
	if run.PausedAt != nil {
		s := run.PausedAt.Format("2006-01-02T15:04:05.000Z07:00")
		dto.PausedAt = &s
	}
	if run.ResumedAt != nil {
		s := run.ResumedAt.Format("2006-01-02T15:04:05.000Z07:00")
		dto.ResumedAt = &s
	}

	if !run.ExecutionTarget.IsZero() {
		dto.Execution = &TaskExecutionDTO{
			Placement: string(run.ExecutionPlacement),
			Target:    run.ExecutionTarget,
			AttemptID: string(run.ExecutionAttemptID),
		}
		if run.ExecutionResolvedAt != nil {
			s := run.ExecutionResolvedAt.Format("2006-01-02T15:04:05.000Z07:00")
			dto.Execution.ResolvedAt = s
		}
		dto.Execution.ResolvedBy = run.ExecutionResolvedBy
	}

	return dto
}

type TaskRuntimeHandler struct {
	service *TaskRuntimeService
}

func NewTaskRuntimeHandler(service *TaskRuntimeService) *TaskRuntimeHandler {
	return &TaskRuntimeHandler{service: service}
}

func (h *TaskRuntimeHandler) GetTaskRun(ctx context.Context, taskRunID string) (*TaskDTO, error) {
	run, err := h.service.GetTaskRun(ctx, taskRunID)
	if err != nil {
		return nil, err
	}
	return TaskRunToDTO(run), nil
}

func (h *TaskRuntimeHandler) ListTaskRuns(ctx context.Context, filter ListTasksFilter) ([]*TaskDTO, error) {
	runs, err := h.service.ListTaskRuns(ctx, filter)
	if err != nil {
		return nil, err
	}
	dtos := make([]*TaskDTO, 0, len(runs))
	for _, run := range runs {
		dtos = append(dtos, TaskRunToDTO(run))
	}
	return dtos, nil
}

func (h *TaskRuntimeHandler) GetProgress(ctx context.Context, taskRunID string) (*TaskRunProgress, error) {
	return h.service.GetProgress(ctx, taskRunID)
}

func (h *TaskRuntimeHandler) GetResult(ctx context.Context, taskRunID string) (*TaskRunResult, error) {
	return h.service.GetResult(ctx, taskRunID)
}

func (h *TaskRuntimeHandler) GetTaskRunJSON(ctx context.Context, taskRunID string) (json.RawMessage, error) {
	run, err := h.service.GetTaskRun(ctx, taskRunID)
	if err != nil {
		return nil, err
	}
	return json.Marshal(TaskRunToDTO(run))
}

func (h *TaskRuntimeHandler) BindExecutionTarget(
	ctx context.Context,
	taskRunID string,
	request TrustedExecutionTargetRequest,
) (*TaskDTO, error) {
	run, err := h.service.BindExecutionTarget(ctx, taskRunID, request)
	if err != nil {
		return nil, err
	}
	return TaskRunToDTO(run), nil
}

func (h *TaskRuntimeHandler) ClearExecutionConnectionBinding(
	ctx context.Context,
	taskRunID string,
) error {
	return h.service.ClearExecutionConnectionBinding(ctx, taskRunID)
}

func (h *TaskRuntimeHandler) Enqueue(ctx context.Context, req EnqueueTaskRequest, def *TaskDefinition) (*EnqueueTaskResult, error) {
	return h.service.Enqueue(ctx, req, def)
}

func (h *TaskRuntimeHandler) Cancel(ctx context.Context, taskRunID, reason string) error {
	return h.service.Cancel(ctx, taskRunID, reason)
}

func (h *TaskRuntimeHandler) Retry(ctx context.Context, taskRunID string) (*TaskRun, error) {
	return h.service.Retry(ctx, taskRunID)
}

func (h *TaskRuntimeHandler) Recover(ctx context.Context, taskRunID string) (*TaskRun, error) {
	return h.service.Recover(ctx, taskRunID)
}

func (h *TaskRuntimeHandler) GetTaskDefinition(ctx context.Context, defID string) (*TaskDefinition, error) {
	return h.service.GetTaskDefinition(ctx, defID)
}

func (h *TaskRuntimeHandler) PutTaskDefinition(ctx context.Context, def *TaskDefinition) error {
	return h.service.PutTaskDefinition(ctx, def)
}

func (h *TaskRuntimeHandler) DeleteTaskDefinition(ctx context.Context, defID string) error {
	return h.service.DeleteTaskDefinition(ctx, defID)
}

func (h *TaskRuntimeHandler) DeleteByExtension(ctx context.Context, extensionID string) error {
	return h.service.DeleteByExtension(ctx, extensionID)
}

func (h *TaskRuntimeHandler) ListTaskDefinitions(ctx context.Context, extensionID string) ([]*TaskDefinition, error) {
	return h.service.ListTaskDefinitions(ctx, extensionID)
}

func FormatTime(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.Format("2006-01-02T15:04:05.000Z07:00")
}
