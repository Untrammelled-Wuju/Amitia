package adapters

import (
	"context"
	"time"

	"github.com/u-ai/backend/internal/desktoppet/behavior"
	"github.com/u-ai/backend/internal/desktoppet/behavior/events"
	"github.com/u-ai/backend/log"
)

type ActivityChangeEvent struct {
	CharacterID string
	UserID      string
	Old         behavior.ActivityBehaviorSnapshot
	New         behavior.ActivityBehaviorSnapshot
	OccurredAt  time.Time
}

type ActivityChangeObserver interface {
	OnActivityChanged(ctx context.Context, event ActivityChangeEvent)
}

type ActivityAdapter struct {
	publisher     behavior.BehaviorEventPublisher
	clock         behavior.Clock
	ownerResolver CharacterOwnerPort
}

func NewActivityAdapter(publisher behavior.BehaviorEventPublisher, clock behavior.Clock, ownerResolver CharacterOwnerPort) *ActivityAdapter {
	return &ActivityAdapter{publisher: publisher, clock: clock, ownerResolver: ownerResolver}
}

func (a *ActivityAdapter) OnActivityChanged(ctx context.Context, event ActivityChangeEvent) {
	if a == nil || a.publisher == nil {
		return
	}
	userID := event.UserID
	if userID == "" && a.ownerResolver != nil {
		userID = a.ownerResolver.ResolveUserID(ctx, event.CharacterID)
	}
	now := a.clock.Now()
	occurredAt := event.OccurredAt
	if occurredAt.IsZero() {
		occurredAt = now
	}

	builder := events.NewEnvelope("character.activity.changed", behavior.OriginActivity).
		UserID(userID).
		CharacterID(event.CharacterID).
		OccurredAt(occurredAt).
		DedupKey(events.BuildDedupKey(event.CharacterID, "activity.changed", event.New.Version))

	builder.PayloadField("activityKey", event.New.ActivityKey)
	builder.PayloadField("source", event.New.Source)
	builder.PayloadField("confidence", event.New.Confidence)
	builder.PayloadField("version", event.New.Version)

	envelope := builder.Build(now)
	if err := a.publisher.PublishBehaviorEvent(ctx, envelope); err != nil {
		log.Warn("activity adapter: publish failed", map[string]interface{}{
			"error":       err.Error(),
			"characterId": event.CharacterID,
		})
	}
}

type NoopActivityAdapter struct{}

func (n *NoopActivityAdapter) OnActivityChanged(_ context.Context, _ ActivityChangeEvent) {}
