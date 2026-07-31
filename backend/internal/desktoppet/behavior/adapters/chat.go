package adapters

import (
	"context"

	"github.com/u-ai/backend/internal/desktoppet/behavior"
	"github.com/u-ai/backend/internal/desktoppet/behavior/events"
	"github.com/u-ai/backend/log"
)

type ChatAdapter struct {
	publisher behavior.BehaviorEventPublisher
	clock     behavior.Clock
}

func NewChatAdapter(publisher behavior.BehaviorEventPublisher, clock behavior.Clock) *ChatAdapter {
	return &ChatAdapter{publisher: publisher, clock: clock}
}

func (a *ChatAdapter) OnChatLifecycle(ctx context.Context, evt behavior.ChatLifecycleEvent) {
	if a == nil || a.publisher == nil {
		return
	}
	now := a.clock.Now()
	envelope := events.EnvelopeFromChatEvent(evt, now)
	if err := a.publisher.PublishBehaviorEvent(ctx, envelope); err != nil {
		log.Warn("chat adapter: publish failed", map[string]interface{}{
			"error":       err.Error(),
			"eventType":   envelope.EventType,
			"characterId": envelope.CharacterID,
		})
	}
}

type NoopChatAdapter struct{}

func (n *NoopChatAdapter) OnChatLifecycle(_ context.Context, _ behavior.ChatLifecycleEvent) {}
