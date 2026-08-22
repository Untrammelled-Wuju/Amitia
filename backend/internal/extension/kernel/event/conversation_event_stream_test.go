package event

import (
	"context"
	"encoding/json"
	"testing"
)

func TestConversationScopedEventMirrorsAtomicallyWithCanonicalSequence(t *testing.T) {
	svc, _, _, _, cleanup := setupTestService(t, true, true, true, true)
	defer cleanup()
	if _, err := svc.db.Exec(`CREATE TABLE IF NOT EXISTS extension_conversation_event_sequences (
		conversation_id TEXT PRIMARY KEY,
		last_sequence INTEGER NOT NULL DEFAULT 0,
		updated_at DATETIME NOT NULL
	)`); err != nil {
		t.Fatalf("create sequence table: %v", err)
	}

	publish := func(message string) {
		t.Helper()
		payload, _ := json.Marshal(map[string]interface{}{"message": message})
		if _, err := svc.Publish(context.Background(), "system.test", 1, payload, PublishOptions{
			ProducerID:    "test-producer",
			ProducerType:  EventProducerTypeHost,
			AggregateType: "conversation",
			AggregateID:   "conv-1",
		}); err != nil {
			t.Fatalf("publish %s: %v", message, err)
		}
	}

	publish("first")
	publish("second")

	records, err := svc.ListConversationUIEventsAfterSequence(context.Background(), "conv-1", 0, 10)
	if err != nil {
		t.Fatalf("list canonical events: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("canonical event count = %d, want 2", len(records))
	}
	for index, record := range records {
		want := int64(index + 1)
		if record.AggregateVersion == nil || *record.AggregateVersion != want {
			t.Fatalf("record %d sequence = %v, want %d", index, record.AggregateVersion, want)
		}
		var payload map[string]interface{}
		if err := json.Unmarshal(record.Payload, &payload); err != nil {
			t.Fatalf("decode canonical payload: %v", err)
		}
		if payload["type"] != "system.test" {
			t.Fatalf("canonical type = %v, want system.test", payload["type"])
		}
		if payload["conversationId"] != "conv-1" {
			t.Fatalf("canonical conversationId = %v", payload["conversationId"])
		}
		if got := int64(payload["sequence"].(float64)); got != want {
			t.Fatalf("canonical payload sequence = %d, want %d", got, want)
		}
	}

	afterOne, err := svc.ListConversationUIEventsAfterSequence(context.Background(), "conv-1", 1, 10)
	if err != nil {
		t.Fatalf("list after sequence: %v", err)
	}
	if len(afterOne) != 1 || afterOne[0].AggregateVersion == nil || *afterOne[0].AggregateVersion != 2 {
		t.Fatalf("cursor query = %#v, want only sequence 2", afterOne)
	}
}

func TestConversationUIEventBeforeSequenceReturnsAscendingStablePage(t *testing.T) {
	svc, _, _, _, cleanup := setupTestService(t, true, true, true, true)
	defer cleanup()
	if _, err := svc.db.Exec(`CREATE TABLE IF NOT EXISTS extension_conversation_event_sequences (
		conversation_id TEXT PRIMARY KEY,
		last_sequence INTEGER NOT NULL DEFAULT 0,
		updated_at DATETIME NOT NULL
	)`); err != nil {
		t.Fatalf("create sequence table: %v", err)
	}

	for index := 1; index <= 5; index++ {
		payload, _ := json.Marshal(map[string]interface{}{"index": index})
		if _, _, err := svc.PublishConversationUIEvent(context.Background(), "conv-before", payload, ""); err != nil {
			t.Fatalf("publish %d: %v", index, err)
		}
	}

	latest, err := svc.ListConversationUIEventsBeforeSequence(context.Background(), "conv-before", 0, 2)
	if err != nil {
		t.Fatalf("latest page: %v", err)
	}
	if len(latest) != 2 || latest[0].AggregateVersion == nil || latest[1].AggregateVersion == nil || *latest[0].AggregateVersion != 4 || *latest[1].AggregateVersion != 5 {
		t.Fatalf("latest page sequences = %#v, want [4 5]", latest)
	}

	older, err := svc.ListConversationUIEventsBeforeSequence(context.Background(), "conv-before", 4, 2)
	if err != nil {
		t.Fatalf("older page: %v", err)
	}
	if len(older) != 2 || older[0].AggregateVersion == nil || older[1].AggregateVersion == nil || *older[0].AggregateVersion != 2 || *older[1].AggregateVersion != 3 {
		t.Fatalf("older page sequences = %#v, want [2 3]", older)
	}
}
