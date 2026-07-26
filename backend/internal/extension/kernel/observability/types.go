package observability

import "time"

type Trace struct {
	TraceID     string            `json:"traceId"`
	RootOpID    string            `json:"rootOpId,omitempty"`
	CreatedAt   time.Time         `json:"createdAt"`
	Metadata    map[string]any    `json:"metadata,omitempty"`
}

type OperationRecord struct {
	OperationID string          `json:"operationId"`
	TraceID     string          `json:"traceId"`
	ParentOpID  string          `json:"parentOpId,omitempty"`
	Type        OperationType   `json:"type"`
	ActorType   ActorType       `json:"actorType"`
	ActorID     string          `json:"actorId"`
	SubjectType SubjectType     `json:"subjectType"`
	SubjectID   string          `json:"subjectId"`
	Status      ExecutionStatus `json:"status"`
	RiskLevel   RiskLevel       `json:"riskLevel,omitempty"`
	StartedAt   time.Time       `json:"startedAt"`
	FinishedAt  *time.Time      `json:"finishedAt,omitempty"`
	Summary     string          `json:"summary,omitempty"`
	ErrorCode   string          `json:"errorCode,omitempty"`
	Outcome     AuditOutcome    `json:"outcome,omitempty"`
	Metadata    map[string]any  `json:"metadata,omitempty"`
	CreatedAt   time.Time       `json:"createdAt"`
}

type InvocationRecord struct {
	InvocationID    string          `json:"invocationId"`
	TraceID         string          `json:"traceId"`
	OperationID     string          `json:"operationId"`
	ParentID        string          `json:"parentId,omitempty"`
	RootID          string          `json:"rootId,omitempty"`

	CapabilityID    string          `json:"capabilityId,omitempty"`
	CapabilityType  string          `json:"capabilityType,omitempty"`
	Source          string          `json:"source,omitempty"`
	OwnerType       string          `json:"ownerType,omitempty"`
	OwnerID         string          `json:"ownerId,omitempty"`
	ExtensionID     string          `json:"extensionId,omitempty"`
	ModuleID        string          `json:"moduleId,omitempty"`
	RuntimeType     string          `json:"runtimeType,omitempty"`
	RuntimeID       string          `json:"runtimeId,omitempty"`

	UserID          string          `json:"userId,omitempty"`
	CharacterID     string          `json:"characterId,omitempty"`
	ConversationID  string          `json:"conversationId,omitempty"`
	ScopeSnapshotID string          `json:"scopeSnapshotId,omitempty"`

	Status          ExecutionStatus `json:"status"`
	RiskLevel       RiskLevel       `json:"riskLevel,omitempty"`
	ApprovalMode    ApprovalMode    `json:"approvalMode,omitempty"`

	InputHash       string          `json:"inputHash,omitempty"`
	OutputHash      string          `json:"outputHash,omitempty"`
	InputSummary    string          `json:"inputSummary,omitempty"`
	OutputSummary   string          `json:"outputSummary,omitempty"`
	ErrorCode       string          `json:"errorCode,omitempty"`
	ErrorSummary    string          `json:"errorSummary,omitempty"`

	RetryCount      int             `json:"retryCount"`
	SideEffectCount int             `json:"sideEffectCount"`

	CreatedAt       time.Time       `json:"createdAt"`
	QueuedAt        *time.Time      `json:"queuedAt,omitempty"`
	StartedAt       *time.Time      `json:"startedAt,omitempty"`
	FinishedAt      *time.Time      `json:"finishedAt,omitempty"`
	DurationMs      int64           `json:"durationMs"`

	Metadata        map[string]any  `json:"metadata,omitempty"`
}

type ExecutionAttempt struct {
	AttemptID      string         `json:"attemptId"`
	InvocationID   string         `json:"invocationId"`
	AttemptNumber  int            `json:"attemptNumber"`
	RuntimeType    string         `json:"runtimeType,omitempty"`
	RuntimeID      string         `json:"runtimeId,omitempty"`
	Status         ExecutionStatus `json:"status"`
	StartedAt      time.Time      `json:"startedAt"`
	FinishedAt     *time.Time     `json:"finishedAt,omitempty"`
	DurationMs     int64          `json:"durationMs"`
	ErrorCode      string         `json:"errorCode,omitempty"`
	Retryable      bool           `json:"retryable"`
	BackoffMs      int64          `json:"backoffMs"`
	ResourceUsage  map[string]any `json:"resourceUsage,omitempty"`
	Metadata       map[string]any `json:"metadata,omitempty"`
}

type RuntimeEventRecord struct {
	EventID      string         `json:"eventId"`
	TraceID      string         `json:"traceId"`
	OperationID  string         `json:"operationId,omitempty"`
	InvocationID string         `json:"invocationId,omitempty"`
	AttemptID    string         `json:"attemptId,omitempty"`
	EventType    string         `json:"eventType"`
	Severity     string         `json:"severity"`
	Timestamp    time.Time      `json:"timestamp"`
	Data         map[string]any `json:"data,omitempty"`
}

type AuditEvent struct {
	AuditID       string         `json:"auditId"`
	TraceID       string         `json:"traceId"`
	OperationID   string         `json:"operationId,omitempty"`
	InvocationID  string         `json:"invocationId,omitempty"`

	ActorType     ActorType      `json:"actorType"`
	ActorID       string         `json:"actorId"`
	SubjectType   SubjectType    `json:"subjectType"`
	SubjectID     string         `json:"subjectId"`

	Action        string         `json:"action"`
	Decision      string         `json:"decision"`
	RiskLevel     string         `json:"riskLevel,omitempty"`
	ScopeSummary  string         `json:"scopeSummary,omitempty"`
	PermissionIDs []string       `json:"permissionIds,omitempty"`

	TargetType    string         `json:"targetType,omitempty"`
	TargetID      string         `json:"targetId,omitempty"`

	Result        string         `json:"result"`
	ErrorCode     string         `json:"errorCode,omitempty"`

	GrantID       string         `json:"grantId,omitempty"`
	ApprovalID    string         `json:"approvalId,omitempty"`
	SnapshotID    string         `json:"snapshotId,omitempty"`

	CreatedAt     time.Time      `json:"createdAt"`
	Metadata      map[string]any `json:"metadata,omitempty"`
}

type ErrorRecord struct {
	ErrorID           string    `json:"errorId"`
	InvocationID      string    `json:"invocationId"`
	AttemptID         string    `json:"attemptId,omitempty"`
	Code              string    `json:"code"`
	Category          string    `json:"category"`
	Retryable         bool      `json:"retryable"`
	UserVisible       bool      `json:"userVisible"`
	SanitizedMessage  string    `json:"sanitizedMessage"`
	InternalReference string    `json:"internalReference,omitempty"`
	CreatedAt         time.Time `json:"createdAt"`
}
