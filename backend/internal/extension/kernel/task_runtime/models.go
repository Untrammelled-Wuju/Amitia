package task_runtime

import (
	"encoding/json"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/permission"
	"github.com/u-ai/backend/internal/extension/kernel/script_host"
)

type TaskIdempotency string

const (
	Idempotent              TaskIdempotency = "idempotent"
	ConditionallyIdempotent TaskIdempotency = "conditionally_idempotent"
	NonIdempotent           TaskIdempotency = "non_idempotent"
)

type TaskRecoverability string

const (
	NotRecoverable           TaskRecoverability = "not_recoverable"
	CheckpointRecoverable    TaskRecoverability = "checkpoint_recoverable"
	RestartableFromBeginning TaskRecoverability = "restartable_from_beginning"
	ManualRecovery           TaskRecoverability = "manual_recovery"
)

type TaskResultPolicy string

const (
	ResultInlineJSON TaskResultPolicy = "inline_json"
	ResultArtifact   TaskResultPolicy = "artifact"
	ResultAuto       TaskResultPolicy = "auto"
)

type TaskCleanupPolicy string

const (
	CleanupAlways         TaskCleanupPolicy = "always"
	CleanupOnSuccess      TaskCleanupPolicy = "on_success"
	CleanupOnFailure      TaskCleanupPolicy = "on_failure"
	CleanupRetainForDebug TaskCleanupPolicy = "retain_for_debug"
)

type TaskRuntimeType string

const (
	RuntimeTaskJavaScript TaskRuntimeType = "task_javascript"
)

type PermissionRequirement = permission.PermissionRequirement

type ScopeRule struct {
	ScopeType  string   `json:"scopeType"`
	ScopeIDs   []string `json:"scopeIds,omitempty"`
	Namespaces []string `json:"namespaces,omitempty"`
}

type TaskRetryPolicy struct {
	MaxAttempts         int           `json:"maxAttempts"`
	InitialBackoff      time.Duration `json:"initialBackoff"`
	MaxBackoff          time.Duration `json:"maxBackoff"`
	Multiplier          float64       `json:"multiplier"`
	RetryableErrorCodes []string      `json:"retryableErrorCodes,omitempty"`
}

func DefaultRetryPolicy() TaskRetryPolicy {
	return TaskRetryPolicy{
		MaxAttempts:    3,
		InitialBackoff: 5 * time.Second,
		MaxBackoff:     5 * time.Minute,
		Multiplier:     2.0,
	}
}

type TaskTimeoutPolicy struct {
	DefaultTimeout time.Duration `json:"defaultTimeout"`
	MaxTimeout     time.Duration `json:"maxTimeout"`
	HardKillAfter  time.Duration `json:"hardKillAfter"`
}

func DefaultTimeoutPolicy() TaskTimeoutPolicy {
	return TaskTimeoutPolicy{
		DefaultTimeout: 30 * time.Minute,
		MaxTimeout:     24 * time.Hour,
		HardKillAfter:  30 * time.Second,
	}
}

type TaskRunStatus string

const (
	RunStatusCreated            TaskRunStatus = "created"
	RunStatusQueued             TaskRunStatus = "queued"
	RunStatusStarting           TaskRunStatus = "starting"
	RunStatusRunning            TaskRunStatus = "running"
	RunStatusCheckpointing      TaskRunStatus = "checkpointing"
	RunStatusPausing            TaskRunStatus = "pausing"
	RunStatusPaused             TaskRunStatus = "paused"
	RunStatusResuming           TaskRunStatus = "resuming"
	RunStatusCancelling         TaskRunStatus = "cancelling"
	RunStatusCancelled          TaskRunStatus = "cancelled"
	RunStatusSucceeded          TaskRunStatus = "succeeded"
	RunStatusFailed             TaskRunStatus = "failed"
	RunStatusTimedOut           TaskRunStatus = "timed_out"
	RunStatusRecoveryRequired   TaskRunStatus = "recovery_required"
	RunStatusManualIntervention TaskRunStatus = "manual_intervention"
)

func (s TaskRunStatus) IsTerminal() bool {
	switch s {
	case RunStatusSucceeded, RunStatusFailed, RunStatusCancelled,
		RunStatusTimedOut, RunStatusManualIntervention:
		return true
	}
	return false
}

func (s TaskRunStatus) IsActive() bool {
	switch s {
	case RunStatusCreated, RunStatusQueued, RunStatusStarting, RunStatusRunning,
		RunStatusCheckpointing, RunStatusPausing, RunStatusPaused,
		RunStatusResuming, RunStatusCancelling, RunStatusRecoveryRequired:
		return true
	}
	return false
}

var validTransitions = map[TaskRunStatus][]TaskRunStatus{
	RunStatusCreated:          {RunStatusQueued, RunStatusCancelled},
	RunStatusQueued:           {RunStatusStarting, RunStatusCancelled},
	RunStatusStarting:         {RunStatusRunning, RunStatusFailed, RunStatusCancelled, RunStatusRecoveryRequired},
	RunStatusRunning:          {RunStatusCheckpointing, RunStatusPaused, RunStatusCancelling, RunStatusSucceeded, RunStatusFailed, RunStatusTimedOut, RunStatusRecoveryRequired},
	RunStatusCheckpointing:    {RunStatusRunning, RunStatusPaused, RunStatusFailed, RunStatusRecoveryRequired},
	RunStatusPaused:           {RunStatusRunning, RunStatusCancelling, RunStatusRecoveryRequired},
	RunStatusCancelling:       {RunStatusCancelled, RunStatusFailed},
	RunStatusRecoveryRequired: {RunStatusStarting, RunStatusManualIntervention, RunStatusCancelled},
}

func IsValidTransition(from, to TaskRunStatus) bool {
	if from == to {
		return true
	}
	allowed, ok := validTransitions[from]
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

func MustTransition(from, to TaskRunStatus) error {
	if !IsValidTransition(from, to) {
		return NewTaskError(ErrTaskStateTransitionInvalid,
			"invalid transition from "+string(from)+" to "+string(to))
	}
	return nil
}

// TaskRun 是所有本地、Cloud、Device 分布式执行任务的唯一生命周期实体。
// 未来只能扩展 execution placement / assigned provider / device 等字段。
// 禁止创建平行 RemoteTask、CloudTask、DeviceTask 状态机。
type TaskRun struct {
	TaskRunID            string                 `json:"taskRunId"`
	OperationID          string                 `json:"operationId"`
	InvocationID         string                 `json:"invocationId"`
	TaskDefinitionID     string                 `json:"taskDefinitionId"`
	ExtensionID          string                 `json:"extensionId"`
	ModuleID             string                 `json:"moduleId"`
	Status               TaskRunStatus          `json:"status"`
	Priority             int                    `json:"priority"`
	ExecutionPlacement   TaskExecutionPlacement `json:"executionPlacement,omitempty"`
	ExecutionTarget      TaskExecutionTarget    `json:"executionTarget,omitempty"`
	ExecutionAttemptID   TaskExecutionAttemptID `json:"executionAttemptId,omitempty"`
	ExecutionResolvedAt  *time.Time             `json:"executionResolvedAt,omitempty"`
	ExecutionResolvedBy  string                 `json:"executionResolvedBy,omitempty"`
	Input                json.RawMessage        `json:"input"`
	InputHash            string                 `json:"inputHash"`
	InputArtifactID      *string                `json:"inputArtifactId,omitempty"`
	TraceID              string                 `json:"traceId,omitempty"`
	CorrelationID        string                 `json:"correlationId,omitempty"`
	CausationID          string                 `json:"causationId,omitempty"`
	Source               string                 `json:"source,omitempty"`
	ScopeSnapshotID      string                 `json:"scopeSnapshotId,omitempty"`
	PermissionSnapshotID string                 `json:"permissionSnapshotId,omitempty"`
	DependencySnapshotID string                 `json:"dependencySnapshotId,omitempty"`
	RuntimeInstanceID    *string                `json:"runtimeInstanceId,omitempty"`
	CheckpointID         *string                `json:"checkpointId,omitempty"`
	ResultArtifactID     *string                `json:"resultArtifactId,omitempty"`
	Attempt              int                    `json:"attempt"`
	MaxAttempts          int                    `json:"maxAttempts"`
	CreatedAt            time.Time              `json:"createdAt"`
	QueuedAt             *time.Time             `json:"queuedAt,omitempty"`
	StartedAt            *time.Time             `json:"startedAt,omitempty"`
	FinishedAt           *time.Time             `json:"finishedAt,omitempty"`
	DeadlineAt           *time.Time             `json:"deadlineAt,omitempty"`
	CancelRequestedAt    *time.Time             `json:"cancelRequestedAt,omitempty"`
	PauseReason          *string                `json:"pauseReason,omitempty"`
	PauseRequestedAt     *time.Time             `json:"pauseRequestedAt,omitempty"`
	PausedAt             *time.Time             `json:"pausedAt,omitempty"`
	ResumedAt            *time.Time             `json:"resumedAt,omitempty"`
	ErrorCode            *string                `json:"errorCode,omitempty"`
	ErrorMessage         *string                `json:"errorMessage,omitempty"`
	Generation           int64                  `json:"generation"`
	Revision             int64                  `json:"revision"`
}

func (r *TaskRun) EffectiveExecutionPlacement() TaskExecutionPlacement {
	if r == nil {
		return TaskExecutionPlacementLocal
	}
	return r.ExecutionPlacement.Normalize()
}

func (r *TaskRun) IsRemoteExecution() bool {
	return r.EffectiveExecutionPlacement().IsRemote()
}

func (r *TaskRun) HasResolvedExecutionTarget() bool {
	if r == nil {
		return false
	}
	placement := r.EffectiveExecutionPlacement()
	if err := ValidateResolvedTaskExecutionTarget(placement, r.ExecutionTarget); err != nil {
		return false
	}
	return true
}

func (r *TaskRun) BindExecutionTarget(
	decision TaskPlacementDecision,
	resolvedBy string,
	at time.Time,
) error {
	if !decision.Resolved {
		return NewTaskError(ErrTaskExecutionTargetUnresolved, "placement decision is not resolved")
	}

	if err := ValidateTaskExecutionTargetShape(r.ExecutionPlacement, decision.Target); err != nil {
		return err
	}

	if err := ValidateResolvedTaskExecutionTarget(r.ExecutionPlacement, decision.Target); err != nil {
		return err
	}

	if r.ExecutionResolvedAt != nil {
		if !r.ExecutionTarget.StableEqual(decision.Target) {
			return NewTaskError(ErrTaskExecutionTargetConflict, "stable execution target already bound; re-placement not supported in G13")
		}
		r.ExecutionResolvedAt = &at
		r.ExecutionResolvedBy = resolvedBy
		return nil
	}

	r.ExecutionPlacement = decision.Placement
	r.ExecutionTarget = decision.Target.Normalize()
	r.ExecutionResolvedAt = &at
	r.ExecutionResolvedBy = resolvedBy
	return nil
}

func (r *TaskRun) ClearTransientConnectionBinding() {
	if r == nil {
		return
	}
	r.ExecutionTarget.RuntimeSessionID = ""
	r.ExecutionTarget.ConnectionGeneration = 0
}

func cloneTaskRun(run *TaskRun) *TaskRun {
	if run == nil {
		return nil
	}
	clone := *run
	if run.Input != nil {
		clone.Input = append([]byte(nil), run.Input...)
	}
	if run.ExecutionResolvedAt != nil {
		t := *run.ExecutionResolvedAt
		clone.ExecutionResolvedAt = &t
	}
	if run.QueuedAt != nil {
		t := *run.QueuedAt
		clone.QueuedAt = &t
	}
	if run.StartedAt != nil {
		t := *run.StartedAt
		clone.StartedAt = &t
	}
	if run.FinishedAt != nil {
		t := *run.FinishedAt
		clone.FinishedAt = &t
	}
	if run.DeadlineAt != nil {
		t := *run.DeadlineAt
		clone.DeadlineAt = &t
	}
	if run.CancelRequestedAt != nil {
		t := *run.CancelRequestedAt
		clone.CancelRequestedAt = &t
	}
	if run.PauseRequestedAt != nil {
		t := *run.PauseRequestedAt
		clone.PauseRequestedAt = &t
	}
	if run.PausedAt != nil {
		t := *run.PausedAt
		clone.PausedAt = &t
	}
	if run.ResumedAt != nil {
		t := *run.ResumedAt
		clone.ResumedAt = &t
	}
	if run.RuntimeInstanceID != nil {
		s := *run.RuntimeInstanceID
		clone.RuntimeInstanceID = &s
	}
	if run.CheckpointID != nil {
		s := *run.CheckpointID
		clone.CheckpointID = &s
	}
	if run.ResultArtifactID != nil {
		s := *run.ResultArtifactID
		clone.ResultArtifactID = &s
	}
	if run.InputArtifactID != nil {
		s := *run.InputArtifactID
		clone.InputArtifactID = &s
	}
	if run.ErrorCode != nil {
		s := *run.ErrorCode
		clone.ErrorCode = &s
	}
	if run.ErrorMessage != nil {
		s := *run.ErrorMessage
		clone.ErrorMessage = &s
	}
	if run.PauseReason != nil {
		s := *run.PauseReason
		clone.PauseReason = &s
	}
	clone.ExecutionTarget = run.ExecutionTarget
	return &clone
}

func (r *TaskRun) HasStaleDeviceConnection(
	activeSessionID interface{ String() string },
	generation int64,
) bool {
	if r == nil {
		return false
	}
	if r.EffectiveExecutionPlacement() != TaskExecutionPlacementDevice {
		return false
	}
	currentSession := string(r.ExecutionTarget.RuntimeSessionID)
	if currentSession != activeSessionID.String() {
		return true
	}
	return r.ExecutionTarget.ConnectionGeneration != generation
}

type TaskRunProgress struct {
	TaskRunID  string          `json:"taskRunId"`
	Sequence   int64           `json:"sequence"`
	Current    *float64        `json:"current,omitempty"`
	Total      *float64        `json:"total,omitempty"`
	Percentage *float64        `json:"percentage,omitempty"`
	Stage      string          `json:"stage,omitempty"`
	Message    string          `json:"message,omitempty"`
	Details    json.RawMessage `json:"details,omitempty"`
	UpdatedAt  time.Time       `json:"updatedAt"`
}

type TaskCheckpoint struct {
	CheckpointID   string          `json:"checkpointId"`
	TaskRunID      string          `json:"taskRunId"`
	Version        int64           `json:"version"`
	Payload        json.RawMessage `json:"payload"`
	PayloadHash    string          `json:"payloadHash"`
	DefinitionHash string          `json:"definitionHash"`
	InputHash      string          `json:"inputHash"`
	CreatedAt      time.Time       `json:"createdAt"`
}

type TaskRunResult struct {
	TaskRunID  string           `json:"taskRunId"`
	ResultType TaskResultPolicy `json:"resultType"`
	ResultJSON json.RawMessage  `json:"resultJson,omitempty"`
	ArtifactID string           `json:"artifactId,omitempty"`
	ResultHash string           `json:"resultHash,omitempty"`
	CreatedAt  time.Time        `json:"createdAt"`
}

type TaskQueueEntry struct {
	TaskRunID      string     `json:"taskRunId"`
	Priority       int        `json:"priority"`
	AvailableAt    time.Time  `json:"availableAt"`
	LeaseOwner     string     `json:"leaseOwner,omitempty"`
	LeaseExpiresAt *time.Time `json:"leaseExpiresAt,omitempty"`
	CreatedAt      time.Time  `json:"createdAt"`
}

type TaskRuntimeConfig struct {
	GlobalMaxConcurrent        int
	PerExtensionMaxConcurrent  int
	PerDefinitionMaxConcurrent int
	DefaultTimeout             time.Duration
	MaxProgressPerSecond       int
	MaxCheckpointBytes         int
	MaxInlineResultBytes       int
	MaxRetryAttempts           int
	WorkspaceRoot              string
	NodeEnvironmentResolver    script_host.NodeEnvironmentResolver
	HostArtifactResolver       script_host.ArtifactResolver
	LeaseDuration              time.Duration
	CancelGracePeriod          time.Duration
	PauseGracePeriod           time.Duration
}

func DefaultTaskRuntimeConfig() TaskRuntimeConfig {
	return TaskRuntimeConfig{
		GlobalMaxConcurrent:        4,
		PerExtensionMaxConcurrent:  2,
		PerDefinitionMaxConcurrent: 1,
		DefaultTimeout:             30 * time.Minute,
		MaxProgressPerSecond:       5,
		MaxCheckpointBytes:         1 << 20,
		MaxInlineResultBytes:       256 << 10,
		MaxRetryAttempts:           3,
		WorkspaceRoot:              "",
		LeaseDuration:              2 * time.Minute,
		CancelGracePeriod:          10 * time.Second,
		PauseGracePeriod:           30 * time.Second,
	}
}

type EnqueueTaskRequest struct {
	TaskDefinitionID     string                 `json:"taskDefinitionId"`
	ExtensionID          string                 `json:"extensionId"`
	ModuleID             string                 `json:"moduleId"`
	Input                json.RawMessage        `json:"input"`
	Priority             int                    `json:"priority"`
	ExecutionPlacement   TaskExecutionPlacement `json:"executionPlacement,omitempty"`
	OperationID          string                 `json:"operationId"`
	InvocationID         string                 `json:"invocationId,omitempty"`
	TraceID              string                 `json:"traceId,omitempty"`
	CorrelationID        string                 `json:"correlationId,omitempty"`
	CausationID          string                 `json:"causationId,omitempty"`
	Source               string                 `json:"source,omitempty"`
	ScopeSnapshotID      string                 `json:"scopeSnapshotId"`
	PermissionSnapshotID string                 `json:"permissionSnapshotId"`
}

type TrustedExecutionTargetRequest struct {
	Placement  TaskExecutionPlacement `json:"placement"`
	Target     TaskExecutionTarget    `json:"target"`
	ResolvedBy string                 `json:"resolvedBy"`
}

type EnqueueTaskResult struct {
	TaskRunID string        `json:"taskRunId"`
	Status    TaskRunStatus `json:"status"`
	Queued    bool          `json:"queued"`
	Position  int           `json:"position,omitempty"`
}

type ListTasksFilter struct {
	ExtensionID string `json:"extensionId,omitempty"`
	Status      string `json:"status,omitempty"`
	Limit       int    `json:"limit,omitempty"`
	Offset      int    `json:"offset,omitempty"`
}
