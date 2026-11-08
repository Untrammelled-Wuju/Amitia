package event

import (
	"context"
	"testing"
)

func TestDefaultHostEventTypesIncludeGameHostNotification(t *testing.T) {
	registry := NewEventSchemaRegistry()
	for _, definition := range DefaultHostEventTypes() {
		if err := registry.RegisterEventType(context.Background(), definition); err != nil {
			t.Fatalf("register %s v%d: %v", definition.EventTypeID, definition.Version, err)
		}
	}
	definition, err := registry.GetEventType(context.Background(), "gamehost.notification", 1)
	if err != nil {
		t.Fatalf("get gamehost.notification v1: %v", err)
	}
	if !definition.ProducerPolicy.RequireSystemTrust {
		t.Fatal("gamehost.notification must require system trust")
	}
}
