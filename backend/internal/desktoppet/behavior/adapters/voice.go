package adapters

import (
	"context"

	"github.com/u-ai/backend/internal/desktoppet/behavior"
	"github.com/u-ai/backend/internal/desktoppet/behavior/events"
	"github.com/u-ai/backend/log"
)

type VoiceAdapter struct {
	publisher behavior.BehaviorEventPublisher
	clock     behavior.Clock
}

func NewVoiceAdapter(publisher behavior.BehaviorEventPublisher, clock behavior.Clock) *VoiceAdapter {
	return &VoiceAdapter{publisher: publisher, clock: clock}
}

func (a *VoiceAdapter) OnVoiceLifecycle(ctx context.Context, evt behavior.VoiceLifecycleEvent) {
	if a == nil || a.publisher == nil {
		return
	}
	now := a.clock.Now()
	envelope := events.EnvelopeFromVoiceEvent(evt, now)
	if err := a.publisher.PublishBehaviorEvent(ctx, envelope); err != nil {
		log.Warn("voice adapter: publish failed", map[string]interface{}{
			"error":       err.Error(),
			"eventType":   envelope.EventType,
			"characterId": envelope.CharacterID,
		})
	}
}

type NoopVoiceAdapter struct{}

func (n *NoopVoiceAdapter) OnVoiceLifecycle(_ context.Context, _ behavior.VoiceLifecycleEvent) {}
