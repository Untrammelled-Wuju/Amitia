package adapters

import (
	"context"
	"fmt"
	"time"

	"github.com/u-ai/backend/internal/desktoppet/behavior"
	"github.com/u-ai/backend/internal/desktoppet/behavior/events"
	"github.com/u-ai/backend/log"
)

type InteractionAdapter struct {
	publisher behavior.BehaviorEventPublisher
	clock     behavior.Clock
}

func NewInteractionAdapter(publisher behavior.BehaviorEventPublisher, clock behavior.Clock) *InteractionAdapter {
	return &InteractionAdapter{publisher: publisher, clock: clock}
}

func (a *InteractionAdapter) OnInteractionLifecycle(ctx context.Context, evt behavior.InteractionLifecycleEvent) {
	if a == nil || a.publisher == nil {
		return
	}
	now := a.clock.Now()
	envelope := buildInteractionEnvelope(evt, now)
	if err := a.publisher.PublishBehaviorEvent(ctx, envelope); err != nil {
		log.Warn("interaction adapter: publish failed", map[string]interface{}{
			"error":       err.Error(),
			"eventType":   envelope.EventType,
			"characterId": envelope.CharacterID,
		})
	}
}

func buildInteractionEnvelope(evt behavior.InteractionLifecycleEvent, now time.Time) behavior.BehaviorEventEnvelope {
	builder := events.NewEnvelope(evt.Phase, behavior.OriginInteraction).
		UserID(evt.UserID).
		CharacterID(evt.CharacterID).
		InteractionID(evt.InteractionID).
		OccurredAt(evt.OccurredAt).
		DedupKey(events.BuildDedupKey(evt.InteractionID, evt.Phase, fmt.Sprintf("v%d", evt.StatusVersion)))

	if evt.ConversationID != "" {
		builder.ConversationID(evt.ConversationID)
	}
	if evt.CorrelationID != "" {
		builder.CorrelationID(evt.CorrelationID)
	}
	if evt.Origin != "" {
		builder.PayloadField("origin", evt.Origin)
	}
	builder.PayloadField("statusVersion", evt.StatusVersion)
	return builder.Build(now)
}

type NoopInteractionAdapter struct{}

func (n *NoopInteractionAdapter) OnInteractionLifecycle(_ context.Context, _ behavior.InteractionLifecycleEvent) {}
