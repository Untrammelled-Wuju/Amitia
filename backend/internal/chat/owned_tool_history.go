package chat

const (
	assistantTurnStatusQueued              = "queued"
	assistantTurnStatusStarting            = "starting"
	assistantTurnStatusRunning             = "running"
	assistantTurnStatusWaitingTool         = "waiting_tool"
	assistantTurnStatusCompleted           = "completed"
	assistantTurnStatusFailed              = "failed"
	assistantTurnStatusInterrupted         = "interrupted"
	assistantTurnStatusNeedsReconciliation = "needs_reconciliation"

	assistantTurnItemReasoning  = "reasoning"
	assistantTurnItemToolCall   = "tool_call"
	assistantTurnItemToolResult = "tool_result"
	assistantTurnItemText       = "text"
	assistantTurnItemError      = "error"
)

type AssistantTurn struct {
	ID             string              `gorm:"column:id;primaryKey" json:"id"`
	ConversationID string              `gorm:"column:conversation_id;not null;default:'';index" json:"conversationId"`
	CharacterID    string              `gorm:"column:character_id;not null;default:''" json:"characterId"`
	UserMessageID  string              `gorm:"column:user_message_id;not null;default:''" json:"userMessageId"`
	RequestID      string              `gorm:"column:request_id;not null;default:'';index" json:"requestId"`
	ExecutionID    string              `gorm:"column:execution_id;not null;default:'';index" json:"executionId"`
	ParentTurnID   string              `gorm:"column:parent_turn_id;not null;default:'';index" json:"parentTurnId,omitempty"`
	ParentBlockID  string              `gorm:"column:parent_block_id;not null;default:''" json:"parentBlockId,omitempty"`
	AgentID        string              `gorm:"column:agent_id;not null;default:'';index" json:"agentId,omitempty"`
	Sequence       int64               `gorm:"column:sequence;not null;default:0;index" json:"sequence"`
	Status         string              `gorm:"column:status;not null;default:running" json:"status"`
	CreatedAt      string              `gorm:"column:created_at;not null;default:''" json:"createdAt"`
	UpdatedAt      string              `gorm:"column:updated_at;not null;default:''" json:"updatedAt"`
	CompletedAt    string              `gorm:"column:completed_at;not null;default:''" json:"completedAt"`
	Items          []AssistantTurnItem `gorm:"-" json:"items"`
}

func (AssistantTurn) TableName() string { return "assistant_turns" }

type AssistantTurnItem struct {
	ID             string `gorm:"column:id;primaryKey" json:"id"`
	TurnID         string `gorm:"column:turn_id;not null;default:'';index" json:"turnId"`
	ConversationID string `gorm:"column:conversation_id;not null;default:'';index" json:"conversationId"`
	Sequence       int64  `gorm:"column:sequence;not null;default:0;index" json:"sequence"`
	ItemType       string `gorm:"column:item_type;not null;default:''" json:"type"`
	Status         string `gorm:"column:status;not null;default:pending" json:"status"`
	Revision       int64  `gorm:"column:revision;not null;default:0" json:"revision"`
	CallID         string `gorm:"column:call_id;not null;default:''" json:"callId,omitempty"`
	ToolName       string `gorm:"column:tool_name;not null;default:''" json:"toolName,omitempty"`
	Content        string `gorm:"column:content;not null;default:''" json:"content,omitempty"`
	ArgumentsJSON  string `gorm:"column:arguments_json;not null;default:''" json:"argumentsJson,omitempty"`
	ResultJSON     string `gorm:"column:result_json;not null;default:''" json:"resultJson,omitempty"`
	ErrorCode      string `gorm:"column:error_code;not null;default:''" json:"errorCode,omitempty"`
	DurationMS     int64  `gorm:"column:duration_ms;not null;default:0" json:"durationMs,omitempty"`
	IsFinal        int    `gorm:"column:is_final;not null;default:0" json:"isFinal,omitempty"`
	MessageID      string `gorm:"column:message_id;not null;default:''" json:"messageId,omitempty"`
	CreatedAt      string `gorm:"column:created_at;not null;default:''" json:"createdAt"`
	UpdatedAt      string `gorm:"column:updated_at;not null;default:''" json:"updatedAt"`
}

func (AssistantTurnItem) TableName() string { return "assistant_turn_items" }

