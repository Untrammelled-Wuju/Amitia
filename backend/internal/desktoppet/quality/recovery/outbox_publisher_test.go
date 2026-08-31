// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package recovery

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"github.com/u-ai/backend/internal/desktoppet/quality"
	"gorm.io/gorm"
)

type captureQualityEventPublisher struct {
	events []quality.QualityEvent
}

func (p *captureQualityEventPublisher) PublishQualityEvent(_ context.Context, event quality.QualityEvent) error {
	p.events = append(p.events, event)
	return nil
}

func TestOutboxFlushDecodesDomainEventPayload(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:quality-outbox?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&quality.QualityOutboxEventRecord{}))
	repo := quality.NewRepository(db)
	ctx := context.Background()

	payload := quality.QualityOutboxEvent{
		EventType:        quality.OutboxEventEvaluationCompleted,
		ExecutionID:      "exec-1",
		ProcessingTaskID: "task-1",
		ActionKey:        "idle",
		EvaluationID:     "eval-1",
		Status:           string(quality.EvalSucceeded),
		Verdict:          "accepted",
	}
	payloadJSON, err := json.Marshal(payload)
	require.NoError(t, err)
	require.NoError(t, repo.CreateOutboxEvent(ctx, &quality.QualityOutboxEventRecord{
		EventType:   payload.EventType,
		PayloadJSON: string(payloadJSON),
		Status:      "pending",
	}))

	capture := &captureQualityEventPublisher{}
	publisher := NewOutboxPublisher(repo, capture)
	require.NoError(t, publisher.Flush(ctx))
	require.Len(t, capture.events, 1)
	require.Equal(t, "exec-1", capture.events[0].JobID)
	require.Equal(t, "evaluation_completed", capture.events[0].Stage)
	require.Equal(t, 100, capture.events[0].Progress)
	require.Equal(t, "eval-1", capture.events[0].EvaluationID)

	pending, err := repo.ListPendingOutboxEvents(ctx, 10)
	require.NoError(t, err)
	require.Empty(t, pending)
}
