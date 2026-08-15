package task_runtime

import (
	"context"
	"database/sql"
	"time"

	"github.com/u-ai/backend/internal/runtimeidentity"
)

type TaskDomainEventType string

const (
	TaskEventCreated                  TaskDomainEventType = "created"
	TaskEventQueued                   TaskDomainEventType = "queued"
	TaskEventExecutionTargetBound     TaskDomainEventType = "execution_target_bound"
	TaskEventConnectionBindingChanged TaskDomainEventType = "connection_binding_changed"
	TaskEventAttemptStarted           TaskDomainEventType = "attempt_started"
	TaskEventStarting                 TaskDomainEventType = "starting"
	TaskEventResuming                 TaskDomainEventType = "resuming"
	TaskEventRunning                  TaskDomainEventType = "running"
	TaskEventSucceeded                TaskDomainEventType = "succeeded"
	TaskEventFailed                   TaskDomainEventType = "failed"
	TaskEventCancelled                TaskDomainEventType = "cancelled"
	TaskEventPaused                   TaskDomainEventType = "paused"
	TaskEventResumed                  TaskDomainEventType = "resumed"
	TaskEventTimedOut                 TaskDomainEventType = "timed_out"
	TaskEventRecoveryRequired         TaskDomainEventType = "recovery_required"
)

type TaskDomainEvent struct {
	Type       TaskDomainEventType
	Run        TaskRun
	Reason     string
	ErrorCode  string
	OccurredAt time.Time
}

type TaskEventSink interface {
	TaskEvent(ctx context.Context, event TaskDomainEvent) error
}

type TaskTx interface {
	TaskStore
	RawTx() *sql.Tx
}

type TaskUnitOfWork interface {
	WithinTaskTx(
		ctx context.Context,
		fn func(ctx context.Context) error,
	) error
}

func TaskEventPartitionKey(event TaskDomainEvent) string {
	uid := event.Run.ExecutionTarget.UserID
	if uid != "" {
		return uid.String()
	}
	return "system"
}

func TaskEventOrderingKey(event TaskDomainEvent) string {
	return event.Run.TaskRunID
}

func TaskEventAggregateVersion(event TaskDomainEvent) *int64 {
	return &event.Run.Revision
}

func TaskRunUserID(event TaskDomainEvent) runtimeidentity.UserID {
	return event.Run.ExecutionTarget.UserID
}
