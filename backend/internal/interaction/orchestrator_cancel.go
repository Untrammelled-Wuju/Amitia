package interaction

import (
	"context"
	"errors"
	"log"
	"time"
)

func (o *Orchestrator) Cancel(interactionID string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	rec, ok, err := o.tracker.Get(ctx, interactionID)
	if err != nil {
		return err
	}
	if !ok {
		return errors.New("orchestrator: interaction not found")
	}
	if rec.IsTerminal() {
		return nil
	}
	if rec.Status == InteractionStatusCommitted || rec.Status == InteractionStatusDeliveryPending || rec.Status == InteractionStatusDelivered {
		return o.writeCompensationEvent(ctx, rec, "cancel_after_commit")
	}
	if err := o.tracker.RequestCancel(ctx, interactionID, "cancel_requested"); err != nil {
		return o.resolveCancelConflict(ctx, interactionID, err)
	}
	o.cancels.Cancel(interactionID)
	fresh, ok, err := o.tracker.Get(ctx, interactionID)
	if err != nil {
		return err
	}
	if !ok {
		return errors.New("orchestrator: interaction not found")
	}
	_, err = o.tracker.TransitionCAS(ctx, fresh.ID, fresh.StatusVersion, InteractionStatusCancelled)
	if err != nil {
		return o.resolveCancelConflict(ctx, interactionID, err)
	}
	return nil
}

func (o *Orchestrator) CancelByScope(scope InteractionScope) int {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	active, err := o.tracker.ListActive(ctx, scope)
	if err != nil {
		log.Printf("[orchestrator] list active failed during cancel: %v", err)
		return 0
	}
	count := 0
	for _, rec := range active {
		if err := o.tracker.RequestCancel(ctx, rec.ID, "scope_cancel_requested"); err != nil {
			log.Printf("[orchestrator] request cancel failed: %v", err)
			continue
		}
		if o.cancels.Cancel(rec.ID) {
			count++
		}
		fresh, ok, err := o.tracker.Get(ctx, rec.ID)
		if err != nil {
			log.Printf("[orchestrator] reload after cancel request failed: %v", err)
			continue
		}
		if !ok {
			log.Printf("[orchestrator] cancelled record disappeared: %s", rec.ID)
			continue
		}
		if _, err := o.tracker.TransitionCAS(ctx, fresh.ID, fresh.StatusVersion, InteractionStatusCancelled); err != nil {
			if resolveErr := o.resolveCancelConflict(ctx, rec.ID, err); resolveErr != nil {
				log.Printf("[orchestrator] cancel transition failed: %v", resolveErr)
			}
		}
	}
	return count
}

func (o *Orchestrator) writeCompensationEvent(ctx context.Context, rec *InteractionRecord, reason string) error {
	log.Printf("[orchestrator] compensation event for committed interaction %s: %s (compensation outbox deferred)", rec.ID, reason)
	return nil
}

func (o *Orchestrator) supersedeTarget(ctx context.Context, targetID, newID string) error {
	rec, ok, err := o.tracker.Get(ctx, targetID)
	if err != nil {
		return err
	}
	if ok && (rec.Status == InteractionStatusCommitted || rec.Status == InteractionStatusDeliveryPending || rec.Status == InteractionStatusDelivered) {
		return o.writeCompensationEvent(ctx, rec, "supersede_after_commit:"+newID)
	}
	if err := o.tracker.MarkSuperseded(ctx, targetID, newID); err != nil {
		return err
	}
	o.cancels.Cancel(targetID)
	return nil
}

func (o *Orchestrator) resolveCancelConflict(ctx context.Context, id string, err error) error {
	if !isInteractionConflictError(err) {
		return err
	}
	rec, ok, getErr := o.tracker.Get(ctx, id)
	if getErr != nil {
		return getErr
	}
	if !ok {
		return ErrInteractionNotFound
	}
	if rec.IsTerminal() || !canSupersedeStatus(rec.Status) {
		return nil
	}
	return err
}
