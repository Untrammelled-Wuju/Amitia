package adapters

import (
	"context"

	"github.com/u-ai/backend/internal/desktoppet/behavior"
	"github.com/u-ai/backend/internal/desktoppet/behavior/events"
	"github.com/u-ai/backend/log"
)

type DesktopAdapter struct {
	publisher behavior.BehaviorEventPublisher
	clock     behavior.Clock
}

func NewDesktopAdapter(publisher behavior.BehaviorEventPublisher, clock behavior.Clock) *DesktopAdapter {
	return &DesktopAdapter{publisher: publisher, clock: clock}
}

func (a *DesktopAdapter) OnDesktopGesture(ctx context.Context, evt behavior.DesktopGestureEvent) {
	if a == nil || a.publisher == nil {
		return
	}
	now := a.clock.Now()
	envelope := events.EnvelopeFromDesktopEvent(evt, now)
	if err := a.publisher.PublishBehaviorEvent(ctx, envelope); err != nil {
		log.Warn("desktop adapter: publish failed", map[string]interface{}{
			"error":       err.Error(),
			"eventType":   envelope.EventType,
			"characterId": envelope.CharacterID,
		})
	}
}

type NoopDesktopAdapter struct{}

func (n *NoopDesktopAdapter) OnDesktopGesture(_ context.Context, _ behavior.DesktopGestureEvent) {}
