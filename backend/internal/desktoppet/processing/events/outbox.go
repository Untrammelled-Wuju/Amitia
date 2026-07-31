package events

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type ProcessingRevisionCommittedEvent struct {
	UserID                     string `json:"userId"`
	CharacterID                string `json:"characterId"`
	ProcessingTaskID           string `json:"processingTaskId"`
	ProcessingActionID         string `json:"processingActionId"`
	ProcessingAttemptID        string `json:"processingAttemptId"`
	ProcessingRevisionID       string `json:"processingRevisionId"`
	RevisionNumber             int    `json:"revisionNumber"`
	ActionKey                  string `json:"actionKey"`
	SourceManifestID           string `json:"sourceManifestId"`
	SourceGenerationAttemptID  string `json:"sourceGenerationAttemptId"`
	SourceGenerationArtifactID string `json:"sourceGenerationArtifactId"`
	SourceArtifactContentHash  string `json:"sourceArtifactContentHash"`
	FrameCount                 int    `json:"frameCount"`
	RevisionHash               string `json:"revisionHash"`
	ContentRootHash            string `json:"contentRootHash"`
	ConfigHash                 string `json:"configHash"`
	PipelineVersion            string `json:"pipelineVersion"`
	OccurredAt                 string `json:"occurredAt"`
}

const EventTopicProcessingRevisionCommitted = "desktop_pet.processing_revision.committed"

const (
	OutboxStatusPending   = "pending"
	OutboxStatusPublished = "published"
	OutboxStatusFailed    = "failed"
)

type OutboxRecord struct {
	ID          string `gorm:"column:id;primaryKey" json:"id"`
	EventType   string `gorm:"column:event_type" json:"eventType"`
	AggregateID string `gorm:"column:aggregate_id" json:"aggregateId"`
	Payload     string `gorm:"column:payload" json:"payload"`
	Status      string `gorm:"column:status;default:'pending'" json:"status"`
	CreatedAt   string `gorm:"column:created_at" json:"createdAt"`
	PublishedAt string `gorm:"column:published_at;default:''" json:"publishedAt"`
	Error       string `gorm:"column:error;default:''" json:"error"`
	RetryCount  int    `gorm:"column:retry_count;default:0" json:"retryCount"`
}

func (OutboxRecord) TableName() string { return "desktop_pet_processing_event_outbox" }

type OutboxRepository interface {
	CreateOutbox(tx *gorm.DB, record *OutboxRecord) error
	ListPendingOutbox(limit int) ([]OutboxRecord, error)
	MarkPublished(outboxID string) error
	MarkFailed(outboxID string, errMsg string) error
}

type EventOutbox struct {
	repo OutboxRepository
	now  func() string
}

func NewEventOutbox(repo OutboxRepository) *EventOutbox {
	return &EventOutbox{
		repo: repo,
		now: func() string {
			return time.Now().UTC().Format(time.RFC3339)
		},
	}
}

func (o *EventOutbox) EmitProcessingRevisionCommitted(tx *gorm.DB, event ProcessingRevisionCommittedEvent) error {
	if event.OccurredAt == "" {
		event.OccurredAt = o.now()
	}
	payload, err := json.Marshal(event)
	if err != nil {
		return err
	}
	record := &OutboxRecord{
		ID:          uuid.NewString(),
		EventType:   EventTopicProcessingRevisionCommitted,
		AggregateID: event.ProcessingRevisionID,
		Payload:     string(payload),
		Status:      OutboxStatusPending,
		CreatedAt:   o.now(),
	}
	return o.repo.CreateOutbox(tx, record)
}
