package schedule

import (
	"encoding/json"
	"time"
)

type TriggerType string

const (
	TriggerTypeCron     TriggerType = "cron"
	TriggerTypeInterval TriggerType = "interval"
	TriggerTypeOneShot  TriggerType = "one_shot"
)

type TargetType string

const (
	TargetTypeTool           TargetType = "tool"
	TargetTypeWorkflow       TargetType = "workflow"
	TargetTypeTask           TargetType = "task"
	TargetTypeRuntimeHandler TargetType = "runtime_handler"
)

type IdempotencyMode string

const (
	IdempotencyModeIdempotent         IdempotencyMode = "idempotent"
	IdempotencyModeConditionallyIdem  IdempotencyMode = "conditionally_idempotent"
	IdempotencyModeNonIdempotent      IdempotencyMode = "non_idempotent"
)

type ScheduleDefinitionStatus string

const (
	DefinitionStatusCreated      ScheduleDefinitionStatus = "created"
	DefinitionStatusEnabled      ScheduleDefinitionStatus = "enabled"
	DefinitionStatusDisabled     ScheduleDefinitionStatus = "disabled"
	DefinitionStatusPaused       ScheduleDefinitionStatus = "paused"
	DefinitionStatusExpired      ScheduleDefinitionStatus = "expired"
	DefinitionStatusUninstalled  ScheduleDefinitionStatus = "uninstalled"
)

type ScheduleRunStatus string

const (
	RunStatusWaiting     ScheduleRunStatus = "waiting"
	RunStatusDue         ScheduleRunStatus = "due"
	RunStatusLeased      ScheduleRunStatus = "leased"
	RunStatusTriggering  ScheduleRunStatus = "triggering"
	RunStatusRunning     ScheduleRunStatus = "running"
	RunStatusRetryWait   ScheduleRunStatus = "retry_wait"
	RunStatusCompleted   ScheduleRunStatus = "completed"
	RunStatusFailed      ScheduleRunStatus = "failed"
	RunStatusExpired     ScheduleRunStatus = "expired"
	RunStatusBlocked     ScheduleRunStatus = "blocked"
	RunStatusSkipped     ScheduleRunStatus = "skipped"
	RunStatusCancelled   ScheduleRunStatus = "cancelled"
	RunStatusQuarantined ScheduleRunStatus = "quarantined"
	RunStatusRecoveryRequired ScheduleRunStatus = "recovery_required"
)

func (s ScheduleRunStatus) IsTerminal() bool {
	switch s {
	case RunStatusCompleted, RunStatusFailed, RunStatusExpired,
		RunStatusSkipped, RunStatusCancelled, RunStatusQuarantined:
		return true
	}
	return false
}

func (s ScheduleRunStatus) IsActive() bool {
	switch s {
	case RunStatusWaiting, RunStatusDue, RunStatusLeased,
		RunStatusTriggering, RunStatusRunning, RunStatusRetryWait:
		return true
	}
	return false
}

type MisfirePolicy string

const (
	MisfirePolicySkip             MisfirePolicy = "skip"
	MisfirePolicyFireOnce         MisfirePolicy = "fire_once"
	MisfirePolicyCatchUpLimited   MisfirePolicy = "catch_up_limited"
	MisfirePolicyRescheduleFromNow MisfirePolicy = "reschedule_from_now"
)

type OverlapPolicy string

const (
	OverlapPolicyForbid        OverlapPolicy = "forbid"
	OverlapPolicyAllow         OverlapPolicy = "allow"
	OverlapPolicyReplace       OverlapPolicy = "replace"
	OverlapPolicyQueueOne      OverlapPolicy = "queue_one"
	OverlapPolicySkipIfRunning OverlapPolicy = "skip_if_running"
)

type DSTSpringPolicy string

const (
	DSTSpringSkip             DSTSpringPolicy = "skip"
	DSTSpringFireOnceAfterGap DSTSpringPolicy = "fire_once_after_gap"
	DSTSpringNextValidTime    DSTSpringPolicy = "next_valid_time"
)

type DSTFallPolicy string

const (
	DSTFallFireOnceFirst  DSTFallPolicy = "fire_once_first"
	DSTFallFireOnceSecond DSTFallPolicy = "fire_once_second"
	DSTFallFireTwice      DSTFallPolicy = "fire_twice"
)

type CircuitState string

const (
	CircuitStateClosed    CircuitState = "closed"
	CircuitStateOpen      CircuitState = "open"
	CircuitStateHalfOpen  CircuitState = "half_open"
)

type QuarantineReason string

const (
	QuarantineDuplicateTrigger     QuarantineReason = "duplicate_trigger"
	QuarantineDualScheduler        QuarantineReason = "dual_scheduler"
	QuarantineIdempotencyViolation QuarantineReason = "idempotency_violation"
	QuarantineDefinitionTampered   QuarantineReason = "definition_tampered"
	QuarantineTargetReplaced       QuarantineReason = "target_replaced"
	QuarantineScopeEscalation      QuarantineReason = "scope_escalation"
	QuarantinePermissionBypass     QuarantineReason = "permission_bypass"
	QuarantineUndeclaredProcess    QuarantineReason = "undeclared_process"
)

type CronTriggerDefinition struct {
	Expression string `json:"expression"`
	Seconds    bool   `json:"seconds"`
}

type IntervalTriggerDefinition struct {
	Interval time.Duration `json:"interval"`
	AnchorAt time.Time     `json:"anchorAt"`
}

type OneShotTriggerDefinition struct {
	RunAt time.Time `json:"runAt"`
}

type ScheduleTriggerDefinition struct {
	Type      TriggerType               `json:"type"`
	Cron      *CronTriggerDefinition    `json:"cron,omitempty"`
	Interval  *IntervalTriggerDefinition `json:"interval,omitempty"`
	OneShot   *OneShotTriggerDefinition  `json:"oneShot,omitempty"`
}

type ScheduleTargetDefinition struct {
	Type            TargetType        `json:"type"`
	TargetID        string            `json:"targetId"`
	InputTemplate   json.RawMessage   `json:"inputTemplate,omitempty"`
	IdempotencyMode IdempotencyMode   `json:"idempotencyMode"`
}

type ScheduleMisfirePolicy struct {
	Policy     MisfirePolicy `json:"policy"`
	MaxCatchUp int           `json:"maxCatchUp"`
}

type ScheduleOverlapPolicy struct {
	Policy OverlapPolicy `json:"policy"`
}

type ScheduleRetryPolicy struct {
	MaxAttempts         int           `json:"maxAttempts"`
	InitialBackoff      time.Duration `json:"initialBackoff"`
	MaxBackoff          time.Duration `json:"maxBackoff"`
	Multiplier          float64       `json:"multiplier"`
	Jitter              float64       `json:"jitter"`
	RetryableErrorCodes []string      `json:"retryableErrorCodes,omitempty"`
}

type ScheduleJitterPolicy struct {
	Enabled  bool          `json:"enabled"`
	MaxDelay time.Duration `json:"maxDelay"`
	SeedMode string        `json:"seedMode"`
}

type ScheduleConcurrencyPolicy struct {
	MaxConcurrentRuns int `json:"maxConcurrentRuns"`
	PerExtensionLimit int `json:"perExtensionLimit"`
	PerTargetLimit    int `json:"perTargetLimit"`
}

type PermissionRequirement struct {
	Permission string          `json:"permission"`
	Reason     string          `json:"reason,omitempty"`
	Required   bool            `json:"required"`
	Scope      json.RawMessage `json:"scope,omitempty"`
}

type ScopeRule struct {
	ScopeType  string   `json:"scopeType"`
	ScopeIDs   []string `json:"scopeIds,omitempty"`
	Namespaces []string `json:"namespaces,omitempty"`
}

type DependencyRequirement struct {
	Type     string `json:"type"`
	ID       string `json:"id"`
	Optional bool   `json:"optional"`
}

type ScheduleContributionDefinition struct {
	ContributionID       string                      `json:"contributionId"`
	ExtensionID          string                      `json:"extensionId"`
	ModuleID             string                      `json:"moduleId"`
	ScheduleID           string                      `json:"scheduleId"`
	Name                 string                      `json:"name"`
	Description          string                      `json:"description"`
	Trigger              ScheduleTriggerDefinition   `json:"trigger"`
	Target               ScheduleTargetDefinition    `json:"target"`
	Timezone             string                      `json:"timezone"`
	StartAt              *time.Time                  `json:"startAt,omitempty"`
	EndAt                *time.Time                  `json:"endAt,omitempty"`
	EnabledByDefault     bool                        `json:"enabledByDefault"`
	MisfirePolicy        ScheduleMisfirePolicy       `json:"misfirePolicy"`
	OverlapPolicy        ScheduleOverlapPolicy       `json:"overlapPolicy"`
	RetryPolicy          ScheduleRetryPolicy         `json:"retryPolicy"`
	JitterPolicy         ScheduleJitterPolicy        `json:"jitterPolicy"`
	ConcurrencyPolicy    ScheduleConcurrencyPolicy   `json:"concurrencyPolicy"`
	PermissionRequirements []PermissionRequirement   `json:"permissionRequirements,omitempty"`
	ScopeRule            ScopeRule                   `json:"scopeRule"`
	DependencyRequirements []DependencyRequirement   `json:"dependencyRequirements,omitempty"`
	DSTSpringPolicy      DSTSpringPolicy             `json:"dstSpringPolicy,omitempty"`
	DSTFallPolicy        DSTFallPolicy               `json:"dstFallPolicy,omitempty"`
	DefinitionHash       string                      `json:"definitionHash"`
	Version              string                      `json:"version"`
}

type ScheduleState struct {
	ScheduleID       string                `json:"scheduleId"`
	Enabled          bool                  `json:"enabled"`
	Paused           bool                  `json:"paused"`
	Status           ScheduleDefinitionStatus `json:"status"`
	LastScheduledAt  *time.Time            `json:"lastScheduledAt,omitempty"`
	LastTriggeredAt  *time.Time            `json:"lastTriggeredAt,omitempty"`
	LastFinishedAt   *time.Time            `json:"lastFinishedAt,omitempty"`
	NextScheduledAt  *time.Time            `json:"nextScheduledAt,omitempty"`
	NextEffectiveAt  *time.Time            `json:"nextEffectiveAt,omitempty"`
	LastResult       string                `json:"lastResult,omitempty"`
	FailureCount     int                   `json:"failureCount"`
	Generation       int64                 `json:"generation"`
	UpdatedAt        time.Time             `json:"updatedAt"`
}

type ScheduleTriggerRecord struct {
	TriggerID            string                 `json:"triggerId"`
	ScheduleID           string                 `json:"scheduleId"`
	ScheduledAt          time.Time              `json:"scheduledAt"`
	EffectiveAt          time.Time              `json:"effectiveAt"`
	TriggeredAt          *time.Time             `json:"triggeredAt,omitempty"`
	IdempotencyKey       string                 `json:"idempotencyKey"`
	Status               ScheduleRunStatus      `json:"status"`
	LeaseOwner           *string                `json:"leaseOwner,omitempty"`
	LeaseExpiresAt       *time.Time             `json:"leaseExpiresAt,omitempty"`
	ScopeSnapshotID      string                 `json:"scopeSnapshotId,omitempty"`
	PermissionSnapshotID string                 `json:"permissionSnapshotId,omitempty"`
	DependencySnapshotID string                 `json:"dependencySnapshotId,omitempty"`
	OperationID          *string                `json:"operationId,omitempty"`
	InvocationID         *string                `json:"invocationId,omitempty"`
	Attempt              int                    `json:"attempt"`
	Generation           int64                  `json:"generation"`
	Manual               bool                   `json:"manual"`
	ErrorCode            *string                `json:"errorCode,omitempty"`
	ErrorMessage         *string                `json:"errorMessage,omitempty"`
	JitterApplied        time.Duration          `json:"jitterApplied,omitempty"`
	MisfireDecision      string                 `json:"misfireDecision,omitempty"`
	OverlapDecision      string                 `json:"overlapDecision,omitempty"`
	DSTDecision          string                 `json:"dstDecision,omitempty"`
	CreatedAt            time.Time              `json:"createdAt"`
	UpdatedAt            time.Time              `json:"updatedAt"`
}

type ScheduleRunRecord struct {
	RunID           string          `json:"runId"`
	TriggerID       string          `json:"triggerId"`
	ScheduleID      string          `json:"scheduleId"`
	Status          ScheduleRunStatus `json:"status"`
	Attempt         int             `json:"attempt"`
	StartedAt       time.Time       `json:"startedAt"`
	FinishedAt      *time.Time      `json:"finishedAt,omitempty"`
	OperationID     string          `json:"operationId,omitempty"`
	InvocationID    string          `json:"invocationId,omitempty"`
	TargetType      TargetType      `json:"targetType"`
	TargetID        string          `json:"targetId"`
	ResultJSON      json.RawMessage `json:"resultJson,omitempty"`
	ErrorCode       *string         `json:"errorCode,omitempty"`
	ErrorMessage    *string         `json:"errorMessage,omitempty"`
	Generation      int64           `json:"generation"`
	CreatedAt       time.Time       `json:"createdAt"`
	UpdatedAt       time.Time       `json:"updatedAt"`
}

type ScheduleLease struct {
	LeaseID      string    `json:"leaseId"`
	TriggerID    string    `json:"triggerId"`
	ScheduleID   string    `json:"scheduleId"`
	LeaseOwner   string    `json:"leaseOwner"`
	AcquiredAt   time.Time `json:"acquiredAt"`
	ExpiresAt    time.Time `json:"expiresAt"`
	Released     bool      `json:"released"`
	ReleasedAt   *time.Time `json:"releasedAt,omitempty"`
}

type ScheduleMisfireRecord struct {
	MisfireID    string        `json:"misfireId"`
	ScheduleID   string        `json:"scheduleId"`
	ScheduledAt  time.Time     `json:"scheduledAt"`
	DetectedAt   time.Time     `json:"detectedAt"`
	Policy       MisfirePolicy `json:"policy"`
	Action       string        `json:"action"`
	SkippedCount int           `json:"skippedCount"`
	Detail       string        `json:"detail,omitempty"`
}

type ScheduleRetryRecord struct {
	RetryID      string        `json:"retryId"`
	TriggerID    string        `json:"triggerId"`
	ScheduleID   string        `json:"scheduleId"`
	Attempt      int           `json:"attempt"`
	MaxAttempts  int           `json:"maxAttempts"`
	ErrorCode    string        `json:"errorCode"`
	Backoff      time.Duration `json:"backoff"`
	AvailableAt  time.Time     `json:"availableAt"`
	CreatedAt    time.Time     `json:"createdAt"`
}

type ScheduleCircuitRecord struct {
	ScheduleID       string        `json:"scheduleId"`
	State            CircuitState  `json:"state"`
	ConsecutiveFails int           `json:"consecutiveFails"`
	TotalFails       int           `json:"totalFails"`
	TotalSuccess     int           `json:"totalSuccess"`
	LastFailCode     *string       `json:"lastFailCode,omitempty"`
	LastFailTime     *time.Time    `json:"lastFailTime,omitempty"`
	OpenedAt         *time.Time    `json:"openedAt,omitempty"`
	UpdatedAt        time.Time     `json:"updatedAt"`
}

type ScheduleQuarantineRecord struct {
	QuarantineID string           `json:"quarantineId"`
	ScheduleID   string           `json:"scheduleId"`
	Reason       QuarantineReason `json:"reason"`
	Detail       string           `json:"detail"`
	QuarantinedAt time.Time       `json:"quarantinedAt"`
	ReleasedAt   *time.Time       `json:"releasedAt,omitempty"`
}

type NextRunResult struct {
	NextScheduledAt *time.Time
	NextEffectiveAt *time.Time
	CalculationReason string
	DSTDecision       string
}

type ScanResult struct {
	DueTriggers []string
	TotalScanned int
	LeaseAcquired int
}

type ExecuteResult struct {
	TriggerID   string
	Status      ScheduleRunStatus
	OperationID string
	InvocationID string
	ErrorCode   string
	ErrorMessage string
}

type ScheduleConfig struct {
	ScanInterval          time.Duration
	ScanBatchSize         int
	GlobalMaxConcurrent   int
	PerExtensionLimit     int
	PerScheduleLimit      int
	MaxCatchUp            int
	MaxRetryAttempts      int
	LeaseDuration         time.Duration
	LeaseReclaimInterval  time.Duration
	CircuitFailThreshold  int
	CircuitRecoveryAfter  time.Duration
	DefaultTimezone       string
}

func DefaultScheduleConfig() ScheduleConfig {
	return ScheduleConfig{
		ScanInterval:         10 * time.Second,
		ScanBatchSize:        500,
		GlobalMaxConcurrent:  16,
		PerExtensionLimit:    4,
		PerScheduleLimit:     1,
		MaxCatchUp:           3,
		MaxRetryAttempts:     5,
		LeaseDuration:        2 * time.Minute,
		LeaseReclaimInterval: 30 * time.Second,
		CircuitFailThreshold: 5,
		CircuitRecoveryAfter: 5 * time.Minute,
		DefaultTimezone:      "UTC",
	}
}

func DefaultMisfirePolicy() ScheduleMisfirePolicy {
	return ScheduleMisfirePolicy{
		Policy:     MisfirePolicyFireOnce,
		MaxCatchUp: 3,
	}
}

func DefaultOverlapPolicy() ScheduleOverlapPolicy {
	return ScheduleOverlapPolicy{
		Policy: OverlapPolicyForbid,
	}
}

func DefaultRetryPolicy() ScheduleRetryPolicy {
	return ScheduleRetryPolicy{
		MaxAttempts:    3,
		InitialBackoff: 5 * time.Second,
		MaxBackoff:     5 * time.Minute,
		Multiplier:     2.0,
		Jitter:         0.1,
	}
}

func DefaultJitterPolicy() ScheduleJitterPolicy {
	return ScheduleJitterPolicy{
		Enabled:  true,
		MaxDelay: 30 * time.Second,
		SeedMode: "schedule_id_scheduled_at",
	}
}

func DefaultConcurrencyPolicy() ScheduleConcurrencyPolicy {
	return ScheduleConcurrencyPolicy{
		MaxConcurrentRuns: 1,
		PerExtensionLimit: 4,
		PerTargetLimit:    1,
	}
}

func DefaultDSTSpringPolicy() DSTSpringPolicy {
	return DSTSpringSkip
}

func DefaultDSTFallPolicy() DSTFallPolicy {
	return DSTFallFireOnceFirst
}

var validDefinitionTransitions = map[ScheduleDefinitionStatus][]ScheduleDefinitionStatus{
	DefinitionStatusCreated:     {DefinitionStatusEnabled, DefinitionStatusDisabled, DefinitionStatusUninstalled},
	DefinitionStatusEnabled:     {DefinitionStatusDisabled, DefinitionStatusPaused, DefinitionStatusExpired, DefinitionStatusUninstalled},
	DefinitionStatusDisabled:    {DefinitionStatusEnabled, DefinitionStatusUninstalled},
	DefinitionStatusPaused:      {DefinitionStatusEnabled, DefinitionStatusDisabled, DefinitionStatusUninstalled},
	DefinitionStatusExpired:     {DefinitionStatusUninstalled},
}

func IsValidDefinitionTransition(from, to ScheduleDefinitionStatus) bool {
	if from == to {
		return true
	}
	allowed, ok := validDefinitionTransitions[from]
	if !ok {
		return false
	}
	for _, s := range allowed {
		if s == to {
			return true
		}
	}
	return false
}
