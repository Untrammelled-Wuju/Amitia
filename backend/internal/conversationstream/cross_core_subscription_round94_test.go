package conversationstream

import (
	"context"
	"testing"
)

func TestRound94CrossCoreSubscriptionRejectsStaleLocalCursor(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	firstStore, err := NewFileStore(root)
	if err != nil {
		t.Fatal(err)
	}
	secondStore, err := NewFileStore(root)
	if err != nil {
		t.Fatal(err)
	}
	first, second := NewManager(), NewManager()
	first.SetDurableStore(firstStore)
	second.SetDurableStore(secondStore)
	one, err := first.Publish(ctx, AgentUIEvent{ConversationID: "shared", Type: "turn.started", TurnID: "one"}, true)
	if err != nil {
		t.Fatal(err)
	}
	two, err := second.Publish(ctx, AgentUIEvent{ConversationID: "shared", Type: "turn.completed", TurnID: "one"}, true)
	if err != nil {
		t.Fatal(err)
	}
	if one.EventSequence != 1 || two.EventSequence != 2 {
		t.Fatalf("unexpected sequences %d,%d", one.EventSequence, two.EventSequence)
	}
	snapshot, err := first.RuntimeSnapshotCurrent(ctx, "shared")
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.LastEventSequence != 2 {
		t.Fatalf("stale snapshot cursor %d, want 2", snapshot.LastEventSequence)
	}
	if snapshot.ActiveTurn != nil {
		t.Fatalf("stale active turn must not be asserted: %+v", snapshot.ActiveTurn)
	}
	_, replay, covered, cancel, err := first.SubscribeCurrent(ctx, "shared", "subscriber", 1)
	if err != nil {
		t.Fatal(err)
	}
	defer cancel()
	if covered {
		t.Fatalf("local core must not acknowledge missing remote event: %+v", replay)
	}
}

func TestRound94CrossCoreReplayMustNotSkipInteriorSequence(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	storeA, err := NewFileStore(root)
	if err != nil {
		t.Fatal(err)
	}
	storeB, err := NewFileStore(root)
	if err != nil {
		t.Fatal(err)
	}
	first, second := NewManager(), NewManager()
	first.SetDurableStore(storeA)
	second.SetDurableStore(storeB)
	for i, manager := range []*Manager{first, second, first} {
		event, err := manager.Publish(ctx, AgentUIEvent{ConversationID: "shared", Type: "text.delta"}, true)
		if err != nil || event.EventSequence != int64(i+1) {
			t.Fatalf("publish %d: %+v %v", i, event, err)
		}
	}
	_, replay, covered, cancel, err := first.SubscribeCurrent(ctx, "shared", "subscriber", 1)
	if err != nil {
		t.Fatal(err)
	}
	defer cancel()
	if covered {
		t.Fatalf("replay with sequence 2 missing must not be marked covered: %+v", replay)
	}
}

func TestRound94CrossCoreWritesAreVisibleToExistingSubscriberHeartbeat(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	storeA, err := NewFileStore(root)
	if err != nil {
		t.Fatal(err)
	}
	storeB, err := NewFileStore(root)
	if err != nil {
		t.Fatal(err)
	}
	local, remote := NewManager(), NewManager()
	local.SetDurableStore(storeA)
	remote.SetDurableStore(storeB)
	if _, err := local.Publish(ctx, AgentUIEvent{ConversationID: "shared", Type: "text.delta"}, true); err != nil {
		t.Fatal(err)
	}
	_, _, covered, unsubscribe, err := local.SubscribeCurrent(ctx, "shared", "live", 1)
	if err != nil {
		t.Fatal(err)
	}
	defer unsubscribe()
	if !covered {
		t.Fatal("initial local subscriber must be caught up")
	}
	if _, err := remote.Publish(ctx, AgentUIEvent{ConversationID: "shared", Type: "text.delta"}, true); err != nil {
		t.Fatal(err)
	}
	latest, err := local.LatestSequence(ctx, "shared")
	if err != nil {
		t.Fatal(err)
	}
	if latest != 2 {
		t.Fatalf("heartbeat must detect remote durable sequence 2, got %d", latest)
	}
}
