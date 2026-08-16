package host_registry

import (
	"context"
	"time"

	"github.com/u-ai/backend/internal/deviceruntime/protocol"
	"github.com/u-ai/backend/internal/runtimeidentity"
)

type ProviderInstanceUnavailableFunc func(userID runtimeidentity.UserID, deviceID runtimeidentity.DeviceID, runtimeID runtimeidentity.RuntimeID)

type DeviceRuntimePresenceAdapter struct {
	registry    *Registry
	onDisconnected ProviderInstanceUnavailableFunc
}

func NewDeviceRuntimePresenceAdapter(registry *Registry) *DeviceRuntimePresenceAdapter {
	return &DeviceRuntimePresenceAdapter{registry: registry}
}

func NewDeviceRuntimePresenceAdapterWithCallback(registry *Registry, onDisconnected ProviderInstanceUnavailableFunc) *DeviceRuntimePresenceAdapter {
	return &DeviceRuntimePresenceAdapter{
		registry:       registry,
		onDisconnected: onDisconnected,
	}
}

func (a *DeviceRuntimePresenceAdapter) Acquire(
	ctx context.Context,
	identity protocol.SessionIdentity,
	cursor protocol.SessionCursor,
	runtimeVersion string,
	contractVersion protocol.RuntimeContractVersion,
	capabilities []string,
) (runtimeidentity.RuntimeSessionID, protocol.ResumeDecision, error) {
	return "", protocol.ResumeDecision{}, nil
}

func (a *DeviceRuntimePresenceAdapter) Heartbeat(
	ctx context.Context,
	sessionID runtimeidentity.RuntimeSessionID,
	at time.Time,
) error {
	return nil
}

func (a *DeviceRuntimePresenceAdapter) Close(
	ctx context.Context,
	sessionID runtimeidentity.RuntimeSessionID,
	reason string,
) error {
	return nil
}

func (a *DeviceRuntimePresenceAdapter) SessionReady(ctx context.Context, snapshot protocol.PresenceSnapshot) error {
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

func (a *DeviceRuntimePresenceAdapter) SessionDisconnected(ctx context.Context, snapshot protocol.PresenceSnapshot, reason string) error {
	err := a.registry.DisconnectRuntimeSession(ctx, RuntimeSessionBinding{
		UserID:               snapshot.UserID,
		DeviceID:             snapshot.DeviceID,
		RuntimeID:            snapshot.RuntimeID,
		RuntimeSessionID:     snapshot.RuntimeSessionID,
		Platform:             snapshot.Platform,
		ConnectionGeneration: snapshot.ConnectionGeneration,
		At:                   snapshot.At,
	})

	if err == nil && a.onDisconnected != nil {
		a.onDisconnected(snapshot.UserID, snapshot.DeviceID, snapshot.RuntimeID)
	}

	return err
}

var _ protocol.SessionLifecyclePort = (*DeviceRuntimePresenceAdapter)(nil)
