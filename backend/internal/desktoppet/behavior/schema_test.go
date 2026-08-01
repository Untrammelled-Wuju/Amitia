package behavior

import (
	"encoding/json"
	"testing"
	"time"
)

func TestSchemaRegistry_NoDuplicateRegistration(t *testing.T) {
	testDef := &EventSchemaDef{
		EventType:   "test.duplicate.event",
		Reliability: ReliabilityEphemeral,
		DefaultTTL:  5 * time.Second,
		AllowedFields: map[string]string{
			"testField": "string",
		},
		SchemaVersion: 1,
	}
	err := RegisterSchema(testDef)
	if err != nil {
		t.Fatalf("first registration should succeed, got: %v", err)
	}
	err = RegisterSchema(testDef)
	if err == nil {
		t.Fatal("duplicate registration should fail")
	}
}

func TestSchemaRegistry_GetSchema(t *testing.T) {
	def, ok := GetSchema("chat.message.received")
	if !ok {
		t.Fatal("chat.message.received should be registered")
	}
	if def.EventType != "chat.message.received" {
		t.Fatalf("expected chat.message.received, got %s", def.EventType)
	}
	if def.SchemaVersion == 0 {
		t.Fatal("schema version should not be 0")
	}
}

func TestSchemaRegistry_ComputeSchemaHash(t *testing.T) {
	hash := ComputeSchemaHash("chat.message.received")
	if hash == "" {
		t.Fatal("hash should not be empty for registered schema")
	}
	hash2 := ComputeSchemaHash("chat.message.received")
	if hash != hash2 {
		t.Fatalf("hash should be deterministic: %s != %s", hash, hash2)
	}
}

func TestSchemaRegistry_ComputeRegistryHash(t *testing.T) {
	hash := ComputeRegistryHash()
	if hash == "" {
		t.Fatal("registry hash should not be empty")
	}
}

func TestSchemaRegistry_ValidatePayload(t *testing.T) {
	validPayload := json.RawMessage(`{"messageId":"msg-123","channel":"direct","extra":"ignored"}`)
	filtered, err := ValidatePayload("chat.message.received", validPayload)
	if err != nil {
		t.Fatalf("valid payload should pass: %v", err)
	}
	var result map[string]interface{}
	if err := json.Unmarshal(filtered, &result); err != nil {
		t.Fatalf("filtered payload should be valid JSON: %v", err)
	}
	if _, ok := result["messageId"]; !ok {
		t.Fatal("messageId should be in filtered payload")
	}
	if _, ok := result["extra"]; ok {
		t.Fatal("extra should be filtered out")
	}
}

func TestSchemaRegistry_ValidatePayload_InvalidEventType(t *testing.T) {
	payload := json.RawMessage(`{"test":"value"}`)
	_, err := ValidatePayload("nonexistent.event.type", payload)
	if err == nil {
		t.Fatal("should fail for unregistered event type")
	}
}

func TestSchemaRegistry_ListRegisteredEventTypes(t *testing.T) {
	types := ListRegisteredEventTypes()
	if len(types) == 0 {
		t.Fatal("should have registered event types")
	}
	found := false
	for _, t := range types {
		if t == "chat.message.received" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("chat.message.received should be in registered types")
	}
}

func TestSchemaRegistry_RuntimeEventNames(t *testing.T) {
	expectedEvents := []string{
		"runtime.pointer.clicked",
		"runtime.pointer.double_clicked",
		"runtime.pointer.hovered",
		"runtime.drag.started",
		"runtime.drag.moved",
		"runtime.drag.completed",
		"runtime.playback.action_started",
		"runtime.playback.action_completed",
		"runtime.playback.action_interrupted",
		"runtime.playback.action_failed",
	}
	for _, eventType := range expectedEvents {
		if !IsSchemaRegistered(eventType) {
			t.Fatalf("%s should be registered", eventType)
		}
	}
}

func TestSchemaRegistry_IsSchemaRegistered(t *testing.T) {
	if !IsSchemaRegistered("chat.message.received") {
		t.Fatal("chat.message.received should be registered")
	}
	if IsSchemaRegistered("nonexistent.event.unknown") {
		t.Fatal("unknown event should not be registered")
	}
}

func TestSchemaRegistry_GetReliability(t *testing.T) {
	tests := []struct {
		eventType string
		expected  EventReliability
	}{
		{"chat.message.received", ReliabilityRecoverable},
		{"chat.response.completed", ReliabilityDurable},
		{"voice.listening.activity", ReliabilityEphemeral},
	}
	for _, tc := range tests {
		got := GetReliability(tc.eventType)
		if got != tc.expected {
			t.Fatalf("expected %s reliability for %s, got %s", tc.expected, tc.eventType, got)
		}
	}
}
