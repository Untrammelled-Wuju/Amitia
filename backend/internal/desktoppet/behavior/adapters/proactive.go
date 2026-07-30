package adapters

import (
	"context"
	"time"

	"github.com/u-ai/backend/internal/desktoppet/behavior"
	"github.com/u-ai/backend/internal/desktoppet/behavior/events"
	"github.com/u-ai/backend/log"
)

type ProactiveEvent struct {
	ProactiveID    string
	CharacterID    string
	UserID         string
	ConversationID string
	CorrelationID  string
	RuleID         string
	Intent         string
	Phase          string
	InteractionID  string
	Reason         string
	OccurredAt     time.Time
}

type ProactiveObserver interface {
	OnProactiveEvent(ctx context.Context, event ProactiveEvent)
}

type ProactiveAdapter struct {
	publisher behavior.BehaviorEventPublisher
	clock     behavior.Clock
}

func NewProactiveAdapter(publisher behavior.BehaviorEventPublisher, clock behavior.Clock) *ProactiveAdapter {
	return &ProactiveAdapter{publisher: publisher, clock: clock}
}

func (a *ProactiveAdapter) OnProactiveEvent(ctx context.Context, evt ProactiveEvent) {
	if a == nil || a.publisher == nil {
		return
	}
	now := a.clock.Now()
	occurredAt := evt.OccurredAt
	if occurredAt.IsZero() {
		occurredAt = now
	}

	eventType := "proactive.message." + evt.Phase
	builder := events.NewEnvelope(eventType, behavior.OriginProactive).
		UserID(evt.UserID).
		CharacterID(evt.CharacterID).
		OccurredAt(occurredAt).
		DedupKey(events.BuildDedupKey(evt.ProactiveID, evt.Phase))

	if evt.ConversationID != "" {
		builder.ConversationID(evt.ConversationID)
	}
	if evt.CorrelationID != "" {
		builder.CorrelationID(evt.CorrelationID)
		builder.PayloadField("correlationId", evt.CorrelationID)
	}
	if evt.InteractionID != "" {
		builder.InteractionID(evt.InteractionID)
		builder.PayloadField("interactionId", evt.InteractionID)
	}
	if evt.RuleID != "" {
		builder.PayloadField("ruleId", evt.RuleID)
	}
	if evt.Intent != "" {
		builder.PayloadField("intent", evt.Intent)
	}
	if evt.Reason != "" {
		builder.PayloadField("reason", evt.Reason)
	}

	envelope := builder.Build(now)
	if err := a.publisher.PublishBehaviorEvent(ctx, envelope); err != nil {
		log.Warn("proactive adapter: publish failed", map[string]interface{}{
			"error":       err.Error(),
			"eventType":   envelope.EventType,
			"characterId": evt.CharacterID,
		})
	}
}

type NoopProactiveAdapter struct{}

func (n *NoopProactiveAdapter) OnProactiveEvent(_ context.Context, _ ProactiveEvent) {}
