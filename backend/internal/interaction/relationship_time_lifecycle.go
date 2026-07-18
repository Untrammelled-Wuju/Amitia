package interaction

import (
	"context"
	"log"
	"time"

	"github.com/u-ai/backend/internal/temporal"
)

type RelationshipTimeCoordinator interface {
	PrepareInbound(ctx context.Context, input temporal.PrepareInboundInput) (temporal.RelationshipTimeContext, error)
	ReleaseClaim(ctx context.Context, interactionID string, reason string) error
}

func (o *Orchestrator) SetRelationshipTimeCoordinator(coordinator RelationshipTimeCoordinator) {
	o.relationshipTimeMu.Lock()
	o.relationshipTime = coordinator
	o.relationshipTimeMu.Unlock()
}

func (o *Orchestrator) prepareRelationshipTime(ctx context.Context, record *InteractionRecord, req *ProcessRequest) (*temporal.RelationshipTimeContext, error) {
	coordinator := o.relationshipTimeCoordinator()
	if coordinator == nil || record == nil || req == nil {
		return nil, nil
	}
	result, err := coordinator.PrepareInbound(ctx, temporal.PrepareInboundInput{
		UserID:         record.Scope.UserID,
		CharacterID:    record.Scope.CharacterID,
		ConversationID: record.Scope.ConversationID,
		Channel:        record.Scope.Channel,
		PeerID:         record.Scope.PeerID,
		RequestID:      record.Scope.RequestID,
		InteractionID:  record.ID,
		IsInternal:     req.IsInternal,
		Source:         record.Scope.Source,
	})
	if err != nil {
		return nil, err
	}
	if result.Reunion != nil && result.Reunion.State == temporal.ReunionStateClaimed && result.Reunion.ClaimedByInteractionID == record.ID {
		o.relationshipTimeMu.Lock()
		o.preparedRelationshipClaims[record.ID] = struct{}{}
		o.relationshipTimeMu.Unlock()
	}
	return &result, nil
}

func (o *Orchestrator) attachRelationshipTime(runtime *RuntimeAssembly, prepared *temporal.RelationshipTimeContext) {
	if runtime == nil || prepared == nil || runtime.Context.Temporal.Status != LoadStatusReady {
		return
	}
	runtime.Context.Temporal.Value.RelationshipTime = prepared
}

func (o *Orchestrator) releasePreparedRelationshipClaim(ctx context.Context, interactionID, reason string) {
	coordinator := o.relationshipTimeCoordinator()
	if coordinator == nil || !o.hasPreparedRelationshipClaim(interactionID) {
		return
	}
	if err := coordinator.ReleaseClaim(ctx, interactionID, reason); err != nil {
		log.Printf("[orchestrator] release relationship-time claim failed interaction=%s reason=%s: %v", interactionID, reason, err)
		return
	}
	o.forgetPreparedRelationshipClaim(interactionID)
}

func (o *Orchestrator) releaseRelationshipClaimIfUncommitted(ctx context.Context, interactionID, reason string) {
	if !o.hasPreparedRelationshipClaim(interactionID) {
		return
	}
	cleanupCtx, cancel := relationshipTimeCleanupContext(ctx)
	defer cancel()
	record, ok, err := o.tracker.Get(cleanupCtx, interactionID)
	if err != nil {
		log.Printf("[orchestrator] inspect relationship-time claim failed interaction=%s: %v", interactionID, err)
		return
	}
	if ok && interactionHasCommitted(record.Status) {
		o.forgetPreparedRelationshipClaim(interactionID)
		return
	}
	o.releasePreparedRelationshipClaim(cleanupCtx, interactionID, reason)
}

func relationshipTimeCleanupContext(ctx context.Context) (context.Context, context.CancelFunc) {
	if ctx == nil {
		ctx = context.Background()
	} else {
		ctx = context.WithoutCancel(ctx)
	}
	return context.WithTimeout(ctx, 5*time.Second)
}

func (o *Orchestrator) relationshipTimeCoordinator() RelationshipTimeCoordinator {
	o.relationshipTimeMu.Lock()
	defer o.relationshipTimeMu.Unlock()
	return o.relationshipTime
}

func (o *Orchestrator) hasPreparedRelationshipClaim(interactionID string) bool {
	o.relationshipTimeMu.Lock()
	defer o.relationshipTimeMu.Unlock()
	_, ok := o.preparedRelationshipClaims[interactionID]
	return ok
}

func (o *Orchestrator) forgetPreparedRelationshipClaim(interactionID string) {
	o.relationshipTimeMu.Lock()
	delete(o.preparedRelationshipClaims, interactionID)
	o.relationshipTimeMu.Unlock()
}

func interactionHasCommitted(status InteractionStatus) bool {
	switch status {
	case InteractionStatusCommitted, InteractionStatusDeliveryPending, InteractionStatusDelivered, InteractionStatusCompleted:
		return true
	default:
		return false
	}
}
