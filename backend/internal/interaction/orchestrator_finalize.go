package interaction

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/outbox"
	"gorm.io/gorm"
)

func (o *Orchestrator) finalizeProcessorSuccess(ctx context.Context, record *InteractionRecord, req *ProcessRequest, resp *ProcessResponse, runtime RuntimeAssembly, duration time.Duration) (*OrchestrationResult, error) {
	if resp == nil {
		err := errors.New("orchestrator: processor returned nil response")
		if failed, failErr := o.tracker.Fail(ctx, record.ID, record.StatusVersion, "processor_response_missing", err.Error()); failErr == nil {
			record = failed
		}
		result := o.buildResult(record, nil, OutcomeFailed, err)
		result.Duration = duration
		o.releaseRelationshipClaimIfUncommitted(ctx, record.ID, "processor_response_missing")
		return result, err
	}
	if record.Status == InteractionStatusCommitted || record.Status == InteractionStatusCompleted {
		return o.successResult(record, req, resp, resp.Events, duration), nil
	}
	if record.Status != InteractionStatusContextReady || record.StatusVersion != req.ExpectedStatusVersion {
		return o.completionConflictResult(ctx, record, req, resp, duration, ErrVersionConflict)
	}
	expectedVersion := req.ExpectedStatusVersion
	updated, err := o.tracker.UpdateMetadata(ctx, record.ID, InteractionMetadataUpdate{
		CommitID:              metadataFromResponse(resp).CommitID,
		ExpectedStatusVersion: &expectedVersion,
	})
	if err != nil {
		return o.completionConflictResult(ctx, record, req, resp, duration, err)
	}
	record, err = o.tracker.TransitionCAS(ctx, updated.ID, expectedVersion, InteractionStatusCommitted)
	if err != nil {
		return o.completionConflictResult(ctx, updated, req, resp, duration, err)
	}
	o.forgetPreparedRelationshipClaim(record.ID)
	events := resp.Events
	if len(events) == 0 {
		events, err = buildFallbackOutboxEvents(record, resp, runtime)
		if err != nil {
			result := o.buildResult(record, nil, OutcomeFailed, err)
			result.Duration = duration
			return result, err
		}
	}
	completed, err := o.completeWithOutbox(ctx, record, resp.Reply, events)
	if err != nil {
		return o.completionConflictResult(ctx, record, req, resp, duration, err)
	}
	return o.successResult(completed, req, resp, events, duration), nil
}

func (o *Orchestrator) completionConflictResult(ctx context.Context, record *InteractionRecord, req *ProcessRequest, resp *ProcessResponse, duration time.Duration, conflict error) (*OrchestrationResult, error) {
	resolved, outcome, err := o.resolveCompletionConflict(ctx, record.ID, record, conflict)
	if outcome == OutcomeCompleted && err == nil {
		o.forgetPreparedRelationshipClaim(record.ID)
		return o.successResult(resolved, req, resp, resp.Events, duration), nil
	}
	o.releaseRelationshipClaimIfUncommitted(ctx, record.ID, "completion_conflict")
	result := o.buildResult(resolved, nil, outcome, err)
	result.Duration = duration
	return result, err
}

func (o *Orchestrator) successResult(record *InteractionRecord, req *ProcessRequest, resp *ProcessResponse, events []outbox.OutboxRecord, duration time.Duration) *OrchestrationResult {
	resp.RequestID = req.RequestID
	resp.ConversationID = req.ConversationID
	resp.CharacterID = req.CharacterID
	result := o.buildResult(record, resp, OutcomeCompleted, nil)
	result.Duration = duration
	result.Events = events
	return result
}

func (o *Orchestrator) completeWithOutbox(ctx context.Context, record *InteractionRecord, resultRef string, events []outbox.OutboxRecord) (*InteractionRecord, error) {
	if o.outbox == nil {
		return o.tracker.Complete(ctx, record.ID, record.StatusVersion, resultRef)
	}
	tracker, ok := o.tracker.(*SQLiteInteractionTracker)
	if !ok || tracker.db == nil || o.outbox == nil || tracker.db.ConnPool != o.outbox.DB().ConnPool {
		for i := range events {
			if err := o.outbox.Append(events[i]); err != nil {
				return nil, err
			}
		}
		return o.tracker.Complete(ctx, record.ID, record.StatusVersion, resultRef)
	}
	var completed *InteractionRecord
	err := tracker.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		txTracker := NewSQLiteInteractionTracker(tx)
		var innerErr error
		completed, innerErr = txTracker.Complete(ctx, record.ID, record.StatusVersion, resultRef)
		if innerErr != nil {
			return innerErr
		}
		for i := range events {
			if innerErr = o.outbox.AppendWithTx(tx, events[i]); innerErr != nil {
				return innerErr
			}
		}
		return nil
	})
	return completed, err
}

func buildFallbackOutboxEvents(record *InteractionRecord, resp *ProcessResponse, runtime RuntimeAssembly) ([]outbox.OutboxRecord, error) {
	now := time.Now()
	completedPayload, err := json.Marshal(map[string]interface{}{
		"interactionId": record.ID,
		"scope":         record.Scope,
		"reply":         resp.Reply,
		"messageIds":    resp.MessageIDs,
	})
	if err != nil {
		return nil, fmt.Errorf("orchestrator: encode completed event: %w", err)
	}
	statePayload, err := json.Marshal(map[string]interface{}{
		"interactionId":  record.ID,
		"conversationId": record.Scope.ConversationID,
		"characterId":    record.Scope.CharacterID,
		"channel":        record.Scope.Channel,
		"status":         string(InteractionStatusCompleted),
		"timestamp":      now,
	})
	if err != nil {
		return nil, fmt.Errorf("orchestrator: encode state event: %w", err)
	}
	runtimePayload, err := json.Marshal(map[string]interface{}{
		"interactionId": record.ID,
		"scope":         record.Scope,
		"path":          runtime.Path,
		"safety":        runtime.Safety,
		"delivery":      runtime.Delivery,
		"timestamp":     now,
	})
	if err != nil {
		return nil, fmt.Errorf("orchestrator: encode runtime event: %w", err)
	}
	return []outbox.OutboxRecord{
		newFallbackOutboxRecord(record.ID, "interaction.completed", completedPayload, now),
		newFallbackOutboxRecord(record.ID, "interaction.state_changed", statePayload, now),
		newFallbackOutboxRecord(record.ID, "interaction.runtime_assembled", runtimePayload, now),
	}, nil
}

func newFallbackOutboxRecord(aggregateID, eventType string, payload []byte, now time.Time) outbox.OutboxRecord {
	return outbox.OutboxRecord{
		ID:          uuid.New().String(),
		AggregateID: aggregateID,
		EventType:   eventType,
		Payload:     payload,
		Status:      outbox.OutboxStatusPending,
		MaxRetries:  outbox.DefaultMaxRetries,
		AvailableAt: now,
		CreatedAt:   now,
	}
}
