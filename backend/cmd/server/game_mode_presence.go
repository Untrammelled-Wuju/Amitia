package main

import (
	"context"
	"errors"
	"fmt"

	"github.com/u-ai/backend/internal/devicemesh/gamemode"
	"github.com/u-ai/backend/internal/extension/kernel/host_registry"
	"github.com/u-ai/backend/internal/runtimeidentity"
)

func markGameModeUnknownAfterDisconnect(ctx context.Context, services *AppServices, sessionID string) error {
	if services == nil || services.GameMode == nil || services.DeviceMesh == nil || services.DeviceMesh.GetSessions() == nil {
		return nil
	}
	sessions := services.DeviceMesh.GetSessions()
	current, err := sessions.GetSession(ctx, runtimeidentity.ParseRuntimeSessionID(sessionID))
	if err != nil {
		return fmt.Errorf("read disconnected runtime session: %w", err)
	}
	active, err := sessions.ListActiveSessions(ctx)
	if err != nil {
		return fmt.Errorf("list active runtime sessions: %w", err)
	}
	for _, candidate := range active {
		if candidate.ID != current.ID && candidate.IsActive() && candidate.SpaceID == current.SpaceID && candidate.DeviceID == current.DeviceID {
			return nil
		}
	}
	if _, err := services.GameMode.MarkUnknown(ctx, current.SpaceID.String(), current.DeviceID.String()); err != nil && !errors.Is(err, context.Canceled) {
		return fmt.Errorf("mark game mode unknown after device disconnect: %w", err)
	}
	return nil
}

func reconcileGameModeAfterDeviceReady(ctx context.Context, services *AppServices, spaceID runtimeidentity.SpaceID, deviceID runtimeidentity.DeviceID) error {
	if services == nil || services.GameMode == nil {
		return nil
	}
	if _, err := services.GameMode.MarkReady(ctx, spaceID.String(), deviceID.String()); err != nil {
		return fmt.Errorf("reconcile game mode after device ready: %w", err)
	}
	return nil
}

func reconcileGameModePresence(ctx context.Context, services *AppServices, spaceID runtimeidentity.SpaceID) error {
	if services == nil || services.GameMode == nil || services.KernelContainer == nil || services.KernelContainer.DeviceRegistry == nil {
		return nil
	}
	current, err := services.GameMode.Get(ctx, spaceID.String())
	if err != nil {
		return fmt.Errorf("read game mode before presence reconcile: %w", err)
	}
	if current.State == gamemode.StateInactive {
		return nil
	}
	presence, err := services.KernelContainer.DeviceRegistry.GetDevicePresence(ctx, spaceID, runtimeidentity.ParseDeviceID(current.DeviceID))
	if errors.Is(err, host_registry.ErrDevicePresenceNotFound) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read game mode owner presence: %w", err)
	}
	if presence.State == host_registry.PresenceStateReady {
		return nil
	}
	if _, err := services.GameMode.MarkUnknown(ctx, spaceID.String(), current.DeviceID); err != nil {
		return fmt.Errorf("mark stale game mode owner unknown: %w", err)
	}
	return nil
}
