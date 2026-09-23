package conversationstream

import (
	"context"
	"fmt"
	"os"
	"sync"
	"testing"
)

func TestFileStorePersistsAndRecoversConversationEvents(t *testing.T) {
	root := t.TempDir()
	store, err := NewFileStore(root)
	if err != nil {
		t.Fatal(err)
	}
	events := []AgentUIEvent{
		{ConversationID: "conv-a", EventSequence: 1, Type: "turn.started", Status: "running"},
		{ConversationID: "conv-a", EventSequence: 2, Type: "text.delta", Status: "running", Payload: map[string]any{"delta": "hello"}},
		{ConversationID: "conv-a", EventSequence: 3, Type: "turn.completed", Status: "completed"},
		{ConversationID: "conv-b", EventSequence: 1, Type: "turn.started", Status: "running"},
	}
	for _, event := range events {
		if err := store.Persist(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	snapshot, err := store.Snapshot(context.Background(), "conv-a")
	if err != nil {
		t.Fatal(err)
	}
	if snapshot == nil || snapshot.LastEventSequence != 3 || snapshot.EventCount != 3 {
		t.Fatalf("unexpected snapshot: %#v", snapshot)
	}

	reopened, err := NewFileStore(root)
	if err != nil {
		t.Fatal(err)
	}
	latest, err := reopened.LatestSequence(context.Background(), "conv-a")
	if err != nil {
		t.Fatal(err)
	}
	if latest != 3 {
		t.Fatalf("latest sequence = %d, want 3", latest)
	}
	got, err := reopened.ListAfter(context.Background(), "conv-a", 1, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].EventSequence != 2 || got[1].EventSequence != 3 {
		t.Fatalf("unexpected recovery events: %#v", got)
	}
	if got[0].Payload["delta"] != "hello" {
		t.Fatalf("payload not recovered: %#v", got[0].Payload)
	}
}

func TestFileStoreAllowsLegacySequenceStartAndRejectsRegression(t *testing.T) {
	store, err := NewFileStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Persist(context.Background(), AgentUIEvent{ConversationID: "conv-a", EventSequence: 167, Type: "turn.started"}); err != nil {
		t.Fatal(err)
	}
	if err := store.Persist(context.Background(), AgentUIEvent{ConversationID: "conv-a", EventSequence: 168, Type: "turn.completed"}); err != nil {
		t.Fatal(err)
	}
	if err := store.Persist(context.Background(), AgentUIEvent{ConversationID: "conv-a", EventSequence: 167, Type: "turn.completed"}); err == nil {
		t.Fatal("expected sequence regression error")
	}
}

func TestFileStoreSupportsConcurrentConversations(t *testing.T) {
	store, err := NewFileStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	const conversations = 16
	const eventsPerConversation = 20
	var wait sync.WaitGroup
	errs := make(chan error, conversations)
	for conversationIndex := 0; conversationIndex < conversations; conversationIndex++ {
		conversationIndex := conversationIndex
		wait.Add(1)
		go func() {
			defer wait.Done()
			conversationID := fmt.Sprintf("conv-%d", conversationIndex)
			for eventIndex := 1; eventIndex <= eventsPerConversation; eventIndex++ {
				err := store.Persist(context.Background(), AgentUIEvent{
					ConversationID: conversationID,
					EventSequence:  int64(eventIndex),
					Type:           "text.delta",
					Status:         "running",
					Payload:        map[string]any{"delta": fmt.Sprintf("%d", eventIndex)},
				})
				if err != nil {
					errs <- err
					return
				}
			}
		}()
	}
	wait.Wait()
	close(errs)
	for err := range errs {
		t.Fatal(err)
	}
	for conversationIndex := 0; conversationIndex < conversations; conversationIndex++ {
		conversationID := fmt.Sprintf("conv-%d", conversationIndex)
		latest, err := store.LatestSequence(context.Background(), conversationID)
		if err != nil {
			t.Fatal(err)
		}
		if latest != eventsPerConversation {
			t.Fatalf("%s latest sequence = %d, want %d", conversationID, latest, eventsPerConversation)
		}
	}
}

func TestFileStoreCompactsTerminalEventLog(t *testing.T) {
	store, err := NewFileStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	store.maxEvents = 5
	store.keepEvents = 3
	for sequence := int64(1); sequence <= 12; sequence++ {
		if err := store.Persist(context.Background(), AgentUIEvent{
			ConversationID: "conv-compact",
			EventSequence:  sequence,
			Type:           "turn.completed",
			Status:         "completed",
		}); err != nil {
			t.Fatal(err)
		}
	}
	latest, err := store.LatestSequence(context.Background(), "conv-compact")
	if err != nil {
		t.Fatal(err)
	}
	if latest != 12 {
		t.Fatalf("latest sequence = %d, want 12", latest)
	}
	events, err := store.ListAfter(context.Background(), "conv-compact", 0, 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 3 || events[0].EventSequence != 10 || events[2].EventSequence != 12 {
		t.Fatalf("unexpected compacted events: %#v", events)
	}
	reopened, err := NewFileStore(store.root)
	if err != nil {
		t.Fatal(err)
	}
	events, err = reopened.ListAfter(context.Background(), "conv-compact", 0, 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 3 || events[0].EventSequence != 10 {
		t.Fatalf("compaction did not survive reopen: %#v", events)
	}
}

func TestFileStoreRecoversFromTruncatedLastEvent(t *testing.T) {
	store, err := NewFileStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for sequence := int64(1); sequence <= 2; sequence++ {
		if err := store.Persist(context.Background(), AgentUIEvent{
			ConversationID: "conv-truncated",
			EventSequence:  sequence,
			Type:           "text.delta",
			Status:         "running",
			Payload:        map[string]any{"delta": fmt.Sprint(sequence)},
		}); err != nil {
			t.Fatal(err)
		}
	}
	path := store.path("conv-truncated")
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.WriteString(`{"eventSequence":3,"conversationId":"conv-truncated"`); err != nil {
		_ = file.Close()
		t.Fatal(err)
	}
	_ = file.Close()

	reopened, err := NewFileStore(store.root)
	if err != nil {
		t.Fatal(err)
	}
	latest, err := reopened.LatestSequence(context.Background(), "conv-truncated")
	if err != nil {
		t.Fatal(err)
	}
	if latest != 2 {
		t.Fatalf("latest sequence = %d, want 2", latest)
	}
	if err := reopened.Persist(context.Background(), AgentUIEvent{
		ConversationID: "conv-truncated",
		EventSequence:  3,
		Type:           "turn.completed",
		Status:         "completed",
	}); err != nil {
		t.Fatal(err)
	}
}
