package queue

import (
	"context"
	"time"
)

func (pq *PriorityQueue) loadCheckpoint() {
	if pq.store == nil {
		return
	}
	ctx := context.Background()
	records, err := pq.store.LoadPending(ctx)
	if err != nil {
		pq.recordPersistenceErrorLocked(err)
		return
	}
	now := time.Now().UTC()
	for _, record := range records {
		priority := PriorityLevel(record.Priority)
		if priority < PriorityP0 || priority > PriorityP5 {
			priority = PriorityP5
		}
		if !record.Deadline.IsZero() && now.After(record.Deadline) {
			continue
		}
		createdAt := record.AvailableAt
		if createdAt.IsZero() {
			createdAt = now
		}
		task := &Task{ID: record.TaskID, Priority: priority, Scope: record.Scope, CreatedAt: createdAt, Deadline: record.Deadline, Status: TaskPending, Done: make(chan struct{})}
		pq.queues[task.Priority] = append(pq.queues[task.Priority], task)
	}
	pq.metrics.RecordQueueDepth(int64(pq.totalLocked()))
	pq.persistLocked()
}

func (pq *PriorityQueue) persistLocked() {
	if pq.store == nil {
		return
	}
	ctx := context.Background()
	for level := PriorityP0; level <= PriorityP5; level++ {
		for _, task := range pq.queues[level] {
			if task.Status == TaskPending || task.Status == TaskRunning {
				record := &RuntimeQueueRecord{TaskID: task.ID, Scope: task.Scope, Priority: int(task.Priority), Status: string(task.Status), AvailableAt: task.CreatedAt, Deadline: task.Deadline, Attempt: 0}
				if err := pq.store.Upsert(ctx, record); err != nil {
					pq.recordPersistenceErrorLocked(err)
				}
			} else {
				_ = pq.store.Delete(ctx, task.ID)
			}
		}
	}
	for task := range pq.activeTasks {
		if task.Status == TaskRunning {
			record := &RuntimeQueueRecord{TaskID: task.ID, Scope: task.Scope, Priority: int(task.Priority), Status: string(TaskPending), AvailableAt: task.CreatedAt, Deadline: task.Deadline, Attempt: 0}
			if err := pq.store.Upsert(ctx, record); err != nil {
				pq.recordPersistenceErrorLocked(err)
			}
		}
	}
	pq.lastPersistErr = nil
}

func (pq *PriorityQueue) recordPersistenceErrorLocked(err error) {
	pq.lastPersistErr = err
	if pq.onPersistError != nil {
		pq.onPersistError(err)
	}
}
