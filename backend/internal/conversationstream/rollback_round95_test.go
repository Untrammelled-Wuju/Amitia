package conversationstream

import (
	"bytes"
	"context"
	"errors"
	"os"
	"testing"
)

func TestRound95RolledBackEventLogCannotReuseAcknowledgedSequence(t *testing.T) {
	ctx := context.Background()
	store, err := NewFileStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	id := "roll-back-shared"
	first := AgentUIEvent{ConversationID: id, EventID: "one", EventSequence: 1, Type: "turn.started"}
	second := AgentUIEvent{ConversationID: id, EventID: "two", EventSequence: 2, Type: "turn.completed"}
	if err := store.Persist(ctx, first); err != nil {
		t.Fatal(err)
	}
	saved, err := os.ReadFile(store.path(id))
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Persist(ctx, second); err != nil {
		t.Fatal(err)
	}
	complete, err := os.ReadFile(store.path(id))
	if err != nil {
		t.Fatal(err)
	}
	snap, err := store.readSnapshot(id)
	if err != nil || snap == nil || snap.LastEventSequence != 2 {
		t.Fatalf("invalid fixture snapshot: %+v %v", snap, err)
	}
	if err := os.WriteFile(store.path(id), saved, 0o600); err != nil {
		t.Fatal(err)
	}
	reopened, err := NewFileStore(store.root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := reopened.LatestSequence(ctx, id); !errors.Is(err, ErrDurableSequenceRollback) {
		t.Fatalf("rolled back log returned valid sequence: %v", err)
	}
	if _, err := reopened.ListAfter(ctx, id, 0, 100); !errors.Is(err, ErrDurableSequenceRollback) {
		t.Fatalf("rolled back log replayed incomplete history: %v", err)
	}
	if _, err := reopened.Snapshot(ctx, id); !errors.Is(err, ErrDurableSequenceRollback) {
		t.Fatalf("rolled back log overwrote valid snapshot: %v", err)
	}
	third := AgentUIEvent{ConversationID: id, EventID: "three", EventSequence: 3, Type: "turn.started"}
	if err := reopened.Persist(ctx, third); !errors.Is(err, ErrDurableSequenceRollback) {
		t.Fatalf("rolled back log accepted another event: %v", err)
	}
	after, err := os.ReadFile(store.path(id))
	if err != nil || !bytes.Equal(saved, after) {
		t.Fatalf("rollback rejection mutated event log: %v", err)
	}
	snap, err = reopened.readSnapshot(id)
	if err != nil || snap.LastEventSequence != 2 {
		t.Fatalf("rollback modified checkpoint: %+v %v", snap, err)
	}
	if err := os.WriteFile(store.path(id), complete, 0o600); err != nil {
		t.Fatal(err)
	}
	latest, err := reopened.LatestSequence(ctx, id)
	if err != nil || latest != 2 {
		t.Fatalf("restored history could not be recovered: sequence=%d err=%v", latest, err)
	}
	if err := reopened.Persist(ctx, third); err != nil {
		t.Fatalf("recovered event store rejected the next authorized append: %v", err)
	}
}

func TestRound95ForeignSnapshotCannotAuthorizeStream(t *testing.T) {
	ctx := context.Background()
	store, err := NewFileStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	id := "foreign-snapshot"
	if err := store.Persist(ctx, AgentUIEvent{ConversationID: id, EventSequence: 1, Type: "turn.completed"}); err != nil {
		t.Fatal(err)
	}
	path := store.snapshotPath(id)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	changed := bytes.Replace(data, []byte(id), []byte("another-stream"), 1)
	if bytes.Equal(changed, data) {
		t.Fatal("fixture did not change the snapshot identity")
	}
	if err := os.WriteFile(path, changed, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := store.LatestSequence(ctx, id); !errors.Is(err, ErrDurableSequenceRollback) {
		t.Fatal("foreign snapshot identity was trusted")
	}
}
