package task_runtime

import (
	"context"
	"fmt"
	"sync"
	"time"
)

type TaskQueue struct {
	store      TaskStore
	leaseOwner string
	leaseDur   time.Duration
	mu         sync.Mutex
}

func NewTaskQueue(store TaskStore, leaseOwner string, leaseDuration time.Duration) *TaskQueue {
	return &TaskQueue{
		store:      store,
		leaseOwner: leaseOwner,
		leaseDur:   leaseDuration,
	}
}

func (q *TaskQueue) Enqueue(ctx context.Context, run *TaskRun) error {
	now := time.Now().UTC()
	entry := &TaskQueueEntry{
		TaskRunID:   run.TaskRunID,
		Priority:    run.Priority,
		AvailableAt: now,
		CreatedAt:   now,
	}
	if err := q.store.EnqueueTask(ctx, entry); err != nil {
		return fmt.Errorf("task_queue: enqueue: %w", err)
	}
	return nil
}

func (q *TaskQueue) ReenqueueWithDelay(ctx context.Context, run *TaskRun, delay time.Duration) error {
	now := time.Now().UTC()
	entry := &TaskQueueEntry{
		TaskRunID:   run.TaskRunID,
		Priority:    run.Priority,
		AvailableAt: now.Add(delay),
		CreatedAt:   now,
	}
	if err := q.store.EnqueueTask(ctx, entry); err != nil {
		return fmt.Errorf("task_queue: reenqueue: %w", err)
	}
	return nil
}

func (q *TaskQueue) Dequeue(ctx context.Context) (*TaskQueueEntry, error) {
	entry, err := q.store.DequeueTask(ctx, q.leaseOwner, q.leaseDur)
	if err != nil {
		return nil, fmt.Errorf("task_queue: dequeue: %w", err)
	}
	return entry, nil
}

func (q *TaskQueue) Remove(ctx context.Context, taskRunID string) error {
	return q.store.RemoveFromQueue(ctx, taskRunID)
}

func (q *TaskQueue) ReclaimExpired(ctx context.Context) (int, error) {
	return q.store.ReclaimExpiredLeases(ctx)
}

func (q *TaskQueue) GetEntry(ctx context.Context, taskRunID string) (*TaskQueueEntry, error) {
	return q.store.GetQueueEntry(ctx, taskRunID)
}

type ConcurrencyLimiter struct {
	store            TaskStore
	globalMax        int
	perExtensionMax  int
	perDefinitionMax int
}

func NewConcurrencyLimiter(store TaskStore, cfg TaskRuntimeConfig) *ConcurrencyLimiter {
	return &ConcurrencyLimiter{
		store:            store,
		globalMax:        cfg.GlobalMaxConcurrent,
		perExtensionMax:  cfg.PerExtensionMaxConcurrent,
		perDefinitionMax: cfg.PerDefinitionMaxConcurrent,
	}
}

func (l *ConcurrencyLimiter) CanStart(ctx context.Context, run *TaskRun) (bool, string, error) {
	global, err := l.store.CountActive(ctx)
	if err != nil {
		return false, "", err
	}
	if global >= l.globalMax {
		return false, "global_concurrency_limit", nil
	}
	extCount, err := l.store.CountActiveByExtension(ctx, run.ExtensionID)
	if err != nil {
		return false, "", err
	}
	if extCount >= l.perExtensionMax {
		return false, "extension_concurrency_limit", nil
	}
	defCount, err := l.store.CountActiveByDefinition(ctx, run.TaskDefinitionID)
	if err != nil {
		return false, "", err
	}
	if defCount >= l.perDefinitionMax {
		return false, "definition_concurrency_limit", nil
	}
	return true, "", nil
}
