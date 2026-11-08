package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/u-ai/backend/internal/devicemesh/gamemode"
	"github.com/u-ai/backend/internal/gamehost/management"
)

type gameModeExecutionGuard struct {
	service *gamemode.Service
}

func gameHostBehaviorAuthorizer(services *AppServices) management.BehaviorInvocationAuthorizer {
	if services == nil || services.GameMode == nil {
		return nil
	}
	guard := &gameModeExecutionGuard{service: services.GameMode}
	return func(ctx context.Context, spaceID, targetDeviceID, pluginID, behaviorID string) error {
		if err := guard.AuthorizeGameHostExecution(ctx, spaceID, targetDeviceID, pluginID+"/"+behaviorID); err != nil {
			return fmt.Errorf("game behavior %s on plugin %s: %w", behaviorID, pluginID, err)
		}
		return nil
	}
}

func (g *gameModeExecutionGuard) AuthorizeGameHostExecution(ctx context.Context, spaceID, targetDeviceID, toolID string) error {
	if g == nil || g.service == nil {
		return fmt.Errorf("game mode authority is unavailable")
	}
	session, err := g.service.Get(ctx, spaceID)
	if err != nil {
		return err
	}
	if session.State == gamemode.StateInactive {
		return fmt.Errorf("game mode is not active for Space %s", spaceID)
	}
	if strings.TrimSpace(session.DeviceID) != strings.TrimSpace(targetDeviceID) {
		return fmt.Errorf("game mode is owned by device %s, not target device %s", session.DeviceID, targetDeviceID)
	}
	switch session.State {
	case gamemode.StateActive, gamemode.StatePaused:
		return nil
	case gamemode.StateUnknown:
		return fmt.Errorf("game mode owner device %s is unknown after disconnect", session.DeviceID)
	case gamemode.StateReconciling:
		return fmt.Errorf("game mode owner device %s is reconciling state", session.DeviceID)
	default:
		return fmt.Errorf("game mode state %s does not allow tool %s", session.State, toolID)
	}
}
