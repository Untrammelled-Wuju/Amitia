package generation

import (
	"encoding/json"
	"fmt"

	"gorm.io/gorm"
)

type OutboxPublisher struct {
	repo OutboxRepository
}

func NewOutboxPublisher(repo OutboxRepository) *OutboxPublisher {
	return &OutboxPublisher{repo: repo}
}

func (p *OutboxPublisher) Publish(tx *gorm.DB, taskID, taskActionID, attemptID string, eventType OutboxEventType, payload interface{}) error {
	if tx == nil {
		return fmt.Errorf("transaction is required for outbox publish")
	}
	payloadJSON := "{}"
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("marshal outbox payload: %w", err)
		}
		payloadJSON = string(data)
	}
	entry := &GenerationOutboxEntry{
		TaskID:       taskID,
		TaskActionID: taskActionID,
		AttemptID:    attemptID,
		EventType:    string(eventType),
		PayloadJSON:  payloadJSON,
		Status:       string(OutboxStatusPending),
		MaxRetries:   3,
	}
	if err := p.repo.CreateTx(tx, entry); err != nil {
		return fmt.Errorf("publish outbox event: %w", err)
	}
	return nil
}
