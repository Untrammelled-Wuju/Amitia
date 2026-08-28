// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package inbox

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/desktoppet/editing"
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

// EvaluationInboxConsumer is the durable ActionRevision event subscriber used
// by the revision-commit outbox. It writes a durable inbox record first and
// then creates the quality evaluation idempotently. The name is retained for
// compatibility with the quality-v2 schema, but events are delivered directly
// by the canonical ActionRevision outbox publisher rather than by polling the
// outbox table a second time.
type EvaluationInboxConsumer struct {
	db         *gorm.DB
	qualitySvc quality.QualityService
}

func NewEvaluationInboxConsumer(db *gorm.DB, qualitySvc quality.QualityService) *EvaluationInboxConsumer {
	return &EvaluationInboxConsumer{db: db, qualitySvc: qualitySvc}
}

// Publish implements revisioncommit.EventPublisher. Non-activation lifecycle
// events are intentionally acknowledged here; quality evaluation is triggered
// only when a revision becomes active.
func (c *EvaluationInboxConsumer) Publish(ctx context.Context, eventID, eventType string, payload []byte) error {
	if eventType != baseline.EventActionRevisionActivated {
		return nil
	}
	if eventID == "" {
		return fmt.Errorf("quality inbox: empty event id")
	}
	if c == nil || c.db == nil || c.qualitySvc == nil {
		return fmt.Errorf("quality inbox: consumer is not initialized")
	}

	record := ActionRevisionEventOutboxRecord{
		EventID:     eventID,
		EventType:   eventType,
		PayloadJSON: string(payload),
		CreatedAt:   time.Now().UTC().Format(time.RFC3339),
	}
	return c.handleActivated(ctx, record)
}

func (c *EvaluationInboxConsumer) handleActivated(ctx context.Context, record ActionRevisionEventOutboxRecord) error {
	var evt ActionRevisionActivatedEvent
	if err := json.Unmarshal([]byte(record.PayloadJSON), &evt); err != nil {
		return fmt.Errorf("unmarshal action revision activated event: %w", err)
	}
	if evt.ActionRevisionID == "" {
		return fmt.Errorf("quality inbox: empty action revision id")
	}

	var revision editing.ActionRevision
	if err := c.db.WithContext(ctx).Where("id = ?", evt.ActionRevisionID).First(&revision).Error; err != nil {
		return fmt.Errorf("load action revision %s: %w", evt.ActionRevisionID, err)
	}
	if revision.Status != editing.RevisionStatusReady && revision.Status != editing.RevisionStatusQualityPending && revision.Status != editing.RevisionStatusQualityReady {
		return fmt.Errorf("action revision %s is not quality-evaluable: status=%s", revision.ID, revision.Status)
	}

	payloadSum := sha256.Sum256([]byte(record.PayloadJSON))
	payloadHash := hex.EncodeToString(payloadSum[:])
	idempotencyKey := "action-revision-activated:" + record.EventID
	now := time.Now().UTC().Format(time.RFC3339)

	var inbox quality.EvaluationRequestInboxRecord
	err := c.db.WithContext(ctx).Where("event_id = ?", record.EventID).First(&inbox).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		return fmt.Errorf("load quality evaluation inbox: %w", err)
	}
	if err == gorm.ErrRecordNotFound {
		receivedAt := record.CreatedAt
		if evt.OccurredAt != "" {
			receivedAt = evt.OccurredAt
		}
		if receivedAt == "" {
			receivedAt = now
		}
		inbox = quality.EvaluationRequestInboxRecord{
			ID:                "qri-" + uuid.NewString(),
			EventID:           record.EventID,
			ActionRevisionID:  revision.ID,
			ActionContentHash: revision.ContentHash,
			ProfileID:         revision.QualityProfileID,
			RuleSetVersion:    revision.QualityRulesetVersion,
			IdempotencyKey:    idempotencyKey,
			PayloadHash:       payloadHash,
			Status:            quality.InboxStatusReceived,
			ReceivedAt:        receivedAt,
			CreatedAt:         now,
			UpdatedAt:         now,
		}
		if err := c.db.WithContext(ctx).Create(&inbox).Error; err != nil {
			// Another delivery may have won the unique event-id race. Reload it
			// and continue the same idempotent workflow.
			if reloadErr := c.db.WithContext(ctx).Where("event_id = ?", record.EventID).First(&inbox).Error; reloadErr != nil {
				return fmt.Errorf("create quality evaluation inbox: %w", err)
			}
		}
	}
	if inbox.Status == quality.InboxStatusCompleted {
		return nil
	}
	if inbox.ActionRevisionID != "" && inbox.ActionRevisionID != revision.ID {
		return fmt.Errorf("quality inbox event collision: event=%s existingRevision=%s incomingRevision=%s", record.EventID, inbox.ActionRevisionID, revision.ID)
	}
	if inbox.PayloadHash != "" && inbox.PayloadHash != payloadHash {
		return fmt.Errorf("quality inbox payload hash mismatch: event=%s", record.EventID)
	}

	if err := c.db.WithContext(ctx).Model(&quality.EvaluationRequestInboxRecord{}).
		Where("id = ?", inbox.ID).
		Updates(map[string]any{
			"status":          quality.InboxStatusValidating,
			"attempt_count":   gorm.Expr("attempt_count + 1"),
			"last_error":      "",
			"updated_at":      now,
			"payload_hash":    payloadHash,
			"idempotency_key": idempotencyKey,
		}).Error; err != nil {
		return fmt.Errorf("claim quality evaluation inbox: %w", err)
	}

	userID := revision.UserID
	if userID == "" {
		userID = evt.UserID
	}
	characterID := revision.CharacterID
	if characterID == "" {
		characterID = evt.CharacterID
	}
	if userID == "" || characterID == "" {
		return c.failInbox(ctx, inbox.ID, fmt.Errorf("quality inbox missing ownership identity for revision %s", revision.ID))
	}

	_, createErr := c.qualitySvc.CreateEvaluation(ctx, quality.CreateEvaluationRequest{
		UserID:               userID,
		CharacterID:          characterID,
		ProcessingTaskID:     revision.ProcessingTaskID,
		ProcessingActionID:   revision.ProcessingActionID,
		ActionRevisionID:     revision.ID,
		ActionContentHash:    revision.ContentHash,
		ProcessingRevisionID: revision.SourceProcessingRevisionID,
		ActionKey:            revision.ActionKey,
		ProfileID:            revision.QualityProfileID,
		RuleSetVersion:       revision.QualityRulesetVersion,
		IdempotencyKey:       idempotencyKey,
	})
	if createErr != nil {
		return c.failInbox(ctx, inbox.ID, createErr)
	}

	completedAt := time.Now().UTC().Format(time.RFC3339)
	if err := c.db.WithContext(ctx).Model(&quality.EvaluationRequestInboxRecord{}).
		Where("id = ?", inbox.ID).
		Updates(map[string]any{
			"status":       quality.InboxStatusCompleted,
			"processed_at": completedAt,
			"last_error":   "",
			"updated_at":   completedAt,
		}).Error; err != nil {
		return fmt.Errorf("complete quality evaluation inbox: %w", err)
	}

	log.Logger.Infof("quality evaluation scheduled from action revision activation: eventID=%s revisionID=%s actionKey=%s",
		record.EventID, revision.ID, revision.ActionKey)
	return nil
}

func (c *EvaluationInboxConsumer) failInbox(ctx context.Context, inboxID string, cause error) error {
	if cause == nil {
		return nil
	}
	now := time.Now().UTC().Format(time.RFC3339)
	_ = c.db.WithContext(ctx).Model(&quality.EvaluationRequestInboxRecord{}).
		Where("id = ?", inboxID).
		Updates(map[string]any{
			"status":     quality.InboxStatusFailedRetry,
			"last_error": cause.Error(),
			"updated_at": now,
		}).Error
	return cause
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
	return "desktop_pet_action_revision_event_outbox"
}
