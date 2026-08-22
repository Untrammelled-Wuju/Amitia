package event

import (
	"context"
	"testing"
)

func TestDefaultHostEventTypesIncludeConversationUIEvent(t *testing.T) {
	registry := NewEventSchemaRegistry()
	for _, def := range DefaultHostEventTypes() {
		if err := registry.RegisterEventType(context.Background(), def); err != nil {
			t.Fatalf("register %s v%d: %v", def.EventTypeID, def.Version, err)
		}
	}
	def, err := registry.GetEventType(context.Background(), "conversation.ui_event", 1)
	if err != nil {
		t.Fatalf("conversation.ui_event must be registered: %v", err)
	}
	if def.OrderingPolicy != OrderingPerPartition {
		t.Fatalf("ordering policy = %q, want %q", def.OrderingPolicy, OrderingPerPartition)
	}
}
