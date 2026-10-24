package conversationstream

import (
	"context"
	"errors"
	"testing"
)

func TestRound91RestartRestoresDurableSnapshotAndCursor(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	store, err := NewFileStore(root)
	if err != nil {
		t.Fatal(err)
	}
	first := NewManager()
	first.SetDurableStore(store)
	for i := 1; i <= 3; i++ {
		event, err := first.Publish(ctx, AgentUIEvent{ConversationID: "resume", Type: "text.delta"}, true)
		if err != nil || event.EventSequence != int64(i) {
			t.Fatalf("publish %d: %+v %v", i, event, err)
		}
	}
	reopened, err := NewFileStore(root)
	if err != nil {
		t.Fatal(err)
	}
	second := NewManager()
	second.SetDurableStore(reopened)
	snapshot, err := second.RuntimeSnapshotCurrent(ctx, "resume")
	if err != nil || snapshot.LastEventSequence != 3 {
		t.Fatalf("restarted snapshot: %+v %v", snapshot, err)
	}
	_, replay, covered, cancel, err := second.SubscribeCurrent(ctx, "resume", "fresh", 3)
	if err != nil {
		t.Fatal(err)
	}
	cancel()
	if !covered || len(replay) != 0 {
		t.Fatalf("caught-up cursor: covered=%v replay=%v", covered, replay)
	}
	_, _, covered, cancel, err = second.SubscribeCurrent(ctx, "resume", "old", 0)
	if err != nil {
		t.Fatal(err)
	}
	cancel()
	if covered {
		t.Fatal("an absent in-memory replay must require snapshot")
	}
	next, err := second.Publish(ctx, AgentUIEvent{ConversationID: "resume", Type: "text.delta"}, true)
	if err != nil || next.EventSequence != 4 {
		t.Fatalf("first post-restart event: %+v %v", next, err)
	}
	_, replay, covered, cancel, err = second.SubscribeCurrent(ctx, "resume", "next", 3)
	if err != nil {
		t.Fatal(err)
	}
	cancel()
	if !covered || len(replay) != 1 || replay[0].EventSequence != 4 {
		t.Fatalf("post-restart replay: covered=%v replay=%+v", covered, replay)
	}
}

type round91UnavailableStore struct{}

func (round91UnavailableStore) Persist(context.Context, AgentUIEvent) error {
	return errors.New("unavailable")
}
func (round91UnavailableStore) ListAfter(context.Context, string, int64, int) ([]AgentUIEvent, error) {
	return nil, errors.New("unavailable")
}
func (round91UnavailableStore) LatestSequence(context.Context, string) (int64, error) {
	return 0, errors.New("unavailable")
}

func TestRound91FailedDurableReadNeverAcknowledgesReplay(t *testing.T) {
	manager := NewManager()
	manager.SetDurableStore(round91UnavailableStore{})
	if _, err := manager.RuntimeSnapshotCurrent(context.Background(), "unavailable"); err == nil {
		t.Fatal("snapshot must fail closed")
	}
	_, _, covered, cancel, err := manager.SubscribeCurrent(context.Background(), "unavailable", "subscriber", 0)
	if err == nil || covered || cancel != nil {
		t.Fatalf("subscription must fail closed: %v %v cancelSet=%t", err, covered, cancel != nil)
	}
}

func TestRound91NewConversationWithoutDurableHistory(t *testing.T) {
	manager := NewManager()
	snapshot, err := manager.RuntimeSnapshotCurrent(context.Background(), "new")
	if err != nil || snapshot.LastEventSequence != 0 {
		t.Fatalf("fresh snapshot: %+v %v", snapshot, err)
	}
	_, _, covered, cancel, err := manager.SubscribeCurrent(context.Background(), "new", "new", 0)
	if err != nil {
		t.Fatal(err)
	}
	cancel()
	if !covered {
		t.Fatal("new conversation must allow initial stream")
	}
}
