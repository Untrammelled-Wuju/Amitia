package attention

import "time"

type BudgetPolicy struct {
	SpaceID                   string    `gorm:"column:space_id;primaryKey" json:"spaceId"`
	Enabled                   bool      `gorm:"column:enabled;not null" json:"enabled"`
	ShadowMode                bool      `gorm:"column:shadow_mode;not null" json:"shadowMode"`
	MaxInterruptionsPerDay    int       `gorm:"column:max_interruptions_per_day;not null;default:24" json:"maxInterruptionsPerDay"`
	MaxRepeatedConfirmations  int       `gorm:"column:max_repeated_confirmations;not null;default:3" json:"maxRepeatedConfirmations"`
	MaxConsecutiveFollowups   int       `gorm:"column:max_consecutive_followups;not null;default:2" json:"maxConsecutiveFollowups"`
	MaxUnchangedNotifications int       `gorm:"column:max_unchanged_notifications;not null;default:1" json:"maxUnchangedNotifications"`
	DedupeWindowMinutes       int       `gorm:"column:dedupe_window_minutes;not null;default:30" json:"dedupeWindowMinutes"`
	Revision                  int64     `gorm:"column:revision;not null;default:1" json:"revision"`
	CreatedAt                 time.Time `gorm:"column:created_at;not null" json:"createdAt"`
	UpdatedAt                 time.Time `gorm:"column:updated_at;not null" json:"updatedAt"`
}

func (BudgetPolicy) TableName() string {
	return "agent_attention_policies"
}

type Event struct {
	ID             string    `gorm:"column:id;primaryKey" json:"id"`
	SpaceID        string    `gorm:"column:space_id;not null;index" json:"spaceId"`
	RoleID         string    `gorm:"column:role_id;not null;default:'';index" json:"roleId"`
	ConversationID string    `gorm:"column:conversation_id;not null;default:'';index" json:"conversationId"`
	SourceKind     string    `gorm:"column:source_kind;not null;default:'';index" json:"sourceKind"`
	SourceID       string    `gorm:"column:source_id;not null;default:'';index" json:"sourceId"`
	EventKind      string    `gorm:"column:event_kind;not null;default:'';index" json:"eventKind"`
	DeliveryState  string    `gorm:"column:delivery_state;not null;default:'';index" json:"deliveryState"`
	Reason         string    `gorm:"column:reason;not null;default:''" json:"reason"`
	PayloadJSON    string    `gorm:"column:payload_json;not null;default:'{}'" json:"payloadJson"`
	IdempotencyKey string    `gorm:"column:idempotency_key;not null;index" json:"idempotencyKey"`
	CreatedAt      time.Time `gorm:"column:created_at;not null;index" json:"createdAt"`
}

func (Event) TableName() string {
	return "agent_attention_events"
}

type CheckInput struct {
	SpaceID        string
	RoleID         string
	ConversationID string
	SourceKind     string
	SourceID       string
	EventKind      string
	IdempotencyKey string
	PayloadJSON    string
}

type CheckResult struct {
	Allowed       bool         `json:"allowed"`
	Shadowed      bool         `json:"shadowed"`
	Reason        string       `json:"reason"`
	Event         Event        `json:"event"`
	Policy        BudgetPolicy `json:"policy"`
	Interruptions int          `json:"interruptions"`
	Duplicates    int          `json:"duplicates"`
}

type Stats struct {
	Policy                 BudgetPolicy `json:"policy"`
	TodayInterruptions     int64        `json:"todayInterruptions"`
	TodaySuppressed        int64        `json:"todaySuppressed"`
	TodayObserved          int64        `json:"todayObserved"`
	RepeatedConfirmations  int64        `json:"repeatedConfirmations"`
	ConsecutiveFollowups   int64        `json:"consecutiveFollowups"`
	UnchangedNotifications int64        `json:"unchangedNotifications"`
}

type Item struct {
	ID             string         `json:"id"`
	Kind           string         `json:"kind"`
	Decision       string         `json:"decision"`
	Priority       int            `json:"priority"`
	RequiresAction bool           `json:"requiresAction"`
	Title          string         `json:"title"`
	Summary        string         `json:"summary"`
	Reason         string         `json:"reason"`
	SpaceID        string         `json:"spaceId"`
	RoleID         string         `json:"roleId,omitempty"`
	ConversationID string         `json:"conversationId,omitempty"`
	ThreadID       string         `json:"threadId,omitempty"`
	RequestID      string         `json:"requestId,omitempty"`
	CoordinationID string         `json:"coordinationId,omitempty"`
	ApprovalID     string         `json:"approvalId,omitempty"`
	TurnID         string         `json:"turnId,omitempty"`
	WorkflowID     string         `json:"workflowId,omitempty"`
	WorkflowRunID  string         `json:"workflowRunId,omitempty"`
	Route          string         `json:"route,omitempty"`
	CreatedAt      time.Time      `json:"createdAt"`
	ExpiresAt      *time.Time     `json:"expiresAt,omitempty"`
	Payload        map[string]any `json:"payload,omitempty"`
}
