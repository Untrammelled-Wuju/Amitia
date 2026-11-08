package host_registry_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/deviceruntime/protocol"
	"github.com/u-ai/backend/internal/extension/kernel/host_registry"
	kernelsqlite "github.com/u-ai/backend/internal/extension/kernel/persistence/sqlite"
	"github.com/u-ai/backend/internal/runtimeidentity"
)

func TestRuntimePresenceHeartbeatRejectsStaleGeneration(t *testing.T) {
	db := openPresenceTestDB(t)
	registry := host_registry.NewRegistry(db)
	adapter := host_registry.NewDeviceRuntimePresenceAdapter(registry)
	now := time.Now().UTC()
	current := protocol.PresenceSnapshot{
		SpaceID:              runtimeidentity.SpaceID("space"),
		DeviceID:             runtimeidentity.DeviceID("device"),
		RuntimeID:            runtimeidentity.RuntimeID("runtime"),
		RuntimeSessionID:     runtimeidentity.RuntimeSessionID("session-current"),
		Platform:             runtimeidentity.PlatformAndroid,
		ConnectionGeneration: 5,
	}
	if err := adapter.SessionReady(context.Background(), current); err != nil {
		t.Fatalf("session ready: %v", err)
	}

	stale := current
	stale.RuntimeSessionID = "session-stale"
	stale.ConnectionGeneration = 4
	if err := adapter.Heartbeat(context.Background(), stale, now); !errors.Is(err, host_registry.ErrStaleRuntimeSessionBinding) {
		t.Fatalf("stale heartbeat error=%v want=%v", err, host_registry.ErrStaleRuntimeSessionBinding)
	}
}

func TestRuntimePresenceForDifferentDevicesDoesNotOverwrite(t *testing.T) {
	db := openPresenceTestDB(t)
	registry := host_registry.NewRegistry(db)
	adapter := host_registry.NewDeviceRuntimePresenceAdapter(registry)
	now := time.Now().UTC()
	first := protocol.PresenceSnapshot{
		SpaceID:              runtimeidentity.SpaceID("space"),
		DeviceID:             runtimeidentity.DeviceID("device-a"),
		RuntimeID:            runtimeidentity.RuntimeID("runtime-a"),
		RuntimeSessionID:     runtimeidentity.RuntimeSessionID("session-a"),
		Platform:             runtimeidentity.PlatformAndroid,
		ConnectionGeneration: 1,
		At:                   now,
	}
	second := protocol.PresenceSnapshot{
		SpaceID:              runtimeidentity.SpaceID("space"),
		DeviceID:             runtimeidentity.DeviceID("device-b"),
		RuntimeID:            runtimeidentity.RuntimeID("runtime-b"),
		RuntimeSessionID:     runtimeidentity.RuntimeSessionID("session-b"),
		Platform:             runtimeidentity.PlatformAndroid,
		ConnectionGeneration: 1,
		At:                   now,
	}
	if err := adapter.SessionReady(context.Background(), first); err != nil {
		t.Fatalf("first session ready: %v", err)
	}
	if err := adapter.SessionReady(context.Background(), second); err != nil {
		t.Fatalf("second session ready: %v", err)
	}

	firstPresence, err := registry.ListRuntimePresenceByDevice(
		context.Background(),
		first.SpaceID,
		first.DeviceID,
	)
	if err != nil {
		t.Fatalf("list first device presence: %v", err)
	}
	secondPresence, err := registry.ListRuntimePresenceByDevice(
		context.Background(),
		second.SpaceID,
		second.DeviceID,
	)
	if err != nil {
		t.Fatalf("list second device presence: %v", err)
	}
	if len(firstPresence) != 1 || firstPresence[0].RuntimeID != first.RuntimeID {
		t.Fatalf("first device presence lost: %+v", firstPresence)
	}
	if len(secondPresence) != 1 || secondPresence[0].RuntimeID != second.RuntimeID {
		t.Fatalf("second device presence lost: %+v", secondPresence)
	}
}

func openPresenceTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Errorf("close sqlite: %v", err)
		}
	})
	if err := kernelsqlite.Migrate(context.Background(), db); err != nil {
		t.Fatalf("migrate sqlite: %v", err)
	}
	return db
}
