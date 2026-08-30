package deviceruntime

import (
	"context"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/deviceruntime/protocol"
	"github.com/u-ai/backend/internal/runtimeidentity"
)

type p0SessionStore struct {
	active RuntimeSession
}

func (s *p0SessionStore) Create(_ context.Context, session RuntimeSession) error {
	s.active = session
	return nil
}
func (s *p0SessionStore) Get(_ context.Context, id runtimeidentity.RuntimeSessionID) (RuntimeSession, error) {
	if s.active.ID != id {
		return RuntimeSession{}, ErrRuntimeSessionNotFound
	}
	return s.active, nil
}
func (s *p0SessionStore) GetActiveByRuntime(_ context.Context, userID runtimeidentity.UserID, deviceID runtimeidentity.DeviceID, runtimeID runtimeidentity.RuntimeID) (RuntimeSession, error) {
	if s.active.UserID != userID || s.active.DeviceID != deviceID || s.active.RuntimeID != runtimeID || !s.active.IsActive() {
		return RuntimeSession{}, ErrRuntimeSessionNotFound
	}
	return s.active, nil
}
func (s *p0SessionStore) Update(_ context.Context, session RuntimeSession) error {
	s.active = session
	return nil
}
func (s *p0SessionStore) ListActive(_ context.Context) ([]RuntimeSession, error) {
	return []RuntimeSession{s.active}, nil
}
func (s *p0SessionStore) CloseActiveOnStartup(_ context.Context, _ time.Time, _ string) error {
	return nil
}
func (s *p0SessionStore) UpdateHeartbeat(_ context.Context, _ runtimeidentity.RuntimeSessionID, _ int64, _ time.Time, _ time.Time) error {
	return nil
}
func (s *p0SessionStore) UpdateCursor(_ context.Context, _ runtimeidentity.RuntimeSessionID, _ int64, cursor protocol.SessionCursor, at time.Time) error {
	s.active.LastAppliedStateRevision = cursor.LastAppliedStateRevision
	s.active.LastProcessedCommandSequence = cursor.LastProcessedCommandSequence
	s.active.LastEventSequence = cursor.LastEventSequence
	s.active.ActualStateHash = cursor.ActualStateHash
	s.active.UpdatedAt = at
	return nil
}
func (s *p0SessionStore) UpdateStatus(_ context.Context, _ runtimeidentity.RuntimeSessionID, _ int64, status protocol.SessionStatus, at time.Time) error {
	s.active.Status = status
	s.active.UpdatedAt = at
	return nil
}
func (s *p0SessionStore) Close(_ context.Context, _ runtimeidentity.RuntimeSessionID, _ int64, reason string, at time.Time) error {
	s.active.Status = protocol.SessionStatusClosed
	s.active.CloseReason = reason
	s.active.ClosedAt = &at
	return nil
}
func (s *p0SessionStore) ReplaceForReconnect(_ context.Context, expectedGeneration int64, updated RuntimeSession) error {
	if s.active.ConnectionGeneration != expectedGeneration {
		return ErrConnectionSuperseded
	}
	s.active = updated
	return nil
}

func TestAcquireClientCursorAheadForcesFullResumeAndStillFencesOldGeneration(t *testing.T) {
	now := time.Now().UTC()
	store := &p0SessionStore{active: RuntimeSession{
		ID:                           "session-1",
		UserID:                       "user-1",
		DeviceID:                     "device-1",
		RuntimeID:                    "runtime-1",
		Platform:                     runtimeidentity.PlatformWindows,
		Status:                       protocol.SessionStatusReady,
		ConnectionGeneration:         4,
		LastAppliedStateRevision:     7,
		LastProcessedCommandSequence: 8,
		LastEventSequence:            9,
		ActualStateHash:              "sha256:server",
		Revision:                     3,
		LastHeartbeatAt:              now,
		ExpiresAt:                    now.Add(time.Hour),
	}}
	service, err := NewService(store, ServiceOptions{
		SessionIDFactory: func() runtimeidentity.RuntimeSessionID { return "unused-new-session" },
	})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	result, err := service.Acquire(context.Background(), AcquireRequest{
		Identity: protocol.SessionIdentity{UserID: "user-1", DeviceID: "device-1", RuntimeID: "runtime-1"},
		Platform: runtimeidentity.PlatformWindows,
		Cursor: protocol.SessionCursor{
			LastAppliedStateRevision:     7,
			LastProcessedCommandSequence: 8,
			LastEventSequence:            10, // optimistic client cursor was never committed
			ActualStateHash:              "sha256:client",
		},
		Now: now.Add(time.Second),
	})
	if err != nil {
		t.Fatalf("acquire: %v", err)
	}
	if result.Resume.Mode != protocol.ResumeModeFull || result.Resume.Reason != "client_cursor_ahead" {
		t.Fatalf("resume=%+v want full/client_cursor_ahead", result.Resume)
	}
	if result.Session.ConnectionGeneration != 5 {
		t.Fatalf("generation=%d want=5; old transport must be fenced", result.Session.ConnectionGeneration)
	}
	if result.Session.LastEventSequence != 9 {
		t.Fatalf("server committed event cursor regressed/advanced incorrectly: got=%d want=9", result.Session.LastEventSequence)
	}
	if result.Session.ActualStateHash != "sha256:server" {
		t.Fatalf("full resume must preserve authoritative server state hash: got=%q", result.Session.ActualStateHash)
	}
	if store.active.ConnectionGeneration != 5 {
		t.Fatalf("store was not replaced for reconnect: generation=%d", store.active.ConnectionGeneration)
	}
}
