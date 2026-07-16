package mindruntime

import (
	"context"
	"time"
)

type ReconciliationTarget string

const (
	ReconciliationSQLiteQdrant         ReconciliationTarget = "sqlite_qdrant"
	ReconciliationSQLiteSurrealDB      ReconciliationTarget = "sqlite_surrealdb"
	ReconciliationInteractionRunMsg    ReconciliationTarget = "interactionrun_messages"
	ReconciliationOutboxSideEffect     ReconciliationTarget = "outbox_side_effect"
	ReconciliationLeaseDelivery        ReconciliationTarget = "lease_delivery"
	ReconciliationTombstoneDerivedData ReconciliationTarget = "tombstone_derived_data"
)

type ReconciliationStrategy string

const (
	StrategyAutoRebuild    ReconciliationStrategy = "auto_rebuild"
	StrategyReindex        ReconciliationStrategy = "reindex"
	StrategyLogicalInvalid ReconciliationStrategy = "logical_invalidate"
	StrategyReleaseLease   ReconciliationStrategy = "release_lease"
	StrategyRetry          ReconciliationStrategy = "retry"
	StrategyCompensate     ReconciliationStrategy = "compensate"
	StrategyManualConfirm  ReconciliationStrategy = "manual_confirm"
)

type ReconciliationStatus string

const (
	ReconciliationStatusIdle      ReconciliationStatus = "idle"
	ReconciliationStatusRunning   ReconciliationStatus = "running"
	ReconciliationStatusPaused    ReconciliationStatus = "paused"
	ReconciliationStatusCompleted ReconciliationStatus = "completed"
	ReconciliationStatusCancelled ReconciliationStatus = "cancelled"
)

type ReconciliationScan struct {
	ID            string                 `json:"id"`
	Target        ReconciliationTarget   `json:"target"`
	Strategy      ReconciliationStrategy `json:"strategy"`
	Status        ReconciliationStatus   `json:"status"`
	StartedAt     time.Time              `json:"startedAt"`
	EndedAt       time.Time              `json:"endedAt,omitempty"`
	CursorID      string                 `json:"cursorId,omitempty"`
	BatchSize     int                    `json:"batchSize"`
	TotalScanned  int64                  `json:"totalScanned"`
	DiffsFound    int64                  `json:"diffsFound"`
	DiffsRepaired int64                  `json:"diffsRepaired"`
	DiffsSkipped  int64                  `json:"diffsSkipped"`
	BudgetUsedMS  int64                  `json:"budgetUsedMs"`
	BudgetLimitMS int64                  `json:"budgetLimitMs"`
	Diffs         []ReconciliationDiff   `json:"diffs,omitempty"`
}
type ReconciliationDiff struct {
	ID             string    `json:"id"`
	ScanID         string    `json:"scanId"`
	Source         string    `json:"source"`
	Target         string    `json:"target"`
	DiffType       string    `json:"diffType"`
	SourceKey      string    `json:"sourceKey"`
	TargetKey      string    `json:"targetKey"`
	Description    string    `json:"description"`
	Severity       string    `json:"severity"`
	AutoRepairable bool      `json:"autoRepairable"`
	RepairAction   string    `json:"repairAction,omitempty"`
	Repaired       bool      `json:"repaired"`
	RepairError    string    `json:"repairError,omitempty"`
	FoundAt        time.Time `json:"foundAt"`
	RepairedAt     time.Time `json:"repairedAt,omitempty"`
}
type ReconciliationConfig struct {
	BatchSize       int           `json:"batchSize"`
	PauseAfterBatch bool          `json:"pauseAfterBatch"`
	BudgetLimitMS   int64         `json:"budgetLimitMs"`
	MaxConcurrency  int           `json:"maxConcurrency"`
	AutoRepair      bool          `json:"autoRepair"`
	RetryCount      int           `json:"retryCount"`
	RetryDelay      time.Duration `json:"retryDelay"`
}
type ReconciliationCheckRequest struct {
	ScanID    string                 `json:"scanId"`
	Target    ReconciliationTarget   `json:"target"`
	Strategy  ReconciliationStrategy `json:"strategy"`
	CursorID  string                 `json:"cursorId,omitempty"`
	BatchSize int                    `json:"batchSize"`
	StartedAt time.Time              `json:"startedAt"`
}
type ReconciliationChecker interface {
	CheckReconciliation(context.Context, ReconciliationCheckRequest) ([]ReconciliationDiff, error)
}
type ReconciliationCheckerFunc func(context.Context, ReconciliationCheckRequest) ([]ReconciliationDiff, error)

func (f ReconciliationCheckerFunc) CheckReconciliation(ctx context.Context, req ReconciliationCheckRequest) ([]ReconciliationDiff, error) {
	return f(ctx, req)
}

type ReconciliationEntity struct {
	Store       string
	Kind        string
	Key         string
	Version     string
	Status      string
	Hash        string
	Deleted     bool
	LeasedUntil time.Time
	Fields      map[string]string
	References  map[string]string
}

type ReconciliationStateSource interface {
	ListReconciliationEntities(context.Context, ReconciliationCheckRequest) ([]ReconciliationEntity, error)
}

type ReconciliationStateSourceFunc func(context.Context, ReconciliationCheckRequest) ([]ReconciliationEntity, error)

func (f ReconciliationStateSourceFunc) ListReconciliationEntities(ctx context.Context, req ReconciliationCheckRequest) ([]ReconciliationEntity, error) {
	return f(ctx, req)
}

type ReconciliationWorkerTarget struct {
	Target   ReconciliationTarget
	Strategy ReconciliationStrategy
	Cursor   string
}
