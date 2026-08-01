// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package inbox

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/u-ai/backend/internal/desktoppet/editing/baseline"
	"github.com/u-ai/backend/internal/desktoppet/quality"
	"github.com/u-ai/backend/log"
	"gorm.io/gorm"
)

type ActionRevisionActivatedEvent struct {
	ActionRevisionID   string `json:"actionRevisionId"`
	ActionStreamID     string `json:"actionStreamId"`
	BindingRevision    int64  `json:"bindingRevision"`
	PreviousRevisionID string `json:"previousRevisionId"`
	ActionKey          string `json:"actionKey"`
	UserID             string `json:"userId"`
	CharacterID        string `json:"characterId"`
	OccurredAt         string `json:"occurredAt"`
}

type EvaluationInboxConsumer struct {
	db         *gorm.DB
	qualitySvc quality.QualityService
	interval   time.Duration
}

func NewEvaluationInboxConsumer(db *gorm.DB, qualitySvc quality.QualityService) *EvaluationInboxConsumer {
	return &EvaluationInboxConsumer{
		db:         db,
		qualitySvc: qualitySvc,
		interval:   5 * time.Second,
	}
}

func (c *EvaluationInboxConsumer) Start(ctx context.Context) {
	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.processBatch(ctx)
		}
	}
}

func (c *EvaluationInboxConsumer) processBatch(ctx context.Context) {
	events, err := c.pollActivatedEvents(ctx)
	if err != nil {
		log.Logger.Errorf("poll activated events failed: %v", err)
		return
	}

	for _, evt := range events {
		if err := c.handleActivated(ctx, evt); err != nil {
			log.Logger.Errorf("handle activated event failed: eventID=%s err=%v", evt.EventID, err)
		}
	}
}

func (c *EvaluationInboxConsumer) pollActivatedEvents(ctx context.Context) ([]ActionRevisionEventOutboxRecord, error) {
	var records []ActionRevisionEventOutboxRecord

	err := c.db.WithContext(ctx).
		Where("event_type = ?", baseline.EventActionRevisionActivated).
		Order("created_at ASC").
		Limit(10).
		Find(&records).Error

	return records, err
}

func (c *EvaluationInboxConsumer) handleActivated(ctx context.Context, record ActionRevisionEventOutboxRecord) error {
	var evt ActionRevisionActivatedEvent
	if err := json.Unmarshal([]byte(record.PayloadJSON), &evt); err != nil {
		return fmt.Errorf("unmarshal event payload: %w", err)
	}

	if evt.ActionRevisionID == "" {
		return fmt.Errorf("empty action revision id")
	}

	inbox := &quality.EvaluationRequestInboxRecord{
		EventID:          record.EventID,
		ActionRevisionID: evt.ActionRevisionID,
		Status:           quality.InboxStatusReceived,
		ReceivedAt:       record.CreatedAt,
	}

	if evt.OccurredAt != "" {
		inbox.ReceivedAt = evt.OccurredAt
	}

	var existing quality.EvaluationRequestInboxRecord
	err := c.db.WithContext(ctx).Where("event_id = ?", record.EventID).First(&existing).Error
	if err == nil {
		return nil
	}

	if err := c.db.WithContext(ctx).Create(inbox).Error; err != nil {
		return fmt.Errorf("create evaluation inbox entry: %w", err)
	}

	log.Logger.Infof("evaluation inbox entry created: eventID=%s revisionID=%s actionKey=%s",
		record.EventID, evt.ActionRevisionID, evt.ActionKey)

	return nil
}

type ActionRevisionEventOutboxRecord struct {
	ID          string `gorm:"column:id;primaryKey"`
	EventID     string `gorm:"column:event_id"`
	EventType   string `gorm:"column:event_type"`
	AggregateID string `gorm:"column:aggregate_id"`
	Sequence    int64  `gorm:"column:sequence"`
	PayloadJSON string `gorm:"column:payload_json"`
	Status      string `gorm:"column:status"`
	AvailableAt string `gorm:"column:available_at"`
	CreatedAt   string `gorm:"column:created_at"`
}

func (ActionRevisionEventOutboxRecord) TableName() string {
	return "desktop_pet_action_revision_events"
}
