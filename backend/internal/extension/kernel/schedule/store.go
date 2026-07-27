package schedule

import (
	"context"
	"time"
)

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
