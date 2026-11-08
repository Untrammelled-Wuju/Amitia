package deviceruntime

import (
	"context"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/deviceruntime/protocol"
	"github.com/u-ai/backend/internal/runtimeidentity"
)

type recordingLifecyclePresence struct {
	heartbeat *protocol.PresenceSnapshot
}

func (p *recordingLifecyclePresence) Acquire(
	context.Context,
	protocol.SessionIdentity,
	protocol.SessionCursor,
	string,
	protocol.RuntimeContractVersion,
	[]string,
) (runtimeidentity.RuntimeSessionID, protocol.ResumeDecision, error) {
	return "", protocol.ResumeDecision{}, nil
}

func (p *recordingLifecyclePresence) Heartbeat(_ context.Context, snapshot protocol.PresenceSnapshot, _ time.Time) error {
	copied := snapshot
	p.heartbeat = &copied
	return nil
}

func (p *recordingLifecyclePresence) Close(context.Context, runtimeidentity.RuntimeSessionID, string) error {
	return nil
}

func (p *recordingLifecyclePresence) SessionReady(context.Context, protocol.PresenceSnapshot) error {
	return nil
}

func (p *recordingLifecyclePresence) SessionDisconnected(context.Context, protocol.PresenceSnapshot, string) error {
	return nil
}

func TestSessionHeartbeatProjectsCurrentRuntimePresence(t *testing.T) {
	now := time.Now().UTC()
	store := &p0SessionStore{active: RuntimeSession{
		ID:                   "rtsess-heartbeat",
		SpaceID:              "space-heartbeat",
		DeviceID:             "device-heartbeat",
		RuntimeID:            "runtime-heartbeat",
		Platform:             runtimeidentity.PlatformAndroid,
		Status:               protocol.SessionStatusReady,
		ConnectionGeneration: 7,
		LastHeartbeatAt:      now.Add(-time.Minute),
		ExpiresAt:            now.Add(time.Hour),
	}}
	presence := &recordingLifecyclePresence{}
	service, err := NewService(store, ServiceOptions{PresencePort: presence})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	if _, err := service.Heartbeat(context.Background(), "rtsess-heartbeat", 7, now); err != nil {
		t.Fatalf("heartbeat: %v", err)
	}
	if presence.heartbeat == nil {
		t.Fatal("heartbeat did not reach runtime presence projection")
	}
	if presence.heartbeat.SpaceID != "space-heartbeat" ||
		presence.heartbeat.DeviceID != "device-heartbeat" ||
		presence.heartbeat.RuntimeID != "runtime-heartbeat" ||
		presence.heartbeat.RuntimeSessionID != "rtsess-heartbeat" ||
		presence.heartbeat.ConnectionGeneration != 7 ||
		!presence.heartbeat.At.Equal(now) {
		t.Fatalf("unexpected presence snapshot: %+v", presence.heartbeat)
	}
}
