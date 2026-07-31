package generation

type OutboxStatus string

const (
	OutboxStatusPending    OutboxStatus = "pending"
	OutboxStatusProcessing OutboxStatus = "processing"
	OutboxStatusCompleted  OutboxStatus = "completed"
	OutboxStatusFailed     OutboxStatus = "failed"
)

type OutboxEventType string

const (
	OutboxEventAttemptSucceeded  OutboxEventType = "attempt_succeeded"
	OutboxEventAttemptFailed     OutboxEventType = "attempt_failed"
	OutboxEventArtifactPersisted OutboxEventType = "artifact_persisted"
	OutboxEventActionCompleted   OutboxEventType = "action_completed"
	OutboxEventTaskCompleted     OutboxEventType = "task_completed"
)

type GenerationOutboxEntry struct {
	ID           string `gorm:"column:id;primaryKey;type:text" json:"id"`
	TaskID       string `gorm:"column:task_id;type:text" json:"taskId"`
	TaskActionID string `gorm:"column:task_action_id;type:text" json:"taskActionId"`
	AttemptID    string `gorm:"column:attempt_id;type:text" json:"attemptId"`
	EventType    string `gorm:"column:event_type;type:text" json:"eventType"`
	PayloadJSON  string `gorm:"column:payload_json;type:text" json:"payloadJson"`
	Status       string `gorm:"column:status;type:text;default:'pending'" json:"status"`
	RetryCount   int    `gorm:"column:retry_count;type:integer;default:0" json:"retryCount"`
	MaxRetries   int    `gorm:"column:max_retries;type:integer;default:3" json:"maxRetries"`
	NextRetryAt  string `gorm:"column:next_retry_at;type:text" json:"nextRetryAt"`
	ProcessedAt  string `gorm:"column:processed_at;type:text" json:"processedAt"`
	ErrorMessage string `gorm:"column:error_message;type:text" json:"errorMessage"`
	CreatedAt    string `gorm:"column:created_at;type:text" json:"createdAt"`
	UpdatedAt    string `gorm:"column:updated_at;type:text" json:"updatedAt"`
}

func (GenerationOutboxEntry) TableName() string {
	return "desktop_pet_generation_outbox"
}
