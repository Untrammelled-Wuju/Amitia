package adapters

import (
	"context"
	"fmt"

	"github.com/u-ai/backend/internal/desktoppet/behavior"
	"github.com/u-ai/backend/internal/desktoppet/behavior/events"
	"github.com/u-ai/backend/log"
)

type PlaybackAdapter struct {
	publisher behavior.BehaviorEventPublisher
	clock     behavior.Clock
	petInfo   PetInfoPort
}

func NewPlaybackAdapter(publisher behavior.BehaviorEventPublisher, clock behavior.Clock, petInfo PetInfoPort) *PlaybackAdapter {
	return &PlaybackAdapter{publisher: publisher, clock: clock, petInfo: petInfo}
}

func (a *PlaybackAdapter) OnPlaybackFeedback(ctx context.Context, feedback behavior.PlaybackFeedback) {
	if a == nil || a.publisher == nil {
		return
	}
	userID := ""
	characterID := ""
	if a.petInfo != nil {
		userID, characterID = a.petInfo.ResolvePetInfo(ctx, feedback.PetInstanceID)
	}
	now := a.clock.Now()
	occurredAt := feedback.OccurredAt
	if occurredAt.IsZero() {
		occurredAt = now
	}

	eventType := "playback.action." + string(feedback.Phase)
	builder := events.NewEnvelope(eventType, behavior.OriginPlayback).
		UserID(userID).
		CharacterID(characterID).
		PetInstanceID(feedback.PetInstanceID).
		OccurredAt(occurredAt).
		DedupKey(events.BuildDedupKey(feedback.CommandID, string(feedback.Phase), fmt.Sprintf("s%d", feedback.Sequence)))

	builder.PayloadField("commandId", feedback.CommandID)
	if feedback.DecisionID != "" {
		builder.PayloadField("decisionId", feedback.DecisionID)
	}
	if feedback.ActionKey != "" {
		builder.PayloadField("actionKey", feedback.ActionKey)
	}
	if feedback.ErrorClass != "" {
		builder.PayloadField("errorClass", feedback.ErrorClass)
	}

	envelope := builder.Build(now)
	if err := a.publisher.PublishBehaviorEvent(ctx, envelope); err != nil {
		log.Warn("playback adapter: publish failed", map[string]interface{}{
			"error":       err.Error(),
			"eventType":   envelope.EventType,
			"characterId": characterID,
		})
	}
}

type NoopPlaybackAdapter struct{}

func (n *NoopPlaybackAdapter) OnPlaybackFeedback(_ context.Context, _ behavior.PlaybackFeedback) {}
