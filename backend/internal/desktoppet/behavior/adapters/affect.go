package adapters

import (
	"context"
	"math"

	"github.com/u-ai/backend/internal/desktoppet/behavior"
	"github.com/u-ai/backend/internal/desktoppet/behavior/events"
	"github.com/u-ai/backend/log"
)

type AffectAdapter struct {
	publisher     behavior.BehaviorEventPublisher
	clock         behavior.Clock
	ownerResolver CharacterOwnerPort
}

func NewAffectAdapter(publisher behavior.BehaviorEventPublisher, clock behavior.Clock, ownerResolver CharacterOwnerPort) *AffectAdapter {
	return &AffectAdapter{publisher: publisher, clock: clock, ownerResolver: ownerResolver}
}

func (a *AffectAdapter) OnAffectChanged(ctx context.Context, characterID string, old, new behavior.AffectBehaviorSnapshot) {
	if a == nil || a.publisher == nil {
		return
	}
	userID := ""
	if a.ownerResolver != nil {
		userID = a.ownerResolver.ResolveUserID(ctx, characterID)
	}
	now := a.clock.Now()
	occurredAt := new.UpdatedAt
	if occurredAt.IsZero() {
		occurredAt = now
	}

	deltaMag := math.Sqrt(
		math.Pow(new.Valence-old.Valence, 2) +
			math.Pow(new.Arousal-old.Arousal, 2) +
			math.Pow(new.Tension-old.Tension, 2) +
			math.Pow(new.Stress-old.Stress, 2),
	)

	builder := events.NewEnvelope("character.affect.changed", behavior.OriginAffect).
		UserID(userID).
		CharacterID(characterID).
		OccurredAt(occurredAt).
		DedupKey(events.BuildDedupKey(characterID, "affect.changed", new.Version))

	builder.PayloadField("version", new.Version)
	builder.PayloadField("valence", new.Valence)
	builder.PayloadField("arousal", new.Arousal)
	builder.PayloadField("tension", new.Tension)
	builder.PayloadField("stress", new.Stress)
	if new.Label != "" {
		builder.PayloadField("label", new.Label)
	}
	builder.PayloadField("confidence", new.Confidence)
	if old.Label != "" {
		builder.PayloadField("prevLabel", old.Label)
	}
	builder.PayloadField("deltaMagnitude", deltaMag)

	envelope := builder.Build(now)
	if err := a.publisher.PublishBehaviorEvent(ctx, envelope); err != nil {
		log.Warn("affect adapter: publish failed", map[string]interface{}{
			"error":       err.Error(),
			"characterId": characterID,
		})
	}
}

type NoopAffectAdapter struct{}

func (n *NoopAffectAdapter) OnAffectChanged(_ context.Context, _ string, _, _ behavior.AffectBehaviorSnapshot) {}
