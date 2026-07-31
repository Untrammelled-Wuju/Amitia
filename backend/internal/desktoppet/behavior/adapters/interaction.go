package adapters

import (
	"context"

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
	envelope := events.EnvelopeFromInteractionEvent(evt, now, behavior.OriginInteraction)
	if err := a.publisher.PublishBehaviorEvent(ctx, envelope); err != nil {
		log.Warn("interaction adapter: publish failed", map[string]interface{}{
			"error":       err.Error(),
			"eventType":   envelope.EventType,
			"characterId": envelope.CharacterID,
		})
	}
}

type NoopInteractionAdapter struct{}

func (n *NoopInteractionAdapter) OnInteractionLifecycle(_ context.Context, _ behavior.InteractionLifecycleEvent) {}
