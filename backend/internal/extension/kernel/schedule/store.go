package schedule

import (
	"context"
	"time"
)

// TriggerCreateStore is an optional stronger persistence contract used by
// planners/recovery workers to atomically claim a durable trigger. Stores that
// implement it must never mutate an already-existing trigger when created=false.
type TriggerCreateStore interface {
	CreateTriggerIfAbsent(ctx context.Context, record *ScheduleTriggerRecord) (created bool, err error)
}

type TriggerLeaseHeartbeatStore interface {
	RenewTriggerLease(context.Context, string, string, time.Time) (bool, error)
	ReleaseOwnedTriggerLease(context.Context, string, string) error
}

type TriggerHistoryCleanupStore interface {
	CleanTriggerRunHistory(context.Context, string) error
}

func createTriggerIfAbsent(ctx context.Context, store ScheduleStore, record *ScheduleTriggerRecord) (bool, error) {
	if strong, ok := store.(TriggerCreateStore); ok {
		return strong.CreateTriggerIfAbsent(ctx, record)
	}
	if existing, err := store.GetTriggerByIdempotencyKey(ctx, record.IdempotencyKey); err == nil && existing != nil {
		return false, nil
	}
	if err := store.PutTrigger(ctx, record); err != nil {
		// A store may enforce uniqueness even if it does not implement the stronger
		// interface. Resolve a concurrent winner before surfacing the write error.
		if existing, getErr := store.GetTriggerByIdempotencyKey(ctx, record.IdempotencyKey); getErr == nil && existing != nil {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

type ScheduleStore interface {
	PutDefinition(ctx context.Context, def *ScheduleContributionDefinition) error
	GetDefinition(ctx context.Context, scheduleID string) (*ScheduleContributionDefinition, error)
	ListDefinitions(ctx context.Context, extensionID string) ([]*ScheduleContributionDefinition, error)
	ListAllDefinitions(ctx context.Context) ([]*ScheduleContributionDefinition, error)
	DeleteDefinition(ctx context.Context, scheduleID string) error

	PutState(ctx context.Context, state *ScheduleState) error
	GetState(ctx context.Context, scheduleID string) (*ScheduleState, error)
	ListDueStates(ctx context.Context, now time.Time, limit int) ([]*ScheduleState, error)
	ListStatesByStatus(ctx context.Context, status ScheduleDefinitionStatus) ([]*ScheduleState, error)

	PutTrigger(ctx context.Context, record *ScheduleTriggerRecord) error
	GetTrigger(ctx context.Context, triggerID string) (*ScheduleTriggerRecord, error)
	GetTriggerByIdempotencyKey(ctx context.Context, key string) (*ScheduleTriggerRecord, error)
	ListTriggersBySchedule(ctx context.Context, scheduleID string, limit int) ([]*ScheduleTriggerRecord, error)
	ListDueTriggers(ctx context.Context, now time.Time, limit int) ([]*ScheduleTriggerRecord, error)
	UpdateTriggerStatus(ctx context.Context, triggerID string, status ScheduleRunStatus, updates map[string]any) error
	AcquireTriggerLease(ctx context.Context, triggerID string, owner string, expiresAt time.Time) (bool, error)
	ReleaseTriggerLease(ctx context.Context, triggerID string) error
	ReclaimExpiredLeases(ctx context.Context, now time.Time) (int, error)
	DeleteTriggersBySchedule(ctx context.Context, scheduleID string) error

	PutRun(ctx context.Context, run *ScheduleRunRecord) error
	GetRun(ctx context.Context, runID string) (*ScheduleRunRecord, error)
	ListRunsBySchedule(ctx context.Context, scheduleID string, limit int) ([]*ScheduleRunRecord, error)
	ListRunsByTrigger(ctx context.Context, triggerID string) ([]*ScheduleRunRecord, error)
	UpdateRunStatus(ctx context.Context, runID string, status ScheduleRunStatus, updates map[string]any) error
	CountActiveRuns(ctx context.Context, scheduleID string) (int, error)
	CountActiveRunsByExtension(ctx context.Context, extensionID string) (int, error)

	PutMisfire(ctx context.Context, record *ScheduleMisfireRecord) error
	ListMisfiresBySchedule(ctx context.Context, scheduleID string, limit int) ([]*ScheduleMisfireRecord, error)

	PutRetry(ctx context.Context, record *ScheduleRetryRecord) error
	ListDueRetries(ctx context.Context, now time.Time, limit int) ([]*ScheduleRetryRecord, error)
	DeleteRetry(ctx context.Context, retryID string) error

	GetCircuit(ctx context.Context, scheduleID string) (*ScheduleCircuitRecord, error)
	PutCircuit(ctx context.Context, record *ScheduleCircuitRecord) error
	DeleteCircuit(ctx context.Context, scheduleID string) error

	PutQuarantine(ctx context.Context, record *ScheduleQuarantineRecord) error
	ListQuarantines(ctx context.Context) ([]*ScheduleQuarantineRecord, error)

	DeleteAllByExtension(ctx context.Context, extensionID string) error
}

type TransactionalScheduleStore interface {
	WithTransaction(context.Context, func(context.Context) error) error
}

type OccurrenceCounter interface {
	CountScheduledTriggers(context.Context, string) (int, error)
}

func countScheduledTriggers(ctx context.Context, store ScheduleStore, id string) (int, error) {
	def, err := store.GetDefinition(ctx, id)
	if err != nil {
		return 0, err
	}
	restored := 0
	if def != nil {
		restored = def.Trigger.RestoredOccurrences
	}
	if restored < 0 {
		return 0, ErrInvalidTriggerType
	}
	if counter, ok := store.(OccurrenceCounter); ok {
		count, err := counter.CountScheduledTriggers(ctx, id)
		if count < 0 || restored > int(^uint(0)>>1)-count {
			return 0, ErrInvalidTriggerType
		}
		return count + restored, err
	}
	triggers, err := store.ListTriggersBySchedule(ctx, id, -1)
	count := restored
	for _, trigger := range triggers {
		if !trigger.Manual {
			count++
		}
	}
	return count, err
}

func withScheduleTransaction(ctx context.Context, store ScheduleStore, fn func(context.Context) error) error {
	if transactional, ok := store.(TransactionalScheduleStore); ok {
		return transactional.WithTransaction(ctx, fn)
	}
	return fn(ctx)
}
