package server

import (
	"testing"
	"time"

	"github.com/u-ai/backend/internal/runtimeidentity"
)

func TestConnectionHubListBySpaceIsPerDeviceNewestGenerationAndStable(t *testing.T) {
	hub := NewConnectionHub()
	spaceID := runtimeidentity.SpaceID("user-1")
	otherUser := runtimeidentity.SpaceID("user-2")

	attach := func(session string, generation int64, owner runtimeidentity.SpaceID, device string, pong time.Time) {
		conn := &MeshConnection{
			SessionID:   runtimeidentity.RuntimeSessionID(session),
			Generation:  generation,
			SpaceID:     owner,
			DeviceID:    runtimeidentity.DeviceID(device),
			RuntimeID:   runtimeidentity.RuntimeID("runtime-" + session),
			LastPongAt:  pong,
			ConnectedAt: pong,
		}
		if _, attached := hub.Attach(conn.SessionID, conn); !attached {
			t.Fatalf("failed to attach %s", session)
		}
	}

	now := time.Now().UTC()
	attach("session-b-old", 1, spaceID, "device-b", now.Add(10*time.Minute))
	attach("session-a", 1, spaceID, "device-a", now)
	attach("session-b-new", 2, spaceID, "device-b", now.Add(-10*time.Minute))
	attach("session-other", 9, otherUser, "device-z", now.Add(time.Hour))

	connections := hub.ListBySpace(spaceID)
	if len(connections) != 2 {
		t.Fatalf("expected two devices, got %d", len(connections))
	}
	if connections[0].DeviceID != runtimeidentity.DeviceID("device-a") ||
		connections[1].DeviceID != runtimeidentity.DeviceID("device-b") {
		t.Fatalf("expected stable device-id order, got %s then %s", connections[0].DeviceID, connections[1].DeviceID)
	}
	if connections[1].Generation != 2 || connections[1].SessionID != runtimeidentity.RuntimeSessionID("session-b-new") {
		t.Fatalf("expected newest generation for device-b, got generation=%d session=%s", connections[1].Generation, connections[1].SessionID)
	}
	if connections[1].LastPongAt.After(connections[0].LastPongAt) {
		t.Fatal("test invariant invalid: newer generation should intentionally have the older heartbeat")
	}
}
