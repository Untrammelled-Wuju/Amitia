package interaction

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

func (o *Orchestrator) waitForQueueTurn(ctx context.Context, scope InteractionScope, record *InteractionRecord) (*InteractionRecord, error) {
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		fresh, _, err := o.ensureFresh(ctx, record.ID)
		if err != nil {
			return fresh, err
		}
		record = fresh
		older, err := o.hasOlderActive(ctx, scope, record)
		if err != nil {
			return record, err
		}
		if !older {
			return record, nil
		}
		select {
		case <-ctx.Done():
			return record, ctx.Err()
		case <-ticker.C:
		}
	}
}

func (o *Orchestrator) hasOlderActive(ctx context.Context, scope InteractionScope, record *InteractionRecord) (bool, error) {
	active, err := o.tracker.ListActive(ctx, scope)
	if err != nil {
		return false, err
	}
	for _, rec := range active {
		if rec.ID == record.ID || !sameSupersedeScope(scope, rec.Scope) {
			continue
		}
		if rec.CreatedAt.Before(record.CreatedAt) || (rec.CreatedAt.Equal(record.CreatedAt) && rec.ID < record.ID) {
			return true, nil
		}
	}
	return false, nil
}

func (o *Orchestrator) acquireQueueScope(scope InteractionScope) func() {
	key := queueScopeKey(scope)
	o.queueMu.Lock()
	lock := o.queueLocks[key]
	if lock == nil {
		lock = &sync.Mutex{}
		o.queueLocks[key] = lock
	}
	o.queueMu.Unlock()
	lock.Lock()
	return lock.Unlock
}

func queueScopeKey(scope InteractionScope) string {
	scope = scope.Normalize()
	return strings.Join([]string{scope.UserID, scope.CharacterID, scope.ConversationID, scope.Channel, scope.PeerID}, "\x00")
}

func outcomeFromQueueWaitError(err error) Outcome {
	switch {
	case errors.Is(err, ErrOrchestratorCancelled), errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return OutcomeCancelled
	case errors.Is(err, ErrOrchestratorSuperseded):
		return OutcomeSuperseded
	default:
		return OutcomeFailed
	}
}

func (o *Orchestrator) ensureFresh(ctx context.Context, id string) (*InteractionRecord, Outcome, error) {
	rec, ok, err := o.tracker.Get(ctx, id)
	if err != nil {
		return nil, OutcomeFailed, err
	}
	if !ok {
		return nil, OutcomeFailed, ErrInteractionNotFound
	}
	switch rec.Status {
	case InteractionStatusCancelled:
		return rec, OutcomeCancelled, ErrOrchestratorCancelled
	case InteractionStatusSuperseded:
		return rec, OutcomeSuperseded, ErrOrchestratorSuperseded
	}
	if !rec.CancelRequestedAt.IsZero() {
		cancelled, err := o.tracker.TransitionCAS(ctx, rec.ID, rec.StatusVersion, InteractionStatusCancelled)
		if err != nil {
			return rec, OutcomeFailed, err
		}
		return cancelled, OutcomeCancelled, ErrOrchestratorCancelled
	}
	return rec, OutcomeCompleted, nil
}

func (o *Orchestrator) ensureFreshAtVersion(ctx context.Context, id string, expectedVersion int64) (*InteractionRecord, Outcome, error) {
	rec, outcome, err := o.ensureFresh(ctx, id)
	if err != nil {
		return rec, outcome, err
	}
	if rec.StatusVersion != expectedVersion {
		return rec, OutcomeFailed, ErrVersionConflict
	}
	if err := ctx.Err(); err != nil {
		return rec, OutcomeCancelled, fmt.Errorf("context expired before version check: %w", err)
	}
	return rec, outcome, nil
}
