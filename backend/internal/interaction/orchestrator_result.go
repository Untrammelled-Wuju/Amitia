package interaction

import (
	"context"
	"errors"
)

func (o *Orchestrator) resolveCompletionConflict(ctx context.Context, id string, fallback *InteractionRecord, err error) (*InteractionRecord, Outcome, error) {
	if !isInteractionConflictError(err) {
		return fallback, OutcomeFailed, err
	}
	rec, ok, getErr := o.tracker.Get(ctx, id)
	if getErr != nil {
		return fallback, OutcomeFailed, getErr
	}
	if !ok {
		return fallback, OutcomeFailed, ErrInteractionNotFound
	}
	switch rec.Status {
	case InteractionStatusCancelled:
		return rec, OutcomeCancelled, ErrOrchestratorCancelled
	case InteractionStatusSuperseded:
		return rec, OutcomeSuperseded, ErrOrchestratorSuperseded
	case InteractionStatusCompleted:
		return rec, OutcomeCompleted, nil
	}
	if !rec.CancelRequestedAt.IsZero() {
		cancelled, cancelErr := o.tracker.TransitionCAS(ctx, rec.ID, rec.StatusVersion, InteractionStatusCancelled)
		if cancelErr == nil {
			return cancelled, OutcomeCancelled, ErrOrchestratorCancelled
		}
		if isInteractionConflictError(cancelErr) {
			return o.resolveCompletionConflict(ctx, id, rec, cancelErr)
		}
		return rec, OutcomeFailed, cancelErr
	}
	return rec, OutcomeFailed, err
}

func isInteractionConflictError(err error) bool {
	return errors.Is(err, ErrVersionConflict) || errors.Is(err, ErrInteractionCASConflict) || errors.Is(err, ErrAlreadyTerminal) || errors.Is(err, ErrInvalidTransition)
}

func (o *Orchestrator) handleIdempotentHit(existing *InteractionRecord) (*OrchestrationResult, error) {
	outcome := outcomeForRecord(existing)
	switch existing.Status {
	case InteractionStatusCompleted, InteractionStatusCommitted, InteractionStatusDeliveryPending, InteractionStatusDelivered:
		var resp *ProcessResponse
		if existing.ResultRef != "" {
			resp = &ProcessResponse{RequestID: existing.Scope.RequestID, ConversationID: existing.Scope.ConversationID, CharacterID: existing.Scope.CharacterID, Reply: existing.ResultRef}
		}
		return o.buildResult(existing, resp, outcome, nil), nil
	case InteractionStatusReceived, InteractionStatusNormalized, InteractionStatusQueued, InteractionStatusProcessing, InteractionStatusContextReady, InteractionStatusDecided, InteractionStatusGenerated:
		return o.buildResult(existing, nil, outcome, nil), ErrOrchestratorProcessing
	case InteractionStatusFailed, InteractionStatusCancelled, InteractionStatusSuperseded:
		return o.buildResult(existing, nil, outcome, nil), ErrOrchestratorDuplicate
	default:
		return o.buildResult(existing, nil, outcome, nil), ErrOrchestratorDuplicate
	}
}

func (o *Orchestrator) buildResult(record *InteractionRecord, resp *ProcessResponse, outcome Outcome, err error) *OrchestrationResult {
	r := &OrchestrationResult{Outcome: outcome, Response: resp}
	if record != nil {
		r.InteractionID = record.ID
	}
	if err != nil {
		r.Error = err.Error()
	}
	return r
}

func outcomeForRecord(record *InteractionRecord) Outcome {
	if record == nil {
		return OutcomeFailed
	}
	switch record.Status {
	case InteractionStatusCompleted:
		return OutcomeCompleted
	case InteractionStatusCommitted, InteractionStatusDeliveryPending, InteractionStatusDelivered:
		return OutcomeDeliveryUnknown
	case InteractionStatusCancelled:
		return OutcomeCancelled
	case InteractionStatusSuperseded:
		return OutcomeSuperseded
	default:
		return OutcomeFailed
	}
}
