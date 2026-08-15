package background

import (
	"context"
	"time"
)

type TaskRunStatus string

const (
	TaskRunStatusQueued     TaskRunStatus = "queued"
	TaskRunStatusRunning    TaskRunStatus = "running"
	TaskRunStatusPaused     TaskRunStatus = "paused"
	TaskRunStatusCompleted  TaskRunStatus = "completed"
	TaskRunStatusFailed     TaskRunStatus = "failed"
	TaskRunStatusCancelled  TaskRunStatus = "cancelled"
	TaskRunStatusExpired    TaskRunStatus = "expired"
)

type TaskRunRecord struct {
	TaskRunID        string
	TaskDefinitionID string
	Status           TaskRunStatus
	Progress         *TaskProgressRecord
	CheckpointID     *string
	StartedAt        *time.Time
	CompletedAt      *time.Time
	ErrorCode        *string
	ErrorMessage     *string
}

type TaskProgressRecord struct {
	TotalUnits     int64
	CompletedUnits int64
	Phase          string
}

type CheckpointData struct {
	LastUnit int64
	Phase    string
	Data     map[string]any
}

type TaskRuntimePort interface {
	GetRun(ctx context.Context, taskRunID string) (*TaskRunRecord, error)
	SubmitRun(ctx context.Context, taskRunID string) error
	ResumeRun(ctx context.Context, taskRunID string) error
	SignalExpiration(ctx context.Context, taskRunID string, reason string) error
	CompleteRun(ctx context.Context, taskRunID string, success bool, errCode string, errMsg string) error
	ReportProgress(ctx context.Context, taskRunID string, totalUnits, completedUnits int64, phase string) error
	GetCheckpoint(ctx context.Context, taskRunID string) (*CheckpointData, error)
	SetCheckpoint(ctx context.Context, taskRunID string, cp CheckpointData) error
	ClearCheckpoint(ctx context.Context, taskRunID string) error
	CancelRun(ctx context.Context, taskRunID string) error
}
