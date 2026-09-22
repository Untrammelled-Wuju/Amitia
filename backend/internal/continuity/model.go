package continuity

import (
	"encoding/json"
	"strings"
	"time"
)

type ThreadStatus string

const (
	ThreadStatusActive    ThreadStatus = "active"
	ThreadStatusWaiting   ThreadStatus = "waiting"
	ThreadStatusBlocked   ThreadStatus = "blocked"
	ThreadStatusPaused    ThreadStatus = "paused"
	ThreadStatusCompleted ThreadStatus = "completed"
	ThreadStatusCancelled ThreadStatus = "cancelled"
)

func (s ThreadStatus) IsTerminal() bool {
	return s == ThreadStatusCompleted || s == ThreadStatusCancelled
}

func ValidThreadStatus(v ThreadStatus) bool {
	switch v {
	case ThreadStatusActive, ThreadStatusWaiting, ThreadStatusBlocked, ThreadStatusPaused, ThreadStatusCompleted, ThreadStatusCancelled:
		return true
	default:
		return false
	}
}

type Thread struct {
	ID             string       `gorm:"column:id;primaryKey" json:"id"`
	SpaceID        string       `gorm:"column:space_id;not null;index" json:"spaceId"`
	CharacterID    string       `gorm:"column:character_id;not null;default:'';index" json:"characterId,omitempty"`
	ParentThreadID string       `gorm:"column:parent_thread_id;not null;default:'';index" json:"parentThreadId,omitempty"`
	Title          string       `gorm:"column:title;not null" json:"title"`
	Goal           string       `gorm:"column:goal;not null;default:''" json:"goal,omitempty"`
	Status         ThreadStatus `gorm:"column:status;not null;default:'active';index" json:"status"`
	Summary        string       `gorm:"column:summary;not null;default:''" json:"summary,omitempty"`
	CurrentState   string       `gorm:"column:current_state;not null;default:''" json:"currentState,omitempty"`
	NextAction     string       `gorm:"column:next_action;not null;default:''" json:"nextAction,omitempty"`
	Priority       int          `gorm:"column:priority;not null;default:0" json:"priority"`
	Confidence     float64      `gorm:"column:confidence;not null;default:1" json:"confidence"`
	Revision       int64        `gorm:"column:revision;not null;default:1" json:"revision"`
	CreatedAt      time.Time    `gorm:"column:created_at;not null" json:"createdAt"`
	UpdatedAt      time.Time    `gorm:"column:updated_at;not null" json:"updatedAt"`
	LastActiveAt   time.Time    `gorm:"column:last_active_at;not null;index" json:"lastActiveAt"`
	CompletedAt    *time.Time   `gorm:"column:completed_at" json:"completedAt,omitempty"`
}

func (Thread) TableName() string { return "continuity_threads" }

type ThreadBinding struct {
	ID           string    `gorm:"column:id;primaryKey" json:"id"`
	ThreadID     string    `gorm:"column:thread_id;not null;index" json:"threadId"`
	BindingType  string    `gorm:"column:binding_type;not null;index" json:"bindingType"`
	BindingID    string    `gorm:"column:binding_id;not null;index" json:"bindingId"`
	Role         string    `gorm:"column:role;not null;default:'context'" json:"role"`
	Confidence   float64   `gorm:"column:confidence;not null;default:1" json:"confidence"`
	Source       string    `gorm:"column:source;not null;default:''" json:"source,omitempty"`
	CreatedAt    time.Time `gorm:"column:created_at;not null" json:"createdAt"`
	LastActiveAt time.Time `gorm:"column:last_active_at;not null;index" json:"lastActiveAt"`
}

func (ThreadBinding) TableName() string { return "continuity_thread_bindings" }

type ThreadEvent struct {
	ID             string    `gorm:"column:id;primaryKey" json:"id"`
	ThreadID       string    `gorm:"column:thread_id;not null;index" json:"threadId"`
	EventType      string    `gorm:"column:event_type;not null;index" json:"eventType"`
	SourceType     string    `gorm:"column:source_type;not null;default:''" json:"sourceType,omitempty"`
	SourceID       string    `gorm:"column:source_id;not null;default:''" json:"sourceId,omitempty"`
	ConversationID string    `gorm:"column:conversation_id;not null;default:'';index" json:"conversationId,omitempty"`
	RequestID      string    `gorm:"column:request_id;not null;default:'';index" json:"requestId,omitempty"`
	ExecutionID    string    `gorm:"column:execution_id;not null;default:'';index" json:"executionId,omitempty"`
	PayloadJSON    string    `gorm:"column:payload_json;not null;default:'{}'" json:"payloadJson"`
	IdempotencyKey string    `gorm:"column:idempotency_key;not null;index" json:"idempotencyKey"`
	OccurredAt     time.Time `gorm:"column:occurred_at;not null;index" json:"occurredAt"`
}

func (ThreadEvent) TableName() string { return "continuity_thread_events" }

type WaitStatus string

const (
	WaitStatusWaiting   WaitStatus = "waiting"
	WaitStatusResolved  WaitStatus = "resolved"
	WaitStatusCancelled WaitStatus = "cancelled"
)

const (
	WaitTypeUser       = "user"
	WaitTypeTime       = "time"
	WaitTypeDevice     = "device"
	WaitTypeExternal   = "external"
	WaitTypeApproval   = "approval"
	WaitTypeDependency = "dependency"
)

func ValidWaitType(v string) bool {
	switch strings.TrimSpace(v) {
	case WaitTypeUser, WaitTypeTime, WaitTypeDevice, WaitTypeExternal, WaitTypeApproval, WaitTypeDependency:
		return true
	default:
		return false
	}
}

const (
	WakeStateNone      = ""
	WakeStatePending   = "pending"
	WakeStateDelivered = "delivered"
	WakeStateSkipped   = "skipped"
	WakeStateFailed    = "failed"
)

type Wait struct {
	ID                string     `gorm:"column:id;primaryKey" json:"id"`
	ThreadID          string     `gorm:"column:thread_id;not null;index" json:"threadId"`
	WaitType          string     `gorm:"column:wait_type;not null;index" json:"waitType"`
	Status            WaitStatus `gorm:"column:status;not null;default:'waiting';index" json:"status"`
	Description       string     `gorm:"column:description;not null;default:''" json:"description,omitempty"`
	ConditionJSON     string     `gorm:"column:condition_json;not null;default:'{}'" json:"conditionJson"`
	ResumeHint        string     `gorm:"column:resume_hint;not null;default:''" json:"resumeHint,omitempty"`
	DueAt             *time.Time `gorm:"column:due_at;index" json:"dueAt,omitempty"`
	ResolvedAt        *time.Time `gorm:"column:resolved_at" json:"resolvedAt,omitempty"`
	ResolvedBy        string     `gorm:"column:resolved_by;not null;default:''" json:"resolvedBy,omitempty"`
	ResolutionJSON    string     `gorm:"column:resolution_json;not null;default:'{}'" json:"resolutionJson,omitempty"`
	SourceExecutionID string     `gorm:"column:source_execution_id;not null;default:'';index" json:"sourceExecutionId,omitempty"`
	AutoResume        bool       `gorm:"column:auto_resume;not null" json:"autoResume"`
	WakeState         string     `gorm:"column:wake_state;not null;default:'';index" json:"wakeState,omitempty"`
	WakeRequestID     string     `gorm:"column:wake_request_id;not null;default:''" json:"wakeRequestId,omitempty"`
	WakeAttempts      int        `gorm:"column:wake_attempts;not null;default:0" json:"wakeAttempts"`
	NextWakeAt        *time.Time `gorm:"column:next_wake_at;index" json:"nextWakeAt,omitempty"`
	LastWakeError     string     `gorm:"column:last_wake_error;not null;default:''" json:"lastWakeError,omitempty"`
	WakeDeliveredAt   *time.Time `gorm:"column:wake_delivered_at" json:"wakeDeliveredAt,omitempty"`
	CreatedAt         time.Time  `gorm:"column:created_at;not null" json:"createdAt"`
	UpdatedAt         time.Time  `gorm:"column:updated_at;not null" json:"updatedAt"`
}

func (Wait) TableName() string { return "continuity_waits" }

type Context struct {
	ThreadID     string         `json:"threadId,omitempty"`
	Title        string         `json:"title,omitempty"`
	Goal         string         `json:"goal,omitempty"`
	Status       ThreadStatus   `json:"status,omitempty"`
	Summary      string         `json:"summary,omitempty"`
	CurrentState string         `json:"currentState,omitempty"`
	NextAction   string         `json:"nextAction,omitempty"`
	Waits        []WaitSummary  `json:"waits,omitempty"`
	RecentEvents []EventSummary `json:"recentEvents,omitempty"`
	Revision     int64          `json:"revision,omitempty"`
}

type WaitSummary struct {
	ID          string     `json:"id"`
	Type        string     `json:"type"`
	Description string     `json:"description,omitempty"`
	ResumeHint  string     `json:"resumeHint,omitempty"`
	DueAt       *time.Time `json:"dueAt,omitempty"`
}

type EventSummary struct {
	Type       string    `json:"type"`
	Summary    string    `json:"summary,omitempty"`
	OccurredAt time.Time `json:"occurredAt"`
}

type Resolution struct {
	Thread     *Thread `json:"thread,omitempty"`
	Method     string  `json:"method"`
	Confidence float64 `json:"confidence"`
	Created    bool    `json:"created"`
}

type ResolveInput struct {
	SpaceID        string
	CharacterID    string
	ConversationID string
	WorkspaceID    string
	ThreadID       string
	Message        string
	SuppressCreate bool
}

type ObservePayload struct {
	Version        string `json:"version"`
	ThreadID       string `json:"threadId"`
	SpaceID        string `json:"spaceId,omitempty"`
	CharacterID    string `json:"characterId,omitempty"`
	ConversationID string `json:"conversationId,omitempty"`
	RequestID      string `json:"requestId,omitempty"`
	ExecutionID    string `json:"executionId,omitempty"`
	UserMessage    string `json:"userMessage,omitempty"`
	AssistantReply string `json:"assistantReply,omitempty"`
	Source         string `json:"source,omitempty"`
}

type WaitPatch struct {
	Action      string          `json:"action"`
	WaitID      string          `json:"waitId,omitempty"`
	Type        string          `json:"type,omitempty"`
	Description string          `json:"description,omitempty"`
	Condition   json.RawMessage `json:"condition,omitempty"`
	ResumeHint  string          `json:"resumeHint,omitempty"`
	DueAt       string          `json:"dueAt,omitempty"`
	AutoResume  *bool           `json:"autoResume,omitempty"`
	Resolution  json.RawMessage `json:"resolution,omitempty"`
}

type ThreadPatch struct {
	StateChanged bool         `json:"stateChanged"`
	Title        string       `json:"title,omitempty"`
	Goal         string       `json:"goal,omitempty"`
	Status       ThreadStatus `json:"status,omitempty"`
	Summary      string       `json:"summary,omitempty"`
	CurrentState string       `json:"currentState,omitempty"`
	NextAction   string       `json:"nextAction,omitempty"`
	Confidence   float64      `json:"confidence,omitempty"`
	EventType    string       `json:"eventType,omitempty"`
	EventSummary string       `json:"eventSummary,omitempty"`
	Waits        []WaitPatch  `json:"waits,omitempty"`
}

type Signal struct {
	WaitType   string         `json:"waitType"`
	SpaceID    string         `json:"spaceId,omitempty"`
	Source     string         `json:"source,omitempty"`
	SourceID   string         `json:"sourceId,omitempty"`
	Attributes map[string]any `json:"attributes,omitempty"`
	OccurredAt time.Time      `json:"occurredAt,omitempty"`
}

type WakeRequest struct {
	WaitID         string `json:"waitId"`
	ThreadID       string `json:"threadId"`
	SpaceID        string `json:"spaceId"`
	CharacterID    string `json:"characterId,omitempty"`
	ConversationID string `json:"conversationId,omitempty"`
	Channel        string `json:"channel,omitempty"`
	PeerID         string `json:"peerId,omitempty"`
	RequestID      string `json:"requestId"`
	Message        string `json:"message"`
	ResumeHint     string `json:"resumeHint,omitempty"`
	ResolvedBy     string `json:"resolvedBy,omitempty"`
}

type ListThreadsFilter struct {
	SpaceID     string
	CharacterID string
	Status      []ThreadStatus
	Query       string
	Limit       int
	Offset      int
}
