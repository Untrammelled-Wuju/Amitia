package mindruntime

import "time"

type DeletionScope string

const (
	DeletionScopeMemory   DeletionScope = "memory"
	DeletionScopeBelief   DeletionScope = "belief"
	DeletionScopeRelation DeletionScope = "relation"
	DeletionScopeTrace    DeletionScope = "trace"
	DeletionScopeAll      DeletionScope = "all"
)

type DeletionStatus string

const (
	DeletionStatusPending   DeletionStatus = "pending"
	DeletionStatusBlocked   DeletionStatus = "blocked"
	DeletionStatusCleaning  DeletionStatus = "cleaning"
	DeletionStatusCompleted DeletionStatus = "completed"
	DeletionStatusFailed    DeletionStatus = "failed"
)

type CleanupItemStatus string

const (
	CleanupItemStatusQueued    CleanupItemStatus = "queued"
	CleanupItemStatusClaimed   CleanupItemStatus = "claimed"
	CleanupItemStatusRetry     CleanupItemStatus = "retry"
	CleanupItemStatusCompleted CleanupItemStatus = "completed"
	CleanupItemStatusDead      CleanupItemStatus = "dead"
)

type RecalculationTaskStatus string

const (
	RecalculationTaskStatusPending   RecalculationTaskStatus = "pending"
	RecalculationTaskStatusClaimed   RecalculationTaskStatus = "claimed"
	RecalculationTaskStatusRunning   RecalculationTaskStatus = "running"
	RecalculationTaskStatusCompleted RecalculationTaskStatus = "completed"
	RecalculationTaskStatusFailed    RecalculationTaskStatus = "failed"
	RecalculationTaskStatusDead      RecalculationTaskStatus = "dead"
)

const (
	DefaultCleanupMaxAttempts      = 5
	DefaultCleanupBatchSize        = 3
	DefaultCleanupLeaseDuration    = 120 * time.Second
	DefaultCleanupRetryBackoffBase = 30 * time.Second
	DefaultMaxCleanupIterations    = 10
	DefaultRecalcMaxAttempts       = 3
	DefaultRecalcLeaseDuration     = 300 * time.Second
	DefaultRecalcBatchSize         = 5
)

type DeletionTombstone struct {
	ID               string         `json:"id"`
	TargetID         string         `json:"targetId"`
	TargetType       string         `json:"targetType"`
	Scope            DeletionScope  `json:"scope"`
	RequestedAt      time.Time      `json:"requestedAt"`
	BlockedUntil     time.Time      `json:"blockedUntil"`
	Status           DeletionStatus `json:"status"`
	ItemsCount       int            `json:"itemsCount"`
	CleanedCount     int            `json:"cleanedCount"`
	FailedCount      int            `json:"failedCount"`
	CompletedAt      *time.Time     `json:"completedAt,omitempty"`
	RetrievalBlocked bool           `json:"retrievalBlocked"`
}

type OutboxCleanupItem struct {
	ID          string            `json:"id"`
	Storage     string            `json:"storage"`
	TargetID    string            `json:"targetId"`
	TargetKind  string            `json:"targetKind"`
	Status      CleanupItemStatus `json:"status"`
	Attempts    int               `json:"attempts"`
	MaxAttempts int               `json:"maxAttempts"`
	NextRetryAt time.Time         `json:"nextRetryAt"`
	LeaseOwner  string            `json:"leaseOwner"`
	LeaseToken  string            `json:"leaseToken"`
	LeasedUntil time.Time         `json:"leasedUntil"`
	LastError   string            `json:"lastError,omitempty"`
	CleanedAt   *time.Time        `json:"cleanedAt,omitempty"`
}

type RecalculationTask struct {
	ID           string                  `json:"id"`
	TriggerType  string                  `json:"triggerType"`
	TargetID     string                  `json:"targetId"`
	AffectedZone string                  `json:"affectedZone"`
	Priority     int                     `json:"priority"`
	CreatedAt    time.Time               `json:"createdAt"`
	Status       RecalculationTaskStatus `json:"status"`
	Description  string                  `json:"description"`
	Attempts     int                     `json:"attempts"`
	MaxAttempts  int                     `json:"maxAttempts"`
	NextRetryAt  time.Time               `json:"nextRetryAt"`
	LeaseOwner   string                  `json:"leaseOwner"`
	LeaseToken   string                  `json:"leaseToken"`
	LeasedUntil  time.Time               `json:"leasedUntil"`
	LastError    string                  `json:"lastError,omitempty"`
	CompletedAt  *time.Time              `json:"completedAt,omitempty"`
}

type SecurityTestKind string

const (
	SecurityTestEmotionalHijacking  SecurityTestKind = "emotional_hijacking"
	SecurityTestExclusiveDependency SecurityTestKind = "exclusive_dependency"
	SecurityTestPromptInjection     SecurityTestKind = "prompt_injection"
	SecurityTestDataLeakage         SecurityTestKind = "data_leakage"
	SecurityTestPostDeletionRecall  SecurityTestKind = "post_deletion_recall"
)

type SecurityTestResult struct {
	Kind     SecurityTestKind `json:"kind"`
	Passed   bool             `json:"passed"`
	Severity string           `json:"severity"`
	Detail   string           `json:"detail"`
	Evidence string           `json:"evidence,omitempty"`
	TestedAt time.Time        `json:"testedAt"`
}

type DeletionRequest struct {
	TargetID   string        `json:"targetId"`
	TargetType string        `json:"targetType"`
	Scope      DeletionScope `json:"scope"`
	Reason     string        `json:"reason"`
}

type OutboxCleanupExecutor interface {
	CleanupOutboxItem(item OutboxCleanupItem) error
}

type RecalculationTaskExecutor interface {
	ExecuteRecalculation(task RecalculationTask) error
}
