package conversationstream

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestRound92DistinctEventsCannotSharePersistedSequence(t *testing.T) {
	ctx := context.Background()
	store, err := NewFileStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	first := NewManager()
	second := NewManager()
	first.SetDurableStore(store)
	second.SetDurableStore(store)
	if _, err := first.RuntimeSnapshotCurrent(ctx, "shared"); err != nil {
		t.Fatal(err)
	}
	if _, err := second.RuntimeSnapshotCurrent(ctx, "shared"); err != nil {
		t.Fatal(err)
	}
	one, err := first.Publish(ctx, AgentUIEvent{ConversationID: "shared", Type: "turn.started", TurnID: "one"}, true)
	if err != nil {
		t.Fatal(err)
	}
	two, err := second.Publish(ctx, AgentUIEvent{ConversationID: "shared", Type: "turn.started", TurnID: "two"}, true)
	if err != nil {
		t.Fatal(err)
	}
	records, listErr := store.ListAfter(ctx, "shared", 0, 10)
	if listErr != nil {
		t.Fatal(listErr)
	}
	if len(records) != 2 || records[0].EventID != one.EventID || records[1].EventID != two.EventID || records[0].EventSequence == records[1].EventSequence {
		t.Fatalf("acknowledged event missing from durable log: first=%+v second=%+v records=%+v", one, two, records)
	}
}

func TestRound92PostCommitSnapshotMaintenanceCannotLoseAcknowledgment(t *testing.T) {
	ctx := context.Background()
	store, err := NewFileStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	manager := NewManager()
	manager.SetDurableStore(store)
	if _, err := manager.Publish(ctx, AgentUIEvent{ConversationID: "post-commit", TurnID: "start", Type: "turn.started"}, true); err != nil {
		t.Fatal(err)
	}
	snapshotPath := store.snapshotPath("post-commit")
	if err := os.MkdirAll(snapshotPath, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(snapshotPath, "block"), []byte("block"), 0o600); err != nil {
		t.Fatal(err)
	}
	one, err := manager.Publish(ctx, AgentUIEvent{ConversationID: "post-commit", TurnID: "one", Type: "turn.completed"}, true)
	if err != nil {
		t.Fatalf("event committed before snapshot maintenance failure: %v", err)
	}
	if one.EventSequence != 2 {
		t.Fatalf("first sequence = %d", one.EventSequence)
	}
	two, err := manager.Publish(ctx, AgentUIEvent{ConversationID: "post-commit", TurnID: "two", Type: "turn.started"}, true)
	if err != nil {
		t.Fatal(err)
	}
	if two.EventSequence != 3 {
		t.Fatalf("second sequence = %d", two.EventSequence)
	}
	records, err := store.ListAfter(ctx, "post-commit", 0, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 3 || records[1].EventID != one.EventID || records[2].EventID != two.EventID {
		t.Fatalf("durable events differ from acknowledgments: %+v", records)
	}
}

func TestRound92CorruptSnapshotCannotResetDurableCursor(t *testing.T) {
	ctx := context.Background()
	store, err := NewFileStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(store.snapshotPath("damaged")), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(store.snapshotPath("damaged"), []byte("not-json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := store.LatestSequence(ctx, "damaged"); err == nil {
		t.Fatal("corrupted persisted snapshot must fail closed")
	}
}

func TestRound92FileStoreDetectsDistinctEventsAtSameSequence(t *testing.T) {
	ctx := context.Background()
	store, err := NewFileStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	original := AgentUIEvent{ConversationID: "collision", EventID: "a", EventSequence: 1, Type: "turn.started"}
	if err := store.Persist(ctx, original); err != nil {
		t.Fatal(err)
	}
	if err := store.Persist(ctx, original); err != nil {
		t.Fatalf("same event idempotency: %v", err)
	}
	changed := original
	changed.EventID = "b"
	if err := store.Persist(ctx, changed); !errors.Is(err, ErrSequenceConflict) {
		t.Fatalf("expected durable event conflict: %v", err)
	}
	records, err := store.ListAfter(ctx, "collision", 0, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 1 || records[0].EventID != "a" {
		t.Fatalf("collision changed committed data: %+v", records)
	}
}

func TestRound92ConcurrentManagersCannotAcknowledgePhantomEvents(t *testing.T) {
	ctx := context.Background()
	store, err := NewFileStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	const count = 12
	managers := make([]*Manager, count)
	for i := range managers {
		managers[i] = NewManager()
		managers[i].SetDurableStore(store)
		if _, err := managers[i].RuntimeSnapshotCurrent(ctx, "many"); err != nil {
			t.Fatal(err)
		}
	}
	type response struct {
		event AgentUIEvent
		err   error
	}
	results := make(chan response, count)
	for i := range managers {
		go func(i int) {
			event, err := managers[i].Publish(ctx, AgentUIEvent{ConversationID: "many", TurnID: string(rune('a' + i)), Type: "turn.started"}, true)
			results <- response{event, err}
		}(i)
	}
	acknowledged := map[string]int64{}
	for i := 0; i < count; i++ {
		r := <-results
		if r.err == nil {
			acknowledged[r.event.EventID] = r.event.EventSequence
		}
	}
	records, err := store.ListAfter(ctx, "many", 0, count+1)
	if err != nil {
		t.Fatal(err)
	}
	found := map[string]int64{}
	for _, record := range records {
		found[record.EventID] = record.EventSequence
	}
	for eventID, seq := range acknowledged {
		if found[eventID] != seq {
			t.Fatalf("phantom acknowledged event %s sequence %d: %+v", eventID, seq, records)
		}
	}
	if len(acknowledged) == 0 {
		t.Fatal("no events accepted")
	}
}

func TestRound92SnapshotPreventsReuseAfterEventLogLoss(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	original, err := NewFileStore(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := original.Persist(ctx, AgentUIEvent{ConversationID: "recovered", EventID: "first", EventSequence: 7, Type: "turn.completed"}); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(original.path("recovered")); err != nil {
		t.Fatal(err)
	}
	reopened, err := NewFileStore(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := reopened.Persist(ctx, AgentUIEvent{ConversationID: "recovered", EventID: "second", EventSequence: 7, Type: "turn.started"}); !errors.Is(err, ErrSequenceConflict) {
		t.Fatalf("must not reuse persisted sequence after journal loss: %v", err)
	}
	latest, err := reopened.LatestSequence(ctx, "recovered")
	if err != nil || latest != 7 {
		t.Fatalf("durable snapshot cursor lost: %d %v", latest, err)
	}
}
