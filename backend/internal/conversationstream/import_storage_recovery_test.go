package conversationstream

import (
	"context"
	"errors"
	"os"
	"testing"
)

func TestFileStoreDeletedLogCannotReplayCachedEvents(t *testing.T) {
	store, err := NewFileStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	event := AgentUIEvent{ConversationID: "lost-log", EventID: "durable", EventSequence: 7, Type: "turn.completed"}
	if err := store.Persist(ctx, event); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(store.path(event.ConversationID)); err != nil {
		t.Fatal(err)
	}
	events, err := store.ListAfter(ctx, event.ConversationID, 0, 10)
	if err != nil || len(events) != 0 {
		t.Fatalf("missing log replayed cached evidence: %+v %v", events, err)
	}
	latest, err := store.LatestSequence(ctx, event.ConversationID)
	if err != nil || latest != 7 {
		t.Fatalf("snapshot cursor lost: %d %v", latest, err)
	}
	if err := store.Persist(ctx, event); !errors.Is(err, ErrSequenceConflict) {
		t.Fatalf("lost-log sequence reused: %v", err)
	}
	event.EventID, event.EventSequence, event.Type = "next", 8, "turn.started"
	if err := store.Persist(ctx, event); err != nil {
		t.Fatal(err)
	}
	events, err = store.ListAfter(ctx, event.ConversationID, 0, 10)
	if err != nil || len(events) != 1 || events[0].EventID != "next" {
		t.Fatalf("old cache survived append: %+v %v", events, err)
	}
}

func TestFileStoreMissingLogRejectsForeignSnapshot(t *testing.T) {
	store, err := NewFileStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	event := AgentUIEvent{ConversationID: "expected", EventSequence: 1, Type: "turn.completed"}
	if err := store.Persist(ctx, event); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(store.path(event.ConversationID)); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(store.snapshotPath(event.ConversationID), []byte(`{"conversationId":"foreign","lastEventSequence":100}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := store.LatestSequence(ctx, event.ConversationID); !errors.Is(err, ErrDurableSequenceRollback) {
		t.Fatalf("foreign cursor accepted: %v", err)
	}
	if _, err := store.ListAfter(ctx, event.ConversationID, 0, 10); !errors.Is(err, ErrDurableSequenceRollback) {
		t.Fatalf("foreign replay accepted: %v", err)
	}
	if _, err := store.Snapshot(ctx, event.ConversationID); !errors.Is(err, ErrDurableSequenceRollback) {
		t.Fatalf("foreign snapshot accepted: %v", err)
	}
	event.EventSequence = 101
	if err := store.Persist(ctx, event); !errors.Is(err, ErrDurableSequenceRollback) {
		t.Fatalf("foreign snapshot authorized append: %v", err)
	}
}
