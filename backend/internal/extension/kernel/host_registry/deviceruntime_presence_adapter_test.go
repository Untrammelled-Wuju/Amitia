package host_registry

import (
	"context"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/deviceruntime/protocol"
	"github.com/u-ai/backend/internal/runtimeidentity"
)

func TestRuntimePresenceHeartbeatRepairsMissingEntry(t *testing.T) {
	registry := NewRegistry(nil)
	adapter := NewDeviceRuntimePresenceAdapter(registry)
	now := time.Now().UTC()
	snapshot := protocol.PresenceSnapshot{
		SpaceID:              runtimeidentity.SpaceID("space"),
		DeviceID:             runtimeidentity.DeviceID("device"),
		RuntimeID:            runtimeidentity.RuntimeID("runtime"),
		RuntimeSessionID:     runtimeidentity.RuntimeSessionID("session"),
		Platform:             runtimeidentity.PlatformAndroid,
		ConnectionGeneration: 3,
	}

	if err := adapter.Heartbeat(context.Background(), snapshot, now); err != nil {
		t.Fatalf("heartbeat repair: %v", err)
	}
	entryID := RuntimeEntryID(snapshot.SpaceID, snapshot.DeviceID, snapshot.RuntimeID)
	entry := registry.entries[entryID]
	if entry == nil {
		t.Fatal("heartbeat did not restore missing runtime presence")
	}
	if entry.PresenceState != PresenceStateReady || entry.ConnectionGeneration != 3 || !entry.LastHeartbeat.Equal(now) {
		t.Fatalf("unexpected repaired entry: %+v", entry)
	}
}
