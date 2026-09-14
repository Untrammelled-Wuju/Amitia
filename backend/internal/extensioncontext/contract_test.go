package extensioncontext

import (
	"encoding/json"
	"testing"
)

func TestDecodeSnapshot(t *testing.T) {
	raw := json.RawMessage(`{"slot":"chat.realtime.schedule","contributions":[{"source":"com.example/plugin:weather","priority":80,"data":{"condition":"sunny"}}]}`)
	snapshot, err := Decode(raw)
	if err != nil {
		t.Fatalf("decode snapshot: %v", err)
	}
	if snapshot.Slot != "chat.realtime.schedule" {
		t.Fatalf("unexpected slot: %s", snapshot.Slot)
	}
	if len(snapshot.Contributions) != 1 {
		t.Fatalf("unexpected contributions: %d", len(snapshot.Contributions))
	}
	if snapshot.Contributions[0].Source != "com.example/plugin:weather" {
		t.Fatalf("unexpected source: %s", snapshot.Contributions[0].Source)
	}
}
