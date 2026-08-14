package host_registry

import (
	"context"

	"github.com/u-ai/backend/internal/deviceruntime"
)

type DeviceRuntimePresenceAdapter struct {
	registry *Registry
}

func NewDeviceRuntimePresenceAdapter(registry *Registry) *DeviceRuntimePresenceAdapter {
	return &DeviceRuntimePresenceAdapter{registry: registry}
}

func (a *DeviceRuntimePresenceAdapter) SessionReady(ctx context.Context, snapshot deviceruntime.PresenceSnapshot) error {
	_, err := a.registry.BindRuntimeSession(ctx, RuntimeSessionBinding{
		UserID:               snapshot.UserID,
		DeviceID:             snapshot.DeviceID,
		RuntimeID:            snapshot.RuntimeID,
		RuntimeSessionID:     snapshot.RuntimeSessionID,
		Platform:             snapshot.Platform,
		ConnectionGeneration: snapshot.ConnectionGeneration,
		At:                   snapshot.At,
	})
	return err
}

func (a *DeviceRuntimePresenceAdapter) Heartbeat(ctx context.Context, snapshot deviceruntime.PresenceSnapshot) error {
	return a.registry.HeartbeatRuntimeSession(ctx, RuntimeSessionBinding{
		UserID:               snapshot.UserID,
		DeviceID:             snapshot.DeviceID,
		RuntimeID:            snapshot.RuntimeID,
		RuntimeSessionID:     snapshot.RuntimeSessionID,
		Platform:             snapshot.Platform,
		ConnectionGeneration: snapshot.ConnectionGeneration,
		At:                   snapshot.At,
	})
}

func (a *DeviceRuntimePresenceAdapter) SessionDisconnected(ctx context.Context, snapshot deviceruntime.PresenceSnapshot, reason string) error {
	return a.registry.DisconnectRuntimeSession(ctx, RuntimeSessionBinding{
		UserID:               snapshot.UserID,
		DeviceID:             snapshot.DeviceID,
		RuntimeID:            snapshot.RuntimeID,
		RuntimeSessionID:     snapshot.RuntimeSessionID,
		Platform:             snapshot.Platform,
		ConnectionGeneration: snapshot.ConnectionGeneration,
		At:                   snapshot.At,
	})
}

var _ deviceruntime.PresencePort = (*DeviceRuntimePresenceAdapter)(nil)
