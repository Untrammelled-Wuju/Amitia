package adapters

import (
	"context"

	"github.com/u-ai/backend/internal/desktoppet/behavior"
	"github.com/u-ai/backend/internal/desktoppet/behavior/events"
	"github.com/u-ai/backend/log"
)

type ToolAdapter struct {
	publisher behavior.BehaviorEventPublisher
	clock     behavior.Clock
}

func NewToolAdapter(publisher behavior.BehaviorEventPublisher, clock behavior.Clock) *ToolAdapter {
	return &ToolAdapter{publisher: publisher, clock: clock}
}

func (a *ToolAdapter) OnToolLifecycle(ctx context.Context, evt behavior.ToolLifecycleEvent) {
	if a == nil || a.publisher == nil {
		return
	}
	now := a.clock.Now()
	envelope := events.EnvelopeFromToolEvent(evt, now)
	if err := a.publisher.PublishBehaviorEvent(ctx, envelope); err != nil {
		log.Warn("tool adapter: publish failed", map[string]interface{}{
			"error":       err.Error(),
			"eventType":   envelope.EventType,
			"characterId": envelope.CharacterID,
		})
	}
}

type NoopToolAdapter struct{}

func (n *NoopToolAdapter) OnToolLifecycle(_ context.Context, _ behavior.ToolLifecycleEvent) {}
