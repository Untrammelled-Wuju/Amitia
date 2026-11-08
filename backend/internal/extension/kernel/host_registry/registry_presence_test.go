package host_registry

import (
	"context"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/runtimeidentity"
)

func TestDisconnectMissingRuntimePresenceIsIdempotent(t *testing.T) {
	registry := NewRegistry(nil)
	err := registry.DisconnectRuntimeSession(context.Background(), RuntimeSessionBinding{
		SpaceID:              runtimeidentity.SpaceID("space"),
		DeviceID:             runtimeidentity.DeviceID("device"),
		RuntimeID:            runtimeidentity.RuntimeID("runtime"),
		RuntimeSessionID:     runtimeidentity.RuntimeSessionID("session"),
		ConnectionGeneration: 1,
		At:                   time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("missing presence disconnect should be idempotent: %v", err)
	}
}
