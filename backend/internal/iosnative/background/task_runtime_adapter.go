package background

import (
	"context"
	"encoding/json"

	"github.com/u-ai/backend/internal/extension/kernel/task_runtime"
)

type taskRuntimeServiceAdapter struct {
	svc *task_runtime.TaskRuntimeService
}

func NewTaskRuntimeServiceAdapter(svc *task_runtime.TaskRuntimeService) TaskRuntimePort {
	return &taskRuntimeServiceAdapter{svc: svc}
}

func (a *taskRuntimeServiceAdapter) GetRun(ctx context.Context, taskRunID string) (*TaskRunRecord, error) {
	run, err := a.svc.GetTaskRun(ctx, taskRunID)
	if err != nil {
		return nil, err
	}
	if run == nil {
		return nil, nil
	}
	rec := &TaskRunRecord{
		TaskRunID:        run.TaskRunID,
		TaskDefinitionID: run.TaskDefinitionID,
		Status:           TaskRunStatus(run.Status),
		CheckpointID:     run.CheckpointID,
		ErrorCode:        run.ErrorCode,
		ErrorMessage:     run.ErrorMessage,
	}
	if run.StartedAt != nil {
		t := *run.StartedAt
		rec.StartedAt = &t
	}
	if run.FinishedAt != nil {
		t := *run.FinishedAt
		rec.CompletedAt = &t
	}
	if prog, err := a.svc.GetProgress(ctx, taskRunID); err == nil && prog != nil {
		rec.Progress = &TaskProgressRecord{
			Phase: prog.Message,
		}
		if prog.Total != nil {
			rec.Progress.TotalUnits = int64(*prog.Total)
		}
		if prog.Current != nil {
			rec.Progress.CompletedUnits = int64(*prog.Current)
		}
	}
	return rec, nil
}

func (a *taskRuntimeServiceAdapter) SubmitRun(ctx context.Context, taskRunID string) error {
	return nil
}

func (a *taskRuntimeServiceAdapter) ResumeRun(ctx context.Context, taskRunID string) error {
	return a.svc.ResumeTask(ctx, task_runtime.ResumeTaskRequest{TaskRunID: taskRunID})
}

func (a *taskRuntimeServiceAdapter) SignalExpiration(ctx context.Context, taskRunID string, reason string) error {
	return a.svc.Cancel(ctx, taskRunID, reason)
}

func (a *taskRuntimeServiceAdapter) CompleteRun(ctx context.Context, taskRunID string, success bool, errCode string, errMsg string) error {
	status := "succeeded"
	if !success {
		status = "failed"
	}
	return a.svc.HandleExternalFinish(ctx, taskRunID, status, nil, "", errCode, errMsg)
}

func (a *taskRuntimeServiceAdapter) ReportProgress(ctx context.Context, taskRunID string, totalUnits, completedUnits int64, phase string) error {
	return a.svc.HandleExternalProgress(ctx, taskRunID, completedUnits, totalUnits, phase)
}

func (a *taskRuntimeServiceAdapter) GetCheckpoint(ctx context.Context, taskRunID string) (*CheckpointData, error) {
	cp, err := a.svc.GetLatestCheckpoint(ctx, taskRunID)
	if err != nil {
		return nil, err
	}
	if cp == nil {
		return nil, nil
	}
	var data map[string]any
	if len(cp.Payload) > 0 {
		_ = json.Unmarshal(cp.Payload, &data)
	}
	return &CheckpointData{
		Data: data,
	}, nil
}

func (a *taskRuntimeServiceAdapter) SetCheckpoint(ctx context.Context, taskRunID string, cp CheckpointData) error {
	payload, err := json.Marshal(cp.Data)
	if err != nil {
		return err
	}
	return a.svc.HandleExternalCheckpoint(ctx, taskRunID, cp.LastUnit, cp.Phase, payload)
}

func (a *taskRuntimeServiceAdapter) ClearCheckpoint(ctx context.Context, taskRunID string) error {
	return a.svc.ClearLatestCheckpoint(ctx, taskRunID)
}

func (a *taskRuntimeServiceAdapter) CancelRun(ctx context.Context, taskRunID string) error {
	return a.svc.Cancel(ctx, taskRunID, "user_cancelled")
}

var _ TaskRuntimePort = (*taskRuntimeServiceAdapter)(nil)
