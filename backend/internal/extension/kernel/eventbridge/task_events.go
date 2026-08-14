package eventbridge

import (
	"context"
	"fmt"

	"github.com/u-ai/backend/internal/extension/kernel/event"
	"github.com/u-ai/backend/internal/extension/kernel/persistence/sqlite"
	"github.com/u-ai/backend/internal/extension/kernel/task_runtime"
)

const (
	taskProducerID = "task-runtime-kernel"
)

type TaskEventPublisher struct {
	publisher *Publisher
}

func NewTaskEventPublisher(publisher *Publisher) *TaskEventPublisher {
	return &TaskEventPublisher{publisher: publisher}
}

func (p *TaskEventPublisher) TaskEvent(ctx context.Context, domainEvent task_runtime.TaskDomainEvent) error {
	typeID, ok := taskEventTypeID(domainEvent.Type)
	if !ok {
		return fmt.Errorf("eventbridge: unknown task event type: %s", domainEvent.Type)
	}
	opts := taskOptions(domainEvent)
	tx, ok := sqlite.TxFromContext(ctx)
	if ok {
		_, err := p.publisher.PublishTx(ctx, tx, typeID, eventVersion, domainEvent, opts)
		return err
	}
	_, err := p.publisher.Publish(ctx, typeID, eventVersion, domainEvent, opts)
	return err
}

func taskEventTypeID(t task_runtime.TaskDomainEventType) (event.EventTypeID, bool) {
	switch t {
	case task_runtime.TaskEventCreated:
		return taskRunCreated, true
	case task_runtime.TaskEventQueued:
		return taskRunQueued, true
	case task_runtime.TaskEventExecutionTargetBound:
		return taskExecutionTargetBound, true
	case task_runtime.TaskEventConnectionBindingChanged:
		return taskExecutionConnectionBindingChanged, true
	case task_runtime.TaskEventAttemptStarted:
		return taskExecutionAttemptStarted, true
	case task_runtime.TaskEventRunning:
		return taskRunRunning, true
	case task_runtime.TaskEventSucceeded:
		return taskRunSucceeded, true
	case task_runtime.TaskEventFailed:
		return taskRunFailed, true
	case task_runtime.TaskEventCancelled:
		return taskRunCancelled, true
	case task_runtime.TaskEventPaused:
		return taskRunPaused, true
	case task_runtime.TaskEventTimedOut:
		return taskRunTimedOut, true
	case task_runtime.TaskEventRecoveryRequired:
		return taskRunRecoveryRequired, true
	}
	return "", false
}

func taskOptions(e task_runtime.TaskDomainEvent) event.PublishOptions {
	return event.PublishOptions{
		Domain:           event.EventDomainTask,
		ProducerType:     event.EventProducerTypeTask,
		ProducerID:       taskProducerID,
		AggregateType:    "task_run",
		AggregateID:      e.Run.TaskRunID,
		AggregateVersion: &e.Run.Revision,
		PartitionKey:     taskPartitionKey(e),
		OrderingKey:      e.Run.TaskRunID,
		TraceID:          e.Run.TraceID,
		OperationID:      e.Run.OperationID,
		CausationID:      e.Run.CausationID,
	}
}

func taskPartitionKey(e task_runtime.TaskDomainEvent) string {
	uid := e.Run.ExecutionTarget.UserID
	if uid != "" {
		return uid.String()
	}
	return "system"
}
