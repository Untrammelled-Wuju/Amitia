package main

import (
	"context"

	"github.com/u-ai/backend/internal/chat"
	"github.com/u-ai/backend/internal/desktoppet/behavior"
	"github.com/u-ai/backend/internal/desktoppet/behavior/adapters"
	"github.com/u-ai/backend/internal/realtime"
)

type desktopPetLifecycleBridge struct{ adapters *adapters.AdapterManager }

func newDesktopPetLifecycleBridge(am *adapters.AdapterManager) *desktopPetLifecycleBridge {
	return &desktopPetLifecycleBridge{adapters: am}
}

func (b *desktopPetLifecycleBridge) OnDesktopPetChatLifecycle(ctx context.Context, evt chat.DesktopPetChatLifecycle) {
	if b == nil || b.adapters == nil {
		return
	}
	b.adapters.OnChatLifecycle(ctx, behavior.ChatLifecycleEvent{
		InteractionID: evt.InteractionID, MessageID: evt.MessageID, CharacterID: evt.CharacterID,
		SpaceID: evt.SpaceID, ConversationID: evt.ConversationID, Phase: evt.Phase,
		StatusVersion: evt.StatusVersion, Origin: evt.Origin, CorrelationID: evt.CorrelationID, OccurredAt: evt.OccurredAt,
	})
	if evt.Origin == "proactive" {
		phase := ""
		switch evt.Phase {
		case "response.started":
			phase = "started"
		case "response.completed":
			phase = "completed"
		case "response.failed", "response.cancelled":
			phase = "suppressed"
		}
		if phase != "" {
			b.adapters.OnProactiveEvent(ctx, adapters.ProactiveEvent{
				ProactiveID: evt.InteractionID, CharacterID: evt.CharacterID, SpaceID: evt.SpaceID,
				ConversationID: evt.ConversationID, CorrelationID: evt.CorrelationID, Intent: "greeting",
				Phase: phase, InteractionID: evt.InteractionID, OccurredAt: evt.OccurredAt,
			})
		}
	}
}

func (b *desktopPetLifecycleBridge) OnDesktopPetToolLifecycle(ctx context.Context, evt chat.DesktopPetToolLifecycle) {
	if b == nil || b.adapters == nil {
		return
	}
	b.adapters.OnToolLifecycle(ctx, behavior.ToolLifecycleEvent{
		InteractionID: evt.InteractionID, CharacterID: evt.CharacterID, SpaceID: evt.SpaceID,
		OperationID: evt.OperationID, ToolCallID: evt.ToolCallID, ToolName: evt.ToolName,
		ToolCategory: evt.ToolCategory, DisplayClass: evt.DisplayClass, Phase: evt.Phase,
		Depth: evt.Depth, ErrorClass: evt.ErrorClass, OccurredAt: evt.OccurredAt,
	})
}

func (b *desktopPetLifecycleBridge) OnVoiceLifecycle(ctx context.Context, evt realtime.DesktopPetVoiceLifecycle) {
	if b == nil || b.adapters == nil {
		return
	}
	b.adapters.OnVoiceLifecycle(ctx, behavior.VoiceLifecycleEvent{
		SessionID: evt.SessionID, TurnID: evt.TurnID, CharacterID: evt.CharacterID,
		SpaceID: evt.SpaceID, ConversationID: evt.ConversationID, Phase: evt.Phase,
		StateVersion: evt.StateVersion, OccurredAt: evt.OccurredAt,
	})
}
