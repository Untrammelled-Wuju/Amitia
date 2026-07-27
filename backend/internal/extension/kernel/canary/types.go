package canary

import (
	"time"
)

type CanaryMode string

const (
	CanaryModeShadow    CanaryMode = "shadow"
	CanaryModeCanary    CanaryMode = "canary"
	CanaryModeLimited   CanaryMode = "limited"
	CanaryModeExpanded  CanaryMode = "expanded"
	CanaryModeFull      CanaryMode = "full"
)

type CanaryStageMode string

const (
	StageModeShadow    CanaryStageMode = "shadow"
	StageModeCanary    CanaryStageMode = "canary"
	StageModeLimited   CanaryStageMode = "limited"
	StageModeExpanded  CanaryStageMode = "expanded"
	StageModeFull      CanaryStageMode = "full"
)

type CohortKeyType string

const (
	CohortKeyCharacter     CohortKeyType = "character"
	CohortKeyConversation  CohortKeyType = "conversation"
	CohortKeyInvocation    CohortKeyType = "invocation"
	CohortKeyManualSet     CohortKeyType = "manual_set"
)

type BackgroundType string

const (
	BGSchedule       BackgroundType = "schedule"
	BGEventSub       BackgroundType = "event_subscription"
	BGHook           BackgroundType = "hook"
	BGTask           BackgroundType = "background_task"
	BGMCPConnection  BackgroundType = "mcp_connection"
	BGTrustedService BackgroundType = "trusted_service"
	BGTray           BackgroundType = "tray"
	BGShortcut       BackgroundType = "global_shortcut"
	BGDesktop        BackgroundType = "desktop"
)

type CanaryStage struct {
	StageID          string        `json:"stage_id"`
	Mode             CanaryStageMode `json:"mode"`
	Percentage       int           `json:"percentage"`
	CharacterIDs     []string      `json:"character_ids,omitempty"`
	ConversationIDs  []string      `json:"conversation_ids,omitempty"`
	ContributionIDs  []string      `json:"contribution_ids,omitempty"`
	MinDuration      time.Duration `json:"min_duration"`
	MinInvocations   int           `json:"min_invocations"`
	AutoAdvance      bool          `json:"auto_advance"`
}

type CanaryHealthPolicy struct {
	MaximumErrorRate         float64       `json:"maximum_error_rate"`
	MaximumRelativeErrorRate float64       `json:"maximum_relative_error_rate"`
	MaximumP95Latency        time.Duration `json:"maximum_p95_latency"`
	MaximumLatencyRegression float64       `json:"maximum_latency_regression"`
	MaximumCrashCount        int           `json:"maximum_crash_count"`
	MaximumTimeoutRate       float64       `json:"maximum_timeout_rate"`
	MaximumInvalidResultRate float64       `json:"maximum_invalid_result_rate"`
	RequiredHealthChecks     []string      `json:"required_health_checks"`
}

type CanaryAbortPolicy struct {
	AbortOnSignatureAnomaly     bool `json:"abort_on_signature_anomaly"`
	AbortOnDataValidationFail   bool `json:"abort_on_data_validation_fail"`
	AbortOnErrorRateExceeded    bool `json:"abort_on_error_rate_exceeded"`
	AbortOnCrashExceeded        bool `json:"abort_on_crash_exceeded"`
	AbortOnLatencyRegression    bool `json:"abort_on_latency_regression"`
	AbortOnSideEffectMismatch   bool `json:"abort_on_side_effect_mismatch"`
	AbortOnPermissionEscalation bool `json:"abort_on_permission_escalation"`
	AbortOnScopeViolation       bool `json:"abort_on_scope_violation"`
	AbortOnDataIncompatible     bool `json:"abort_on_data_incompatible"`
	AbortOnBackgroundDoubleRun  bool `json:"abort_on_background_double_run"`
	AbortOnMigrationError       bool `json:"abort_on_migration_error"`
}

type CanaryPolicy struct {
	PolicyID       string              `json:"policy_id"`
	ExtensionID    string              `json:"extension_id"`
	Mode           CanaryMode          `json:"mode"`
	Stages         []CanaryStage       `json:"stages"`
	CohortKey      CohortKeyType       `json:"cohort_key"`
	StableSeed     string              `json:"stable_seed"`
	MinObservations int                `json:"min_observations"`
	MinDuration    time.Duration      `json:"min_duration"`
	MaxDuration    time.Duration      `json:"max_duration"`
	HealthPolicy   CanaryHealthPolicy  `json:"health_policy"`
	AbortPolicy    CanaryAbortPolicy   `json:"abort_policy"`
	WriteStrategy  string              `json:"write_strategy"`
}

type CanaryStatus string

const (
	CanaryStatusCreated   CanaryStatus = "created"
	CanaryStatusShadow    CanaryStatus = "shadow"
	CanaryStatusCanary    CanaryStatus = "canary"
	CanaryStatusLimited   CanaryStatus = "limited"
	CanaryStatusExpanded  CanaryStatus = "expanded"
	CanaryStatusFull      CanaryStatus = "full"
	CanaryStatusCommitting CanaryStatus = "committing"
	CanaryStatusCompleted CanaryStatus = "completed"
	CanaryStatusPaused    CanaryStatus = "paused"
	CanaryStatusAborting  CanaryStatus = "aborting"
	CanaryStatusAborted   CanaryStatus = "aborted"
	CanaryStatusRolledBack CanaryStatus = "rolled_back"
	CanaryStatusFailed    CanaryStatus = "failed"
)

type CanaryState struct {
	CanaryID      string       `json:"canary_id"`
	ExtensionID   string       `json:"extension_id"`
	PolicyID      string       `json:"policy_id"`
	OldGeneration int64        `json:"old_generation"`
	NewGeneration int64        `json:"new_generation"`
	Status        CanaryStatus `json:"status"`
	CurrentStage  int          `json:"current_stage"`
	StartedAt     time.Time    `json:"started_at"`
	UpdatedAt     time.Time    `json:"updated_at"`
	PausedAt      *time.Time   `json:"paused_at,omitempty"`
	FinishedAt    *time.Time   `json:"finished_at,omitempty"`
	AbortReason   string       `json:"abort_reason,omitempty"`
}

type CohortAssignment struct {
	AssignmentID    string       `json:"assignment_id"`
	ExtensionID     string       `json:"extension_id"`
	ContributionID  string       `json:"contribution_id"`
	CohortType      CohortKeyType `json:"cohort_type"`
	CohortID        string       `json:"cohort_id"`
	Generation      int64        `json:"generation"`
	StageID         string       `json:"stage_id"`
	AssignedAt      time.Time    `json:"assigned_at"`
	AssignmentHash  string       `json:"assignment_hash"`
}

type MetricName string

const (
	MetricStartupSuccess     MetricName = "startup_success_rate"
	MetricRuntimeCrash       MetricName = "runtime_crash_count"
	MetricInvocationSuccess  MetricName = "invocation_success_rate"
	MetricErrorRate          MetricName = "error_rate"
	MetricTimeout            MetricName = "timeout_rate"
	MetricP50Latency         MetricName = "p50_latency_ms"
	MetricP95Latency         MetricName = "p95_latency_ms"
	MetricP99Latency         MetricName = "p99_latency_ms"
	MetricMemoryMB           MetricName = "memory_mb"
	MetricCPUPercent         MetricName = "cpu_percent"
	MetricHostAPIError       MetricName = "host_api_error_count"
	MetricPermissionDenied   MetricName = "permission_denied_count"
	MetricScopeDenied        MetricName = "scope_denied_count"
	MetricInvalidResult      MetricName = "invalid_result_rate"
	MetricSideEffectFailure  MetricName = "side_effect_failure_count"
	MetricUILoadFailure      MetricName = "ui_load_failure_count"
	MetricHookTimeout        MetricName = "hook_timeout_count"
	MetricEventDeadLetter    MetricName = "event_dead_letter_count"
	MetricScheduleFailure    MetricName = "schedule_failure_count"
	MetricTaskFailure        MetricName = "task_failure_count"
)

type MetricStatus string

const (
	MetricStatusNormal    MetricStatus = "normal"
	MetricStatusWarning   MetricStatus = "warning"
	MetricStatusCritical  MetricStatus = "critical"
	MetricStatusBaseline  MetricStatus = "baseline"
)

type CanaryMetric struct {
	ExtensionID    string       `json:"extension_id"`
	Generation     int64        `json:"generation"`
	StageID        string       `json:"stage_id"`
	MetricName     MetricName   `json:"metric_name"`
	MetricValue    float64      `json:"metric_value"`
	SampleCount    int64        `json:"sample_count"`
	WindowStart    time.Time    `json:"window_start"`
	WindowEnd      time.Time    `json:"window_end"`
	BaselineValue  float64      `json:"baseline_value"`
	Status         MetricStatus `json:"status"`
}

type RoutingReason string

const (
	RoutingReasonStableCohort  RoutingReason = "stable_cohort"
	RoutingReasonPercentage    RoutingReason = "percentage"
	RoutingReasonManualSet     RoutingReason = "manual_set"
	RoutingReasonBackground    RoutingReason = "background_ownership"
	RoutingReasonFallback      RoutingReason = "fallback"
	RoutingReasonShadow        RoutingReason = "shadow"
)

type GenerationRoute struct {
	ExtensionID    string        `json:"extension_id"`
	ContributionID string        `json:"contribution_id"`
	CohortType     CohortKeyType `json:"cohort_type"`
	CohortID       string        `json:"cohort_id"`
	Generation     int64         `json:"generation"`
	StageID        string        `json:"stage_id"`
	Reason         RoutingReason `json:"reason"`
	AssignedAt     time.Time     `json:"assigned_at"`
}

type BackgroundOwnership struct {
	ExtensionID    string         `json:"extension_id"`
	BGType         BackgroundType `json:"bg_type"`
	ResourceID     string         `json:"resource_id"`
	OwnerGeneration int64         `json:"owner_generation"`
	AcquiredAt     time.Time      `json:"acquired_at"`
	LeaseExpiresAt *time.Time     `json:"lease_expires_at,omitempty"`
}

type AutoAdvanceResult struct {
	CanAdvance    bool     `json:"can_advance"`
	Reason        string   `json:"reason"`
	Blockers      []string `json:"blockers,omitempty"`
	NextStage     string   `json:"next_stage,omitempty"`
}

type AutoAbortResult struct {
	ShouldAbort bool     `json:"should_abort"`
	Reason      string   `json:"reason"`
	Trigger     string   `json:"trigger"`
	Details     []string `json:"details,omitempty"`
}

type ShadowConstraint struct {
	AllowedSideEffects []string `json:"allowed_side_effects"`
	ForbiddenActions   []string `json:"forbidden_actions"`
}

var DefaultShadowConstraint = ShadowConstraint{
	AllowedSideEffects: []string{"none", "read_only"},
	ForbiddenActions: []string{
		"message_send", "file_write", "memory_write",
		"network_write", "payment", "desktop_action",
		"schedule", "event_side_effect", "trusted_service_listen",
	},
}

type DualWritePolicy struct {
	RequiredIdempotent   bool                   `json:"required_idempotent"`
	RecordBothSides      bool                   `json:"record_both_sides"`
	ValidateConsistency  bool                   `json:"validate_consistency"`
	CompensationStrategy string                 `json:"compensation_strategy"`
	ConflictResolution   string                 `json:"conflict_resolution"`
	ExternalSideEffect   bool                   `json:"external_side_effect"`
}

type ReadCompatibility struct {
	OldReadsNewData  bool `json:"old_reads_new_data"`
	NewReadsOldData  bool `json:"new_reads_old_data"`
	SharedSchema     bool `json:"shared_schema"`
	VersionMarkerOK  bool `json:"version_marker_ok"`
}

type InvocationContext struct {
	ExtensionID    string
	ContributionID string
	CharacterID    string
	ConversationID string
	InvocationID   string
	IsBackground   bool
	BGType         BackgroundType
}
