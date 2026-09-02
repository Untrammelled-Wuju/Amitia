package capability

import (
	"time"

	"github.com/google/uuid"

	"github.com/u-ai/backend/internal/execution"
	"github.com/u-ai/backend/internal/runtimeidentity"
)

type InvocationSource string

const (
	InvocationSourceModel         InvocationSource = "model"
	InvocationSourceUser          InvocationSource = "user"
	InvocationSourceWorkflow      InvocationSource = "workflow"
	InvocationSourcePlugin        InvocationSource = "plugin"
	InvocationSourceSystem        InvocationSource = "system"
	InvocationSourceScheduledTask InvocationSource = "scheduled_task"
	InvocationSourceComputerUse   InvocationSource = "computer_use"
)

func (s InvocationSource) Valid() bool {
	switch s {
	case InvocationSourceModel,
		InvocationSourceUser,
		InvocationSourceWorkflow,
		InvocationSourcePlugin,
		InvocationSourceSystem,
		InvocationSourceScheduledTask,
		InvocationSourceComputerUse:
		return true
	default:
		return false
	}
}

type ApprovalMode string

const (
	ApprovalModeAuto    ApprovalMode = "auto"
	ApprovalModeManual  ApprovalMode = "manual"
	ApprovalModeSession ApprovalMode = "session"
)

func (m ApprovalMode) Valid() bool {
	switch m {
	case ApprovalModeAuto,
		ApprovalModeManual,
		ApprovalModeSession:
		return true
	default:
		return false
	}
}

type InvocationExecutionTarget struct {
	Placement string `json:"placement,omitempty"`

	UserID           runtimeidentity.UserID           `json:"userId,omitempty"`
	DeviceID         runtimeidentity.DeviceID         `json:"deviceId,omitempty"`
	RuntimeID        runtimeidentity.RuntimeID        `json:"runtimeId,omitempty"`
	RuntimeSessionID runtimeidentity.RuntimeSessionID `json:"runtimeSessionId,omitempty"`

	ProviderID         string `json:"providerId,omitempty"`
	ProviderInstanceID string `json:"providerInstanceId,omitempty"`

	ExtensionID string `json:"extensionId,omitempty"`
	ModuleID    string `json:"moduleId,omitempty"`
}

func (t InvocationExecutionTarget) IsZero() bool {
	return t.Placement == "" &&
		t.UserID == "" &&
		t.DeviceID == "" &&
		t.RuntimeID == "" &&
		t.RuntimeSessionID == "" &&
		t.ProviderID == "" &&
		t.ProviderInstanceID == "" &&
		t.ExtensionID == "" &&
		t.ModuleID == ""
}

type ToolInvocationContext struct {
	InvocationID string `json:"invocationId"`

	ExecContext *execution.ExecutionContext `json:"-"`

	ParentID       string           `json:"parentId,omitempty"`
	RootID         string           `json:"rootId,omitempty"`
	ExternalCallID string           `json:"externalCallId,omitempty"`
	UserID         string           `json:"userId"`
	CharacterID    string           `json:"characterId,omitempty"`
	ConversationID string           `json:"conversationId,omitempty"`
	Channel        string           `json:"channel,omitempty"`
	SessionID      string           `json:"sessionId,omitempty"`
	ExtensionID    string           `json:"extensionId,omitempty"`
	ModuleID       string           `json:"moduleId,omitempty"`
	Generation     int64            `json:"generation,omitempty"`
	Source         InvocationSource `json:"source"`
	ApprovalMode   ApprovalMode     `json:"approvalMode,omitempty"`
	ExpiresAt      time.Time        `json:"expiresAt,omitempty"`
	IdempotencyKey string           `json:"idempotencyKey,omitempty"`
	WorkflowRunID  string           `json:"workflowRunId,omitempty"`
	WorkflowNodeID string           `json:"workflowNodeId,omitempty"`
	LogicalAttempt int              `json:"logicalAttempt,omitempty"`
	FencingToken   int64            `json:"fencingToken,omitempty"`
	TraceID        string           `json:"traceId,omitempty"`
	OperationID    string           `json:"operationId,omitempty"`
	CorrelationID  string           `json:"correlationId,omitempty"`
	CausationID    string           `json:"causationId,omitempty"`
	ScheduleID     string           `json:"scheduleId,omitempty"`
	TriggerID      string           `json:"triggerId,omitempty"`

	ScopeSnapshotID      string `json:"scopeSnapshotId,omitempty"`
	PermissionSnapshotID string `json:"permissionSnapshotId,omitempty"`

	IsBackground bool `json:"isBackground,omitempty"`

	DeadlineDuration time.Duration `json:"-"`

	ExecutionTarget InvocationExecutionTarget `json:"executionTarget,omitempty"`

	Metadata map[string]any `json:"metadata,omitempty"`
}

type ToolInvocationOptions struct {
	Parent      *ToolInvocationContext
	ExecContext *execution.ExecutionContext

	ExternalCallID string

	UserID         string
	CharacterID    string
	ConversationID string

	Channel   string
	SessionID string

	ExtensionID string
	ModuleID    string
	Generation  int64

	Source       InvocationSource
	ApprovalMode ApprovalMode

	ExpiresAt time.Time

	IdempotencyKey string
	WorkflowRunID  string
	WorkflowNodeID string
	LogicalAttempt int
	FencingToken   int64

	TraceID     string
	OperationID string

	CorrelationID string
	CausationID   string

	ScheduleID string
	TriggerID  string

	ScopeSnapshotID      string
	PermissionSnapshotID string

	IsBackground bool

	ExecutionTarget InvocationExecutionTarget

	Metadata map[string]any
}

func NewInvocationID() string {
	return uuid.NewString()
}

func NewTraceID() string {
	return uuid.NewString()
}

func NewOperationID() string {
	return uuid.NewString()
}

func NewToolInvocationContext(opts ToolInvocationOptions) ToolInvocationContext {
	invocation := ToolInvocationContext{
		ExecContext:          opts.ExecContext,
		InvocationID:         NewInvocationID(),
		ExternalCallID:       opts.ExternalCallID,
		UserID:               opts.UserID,
		CharacterID:          opts.CharacterID,
		ConversationID:       opts.ConversationID,
		Channel:              opts.Channel,
		SessionID:            opts.SessionID,
		ExtensionID:          opts.ExtensionID,
		ModuleID:             opts.ModuleID,
		Generation:           opts.Generation,
		Source:               opts.Source,
		ApprovalMode:         opts.ApprovalMode,
		ExpiresAt:            opts.ExpiresAt,
		IdempotencyKey:       opts.IdempotencyKey,
		WorkflowRunID:        opts.WorkflowRunID,
		WorkflowNodeID:       opts.WorkflowNodeID,
		LogicalAttempt:       opts.LogicalAttempt,
		FencingToken:         opts.FencingToken,
		CorrelationID:        opts.CorrelationID,
		CausationID:          opts.CausationID,
		ScheduleID:           opts.ScheduleID,
		TriggerID:            opts.TriggerID,
		ScopeSnapshotID:      opts.ScopeSnapshotID,
		PermissionSnapshotID: opts.PermissionSnapshotID,
		IsBackground:         opts.IsBackground,
		ExecutionTarget:      opts.ExecutionTarget,
	}

	if opts.Metadata != nil {
		invocation.Metadata = cloneStringAnyMap(opts.Metadata)
	}

	if opts.ExecContext != nil {
		invocation.RootID = opts.ExecContext.RootExecutionID
	}

	if opts.Parent != nil {
		invocation.ParentID = opts.Parent.InvocationID
		if opts.Parent.RootID != "" {
			invocation.RootID = opts.Parent.RootID
		} else {
			invocation.RootID = opts.Parent.InvocationID
		}
		if invocation.ExecutionTarget.IsZero() && !opts.Parent.ExecutionTarget.IsZero() {
			invocation.ExecutionTarget = opts.Parent.ExecutionTarget
		}
		if opts.OperationID == "" && opts.Parent.OperationID != "" {
			invocation.OperationID = opts.Parent.OperationID
		} else {
			invocation.OperationID = opts.OperationID
		}
		if opts.TraceID == "" {
			invocation.TraceID = opts.Parent.TraceID
		} else {
			invocation.TraceID = opts.TraceID
		}
		if invocation.RootID == "" {
			invocation.RootID = invocation.InvocationID
		}
		if invocation.OperationID == "" {
			invocation.OperationID = NewOperationID()
		}
		if invocation.TraceID == "" {
			invocation.TraceID = NewTraceID()
		}
	} else {
		invocation.RootID = invocation.InvocationID
		if opts.TraceID != "" {
			invocation.TraceID = opts.TraceID
		} else {
			invocation.TraceID = NewTraceID()
		}
		if opts.OperationID != "" {
			invocation.OperationID = opts.OperationID
		} else {
			invocation.OperationID = NewOperationID()
		}
	}

	return invocation
}

func cloneStringAnyMap(src map[string]any) map[string]any {
	if src == nil {
		return nil
	}
	dst := make(map[string]any, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}
