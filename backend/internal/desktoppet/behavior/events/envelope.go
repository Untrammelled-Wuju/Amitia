package events

import (
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"time"

	"github.com/u-ai/backend/internal/desktoppet/behavior"
)

type EnvelopeBuilder struct {
	env     behavior.BehaviorEventEnvelope
	payload map[string]interface{}
}

func NewEnvelope(eventType string, origin behavior.EventOrigin) *EnvelopeBuilder {
	return &EnvelopeBuilder{
		env: behavior.BehaviorEventEnvelope{
			EventID:       behavior.UUIDNew(),
			EventType:     eventType,
			SchemaVersion: 1,
			Origin:        origin,
		},
		payload: make(map[string]interface{}),
	}
}

func (b *EnvelopeBuilder) EventID(id string) *EnvelopeBuilder {
	b.env.EventID = id
	return b
}

func (b *EnvelopeBuilder) UserID(id string) *EnvelopeBuilder {
	b.env.UserID = id
	return b
}

func (b *EnvelopeBuilder) CharacterID(id string) *EnvelopeBuilder {
	b.env.CharacterID = id
	return b
}

func (b *EnvelopeBuilder) ConversationID(id string) *EnvelopeBuilder {
	b.env.ConversationID = id
	return b
}

func (b *EnvelopeBuilder) InteractionID(id string) *EnvelopeBuilder {
	b.env.InteractionID = id
	return b
}

func (b *EnvelopeBuilder) SessionID(id string) *EnvelopeBuilder {
	b.env.SessionID = id
	return b
}

func (b *EnvelopeBuilder) InstallationID(id string) *EnvelopeBuilder {
	b.env.InstallationID = id
	return b
}

func (b *EnvelopeBuilder) PetInstanceID(id string) *EnvelopeBuilder {
	b.env.PetInstanceID = id
	return b
}

func (b *EnvelopeBuilder) CorrelationID(id string) *EnvelopeBuilder {
	b.env.CorrelationID = id
	return b
}

func (b *EnvelopeBuilder) CausationID(id string) *EnvelopeBuilder {
	b.env.CausationID = id
	return b
}

func (b *EnvelopeBuilder) Sequence(seq int64) *EnvelopeBuilder {
	b.env.Sequence = seq
	return b
}

func (b *EnvelopeBuilder) DedupKey(key string) *EnvelopeBuilder {
	b.env.DedupKey = key
	return b
}

func (b *EnvelopeBuilder) PriorityHint(hint int) *EnvelopeBuilder {
	b.env.PriorityHint = hint
	return b
}

func (b *EnvelopeBuilder) OccurredAt(t time.Time) *EnvelopeBuilder {
	b.env.OccurredAt = t
	return b
}

func (b *EnvelopeBuilder) ExpiresAt(t *time.Time) *EnvelopeBuilder {
	b.env.ExpiresAt = t
	return b
}

func (b *EnvelopeBuilder) PayloadField(key string, value interface{}) *EnvelopeBuilder {
	b.payload[key] = value
	return b
}

func (b *EnvelopeBuilder) PayloadRaw(raw json.RawMessage) *EnvelopeBuilder {
	b.env.Payload = raw
	return b
}

func (b *EnvelopeBuilder) Build(now time.Time) behavior.BehaviorEventEnvelope {
	if b.env.OccurredAt.IsZero() {
		b.env.OccurredAt = now
	}
	b.env.ReceivedAt = now
	if b.env.DedupKey == "" {
		b.env.DedupKey = b.env.EventID
	}
	if len(b.payload) > 0 && len(b.env.Payload) == 0 {
		raw, _ := json.Marshal(b.payload)
		b.env.Payload = raw
	}
	if def, ok := behavior.GetSchema(b.env.EventType); ok {
		b.env.SchemaVersion = def.SchemaVersion
		if len(b.env.Payload) > 0 {
			if filtered, err := behavior.ValidatePayload(b.env.EventType, b.env.Payload); err == nil {
				b.env.Payload = filtered
			}
		}
	}
	return b.env
}

func BuildDedupKey(parts ...string) string {
	result := ""
	for i, p := range parts {
		if p == "" {
			continue
		}
		if i > 0 && result != "" {
			result += "+"
		}
		result += p
	}
	if result == "" {
		return "auto-" + uuid.NewString()
	}
	return result
}

func EnvelopeFromInteractionEvent(evt behavior.InteractionLifecycleEvent, now time.Time, origin behavior.EventOrigin) behavior.BehaviorEventEnvelope {
	builder := NewEnvelope("interaction."+evt.Phase, origin).
		UserID(evt.UserID).
		CharacterID(evt.CharacterID).
		InteractionID(evt.InteractionID).
		OccurredAt(evt.OccurredAt).
		DedupKey(BuildDedupKey(evt.InteractionID, evt.Phase, fmt.Sprintf("v%d", evt.StatusVersion)))

	if evt.ConversationID != "" {
		builder.ConversationID(evt.ConversationID)
	}
	if evt.CorrelationID != "" {
		builder.CorrelationID(evt.CorrelationID)
	}
	if evt.Origin != "" {
		builder.PayloadField("origin", evt.Origin)
	}
	builder.PayloadField("statusVersion", evt.StatusVersion)
	return builder.Build(now)
}

func EnvelopeFromChatEvent(evt behavior.ChatLifecycleEvent, now time.Time) behavior.BehaviorEventEnvelope {
	eventType := "chat." + evt.Phase
	builder := NewEnvelope(eventType, behavior.OriginChat).
		UserID(evt.UserID).
		CharacterID(evt.CharacterID).
		InteractionID(evt.InteractionID).
		OccurredAt(evt.OccurredAt).
		DedupKey(BuildDedupKey(evt.InteractionID, evt.MessageID, evt.Phase, fmt.Sprintf("v%d", evt.StatusVersion)))

	if evt.ConversationID != "" {
		builder.ConversationID(evt.ConversationID)
	}
	if evt.CorrelationID != "" {
		builder.CorrelationID(evt.CorrelationID)
	}
	if evt.MessageID != "" {
		builder.PayloadField("messageId", evt.MessageID)
	}
	if evt.Origin != "" {
		builder.PayloadField("origin", evt.Origin)
	}
	if evt.StatusVersion > 0 {
		builder.PayloadField("statusVersion", evt.StatusVersion)
		builder.PayloadField("interactionStatusVersion", evt.StatusVersion)
	}
	return builder.Build(now)
}

func EnvelopeFromToolEvent(evt behavior.ToolLifecycleEvent, now time.Time) behavior.BehaviorEventEnvelope {
	eventType := "agent.tool." + evt.Phase
	builder := NewEnvelope(eventType, behavior.OriginTool).
		UserID(evt.UserID).
		CharacterID(evt.CharacterID).
		InteractionID(evt.InteractionID).
		OccurredAt(evt.OccurredAt).
		DedupKey(BuildDedupKey(evt.InteractionID, evt.OperationID, evt.Phase))

	builder.PayloadField("toolOperationId", evt.OperationID)
	if evt.ToolCategory != "" {
		builder.PayloadField("toolCategory", evt.ToolCategory)
	}
	if evt.DisplayClass != "" {
		builder.PayloadField("displayClass", evt.DisplayClass)
	}
	builder.PayloadField("depth", evt.Depth)
	builder.PayloadField("expectedLongRunning", evt.ExpectedLongRunning)
	if evt.ErrorClass != "" {
		builder.PayloadField("errorClass", evt.ErrorClass)
	}
	return builder.Build(now)
}

func EnvelopeFromVoiceEvent(evt behavior.VoiceLifecycleEvent, now time.Time) behavior.BehaviorEventEnvelope {
	eventType := "voice." + evt.Phase
	builder := NewEnvelope(eventType, behavior.OriginVoice).
		UserID(evt.UserID).
		CharacterID(evt.CharacterID).
		SessionID(evt.SessionID).
		OccurredAt(evt.OccurredAt).
		DedupKey(BuildDedupKey(evt.SessionID, evt.TurnID, evt.Phase, fmt.Sprintf("v%d", evt.StateVersion)))

	if evt.TurnID != "" {
		builder.PayloadField("turnId", evt.TurnID)
	}
	if evt.ConversationID != "" {
		builder.ConversationID(evt.ConversationID)
	}
	builder.PayloadField("stateVersion", evt.StateVersion)
	return builder.Build(now)
}

func canonicalDesktopGestureEventType(gestureType string) string {
	switch gestureType {
	case "clicked":
		return "runtime.pointer.clicked"
	case "double_clicked":
		return "runtime.pointer.double_clicked"
	case "hovered":
		return "runtime.pointer.hovered"
	case "drag.started":
		return "runtime.drag.started"
	case "drag.moved":
		return "runtime.drag.moved"
	case "drag.ended", "drag.completed":
		return "runtime.drag.completed"
	case "drag.cancelled":
		return "runtime.drag.cancelled"
	case "fall.started":
		return "runtime.pet.fall.started"
	case "edge.reached":
		return "runtime.pet.edge.reached"
	case "interacted":
		return "runtime.pet.interacted"
	default:
		return "runtime.pet." + gestureType
	}
}

func EnvelopeFromDesktopEvent(evt behavior.DesktopGestureEvent, now time.Time) behavior.BehaviorEventEnvelope {
	eventType := canonicalDesktopGestureEventType(evt.GestureType)
	builder := NewEnvelope(eventType, behavior.OriginDesktop).
		UserID(evt.UserID).
		CharacterID(evt.CharacterID).
		PetInstanceID(evt.PetInstanceID).
		Sequence(evt.Sequence).
		OccurredAt(evt.OccurredAt).
		DedupKey(BuildDedupKey(evt.GestureID, evt.GestureType, fmt.Sprintf("s%d", evt.Sequence)))

	builder.PayloadField("gestureId", evt.GestureID)
	builder.PayloadField("sequence", evt.Sequence)
	if eventType == "runtime.pet.interacted" {
		builder.PayloadField("interactionType", evt.GestureType)
	}
	return builder.Build(now)
}
