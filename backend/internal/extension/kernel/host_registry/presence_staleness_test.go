package host_registry

import (
	"testing"
	"time"
)

func TestDevicePresenceExpiresAfterMissedHeartbeats(t *testing.T) {
	now := time.Now().UTC()
	fresh := aggregateDevicePresence([]*RuntimeEntry{{
		SpaceID:              "space-a",
		DeviceID:             "device-a",
		RuntimeID:            "runtime-a",
		PresenceState:        PresenceStateReady,
		LastHeartbeat:        now.Add(-30 * time.Second),
		ConnectionGeneration: 1,
	}})
	if fresh.State != PresenceStateReady {
		t.Fatalf("fresh heartbeat was not ready: %+v", fresh)
	}
	stale := aggregateDevicePresence([]*RuntimeEntry{{
		SpaceID:              "space-a",
		DeviceID:             "device-a",
		RuntimeID:            "runtime-a",
		PresenceState:        PresenceStateReady,
		LastHeartbeat:        now.Add(-91 * time.Second),
		ConnectionGeneration: 1,
	}})
	if stale.State == PresenceStateReady {
		t.Fatalf("stale heartbeat remained ready: %+v", stale)
	}
}
