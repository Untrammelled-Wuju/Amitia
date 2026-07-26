package scope

import "time"

type ScopeType string

const (
	ScopeGlobal       ScopeType = "global"
	ScopeCharacter    ScopeType = "character"
	ScopeConversation ScopeType = "conversation"
	ScopeExtension    ScopeType = "extension"
	ScopeModule       ScopeType = "module"
	ScopeResource     ScopeType = "resource"
	ScopeInvocation   ScopeType = "invocation"
	ScopeSession      ScopeType = "session"
)

type ScopeRef struct {
	Type           ScopeType `json:"type"`
	CharacterID    string    `json:"characterId,omitempty"`
	ConversationID string    `json:"conversationId,omitempty"`
	ExtensionID    string    `json:"extensionId,omitempty"`
	ModuleID       string    `json:"moduleId,omitempty"`
	ResourceType   string    `json:"resourceType,omitempty"`
	ResourceID     string    `json:"resourceId,omitempty"`
	InvocationID   string    `json:"invocationId,omitempty"`
	SessionID      string    `json:"sessionId,omitempty"`
}

type ScopeSubjectType string

const (
	SubjectExtension    ScopeSubjectType = "extension"
	SubjectModule       ScopeSubjectType = "module"
	SubjectTool         ScopeSubjectType = "tool"
	SubjectAgentSkill   ScopeSubjectType = "agent_skill"
	SubjectWorkflow     ScopeSubjectType = "workflow"
	SubjectMCPServer    ScopeSubjectType = "mcp_server"
	SubjectMCPTool      ScopeSubjectType = "mcp_tool"
	SubjectUIContribution ScopeSubjectType = "ui_contribution"
	SubjectBackgroundTask ScopeSubjectType = "background_task"
	SubjectProvider     ScopeSubjectType = "provider"
)

type ScopeBindingState string

const (
	StateActive   ScopeBindingState = "active"
	StateInactive ScopeBindingState = "inactive"
	StateExpired  ScopeBindingState = "expired"
	StateRevoked  ScopeBindingState = "revoked"
	StatePending  ScopeBindingState = "pending"
)

type ScopeBindingSource string

const (
	SourceSystem    ScopeBindingSource = "system"
	SourceUser      ScopeBindingSource = "user"
	SourcePackage   ScopeBindingSource = "package"
	SourceMigration ScopeBindingSource = "migration"
	SourceRuntime   ScopeBindingSource = "runtime"
	SourceTemporary ScopeBindingSource = "temporary"
)

type ScopeBinding struct {
	BindingID   string             `json:"bindingId"`
	SubjectType ScopeSubjectType   `json:"subjectType"`
	SubjectID   string             `json:"subjectId"`
	Scope       ScopeRef           `json:"scope"`
	State       ScopeBindingState  `json:"state"`
	Source      ScopeBindingSource `json:"source"`
	CreatedAt   time.Time          `json:"createdAt"`
	UpdatedAt   time.Time          `json:"updatedAt"`
	ExpiresAt   *time.Time         `json:"expiresAt,omitempty"`
	Metadata    map[string]any     `json:"metadata,omitempty"`
}

type ScopeSnapshot struct {
	SnapshotID     string      `json:"snapshotId"`
	InvocationID   string      `json:"invocationId"`
	ResolvedScopes []ScopeRef  `json:"resolvedScopes"`
	CharacterID    string      `json:"characterId"`
	ConversationID string      `json:"conversationId"`
	ExtensionID    string      `json:"extensionId"`
	ModuleID       string      `json:"moduleId"`
	CreatedAt      time.Time   `json:"createdAt"`
	ExpiresAt      *time.Time  `json:"expiresAt,omitempty"`
}

type ScopeDecision struct {
	Allowed bool          `json:"allowed"`
	Reasons []ScopeReason `json:"reasons,omitempty"`
	Matched []ScopeBinding `json:"matched,omitempty"`
}

type ScopeReason struct {
	Code        string `json:"code"`
	Description string `json:"description,omitempty"`
	SubjectID   string `json:"subjectId,omitempty"`
}
