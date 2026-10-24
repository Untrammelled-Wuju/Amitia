package conversationstream

import (
	"context"
	"testing"
)

func TestRound90SubscriptionDetectsMissingInitialHistory(t *testing.T) {
	manager := NewManager()
	manager.SetRingSize(2)
	for i := 0; i < 5; i++ {
		if _, err := manager.Publish(context.Background(), AgentUIEvent{ConversationID: "round90", Type: "text.delta"}, false); err != nil {
			t.Fatal(err)
		}
	}
	for _, tc := range []struct {
		name      string
		cursor    int64
		covered   bool
		sequences []int64
	}{
		{"initial-history-lost", 0, false, []int64{4, 5}},
		{"previous-history-lost", 2, false, []int64{4, 5}},
		{"previous-event-covered", 3, true, []int64{4, 5}},
		{"fully-caught-up", 5, true, nil},
		{"invalid-future-cursor", 6, false, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, replay, covered, cancel := manager.Subscribe("round90", "subscriber-"+tc.name, tc.cursor)
			defer cancel()
			if covered != tc.covered || len(replay) != len(tc.sequences) {
				t.Fatalf("cursor=%d covered=%v replay=%d", tc.cursor, covered, len(replay))
			}
			for i, expected := range tc.sequences {
				if replay[i].EventSequence != expected {
					t.Fatalf("replay[%d]=%d, want %d", i, replay[i].EventSequence, expected)
				}
			}
		})
	}
}

func TestRound90SubscriptionEmptySnapshotAllowsFreshStream(t *testing.T) {
	manager := NewManager()
	_, replay, covered, cancel := manager.Subscribe("fresh", "fresh-subscriber", 0)
	defer cancel()
	if !covered || len(replay) != 0 {
		t.Fatalf("empty cursor should be covered: %v %+v", covered, replay)
	}
}

func TestRound90SubscriptionOverflowClosesForSnapshotRecovery(t *testing.T) {
	manager := NewManager()
	manager.SetRingSize(64)
	live, _, covered, cancel := manager.Subscribe("overflow", "overflow-subscriber", 0)
	defer cancel()
	if !covered {
		t.Fatal("fresh subscriber should be covered")
	}
	for index := 0; index < 65; index++ {
		if _, err := manager.Publish(context.Background(), AgentUIEvent{ConversationID: "overflow", Type: "text.delta"}, false); err != nil {
			t.Fatal(err)
		}
	}
	delivered := 0
	for range live {
		delivered++
	}
	if delivered != 64 {
		t.Fatalf("delivered=%d, want full buffer before close", delivered)
	}
	latest, err := manager.LatestSequence(context.Background(), "overflow")
	if err != nil {
		t.Fatal(err)
	}
	if latest <= int64(delivered) {
		t.Fatalf("overflow did not advance recovery cursor: delivered=%d latest=%d", delivered, latest)
	}
}
