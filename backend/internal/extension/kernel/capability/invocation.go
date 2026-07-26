package capability

import "time"

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

type ApprovalMode string

const (
	ApprovalModeAuto    ApprovalMode = "auto"
	ApprovalModeManual  ApprovalMode = "manual"
	ApprovalModeSession ApprovalMode = "session"
)

type ToolInvocationContext struct {
	InvocationID   string           `json:"invocationId"`
	ParentID       string           `json:"parentId,omitempty"`
	UserID         string           `json:"userId"`
	CharacterID    string           `json:"characterId,omitempty"`
	ConversationID string           `json:"conversationId,omitempty"`
	ExtensionID    string           `json:"extensionId,omitempty"`
	ModuleID       string           `json:"moduleId,omitempty"`
	Source         InvocationSource `json:"source"`
	ApprovalMode   ApprovalMode     `json:"approvalMode,omitempty"`
	ExpiresAt      time.Time        `json:"expiresAt,omitempty"`
	IdempotencyKey string           `json:"idempotencyKey,omitempty"`
	TraceID        string           `json:"traceId,omitempty"`
	Metadata       map[string]any   `json:"metadata,omitempty"`
}
