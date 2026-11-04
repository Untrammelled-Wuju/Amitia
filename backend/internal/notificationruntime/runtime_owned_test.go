package notificationruntime

import (
	"context"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/u-ai/backend/internal/conversationstream"
	"gorm.io/gorm"
)

func TestResolveMetadataUsesOwnedRuntimePayload(t *testing.T) {
	runtime := NewRuntime(nil, nil, false)
	event := conversationstream.AgentUIEvent{
		ConversationID: "owned/runtime-key",
		MessageID:      "request-1/assistant",
		Type:           "turn.completed",
		Payload: map[string]any{
			"spaceId":           "space-a",
			"conversationId":    "conversation-a",
			"characterId":       "role-a",
			"characterName":     "林澈",
			"conversationTitle": "周末计划",
			"content":           "明天上午出发。",
		},
	}
	meta := runtime.resolveMetadata(context.Background(), event)
	if meta.spaceID != "space-a" ||
		meta.conversationID != "conversation-a" ||
		meta.characterID != "role-a" ||
		meta.characterName != "林澈" ||
		meta.conversationTitle != "周末计划" ||
		meta.messageContent != "明天上午出发。" {
		t.Fatalf("owned event metadata was not preserved: %#v", meta)
	}
}

func TestRegisterManagerAcceptsOwnedRuntimeManager(t *testing.T) {
	runtime := NewRuntime(nil, nil, false)
	manager := conversationstream.NewManager()
	runtime.RegisterManager(manager)
	runtime.RegisterManager(manager)
	if len(runtime.managedManagers) != 1 || runtime.managedManagers[0] != manager {
		t.Fatalf("owned runtime manager registration was not idempotent: %#v", runtime.managedManagers)
	}
}
