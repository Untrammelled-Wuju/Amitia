package main

import (
	"bytes"
	"testing"
)

func TestRound94DurableHeartbeatRequestsSnapshotForUnseenCrossCoreEvents(t *testing.T) {
	for _, item := range []struct {
		delivered, durable int64
		resync             bool
	}{
		{0, 0, false}, {3, 3, false}, {3, 2, false}, {3, 4, true}, {3, 6, true},
	} {
		actual := ownedRuntimeDurableCursorAdvanced(item.delivered, item.durable)
		if actual != item.resync {
			t.Fatalf("delivered=%d durable=%d resync=%v", item.delivered, item.durable, actual)
		}
	}
	var frame bytes.Buffer
	if !ownedRuntimeDurableCursorAdvanced(3, 4) {
		t.Fatal("remote event was not detected")
	}
	if err := writeOwnedRuntimeSnapshotRequired(&frame); err != nil {
		t.Fatal(err)
	}
	if frame.String() != "event: snapshot_required\ndata: {}\n\n" {
		t.Fatalf("bad snapshot recovery frame: %q", frame.String())
	}
}
