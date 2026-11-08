package main

import (
	"strings"
	"testing"

	"github.com/u-ai/backend/internal/devicemesh/gamemode"
)

func TestGameModeExecutionGuardRequiresOwningTarget(t *testing.T) {
	service := newGameModeRouterTestService(t)
	guard := &gameModeExecutionGuard{service: service}
	if err := guard.AuthorizeGameHostExecution(t.Context(), "space-a", "device-a", "game.move"); err == nil || !strings.Contains(err.Error(), "not active") {
		t.Fatalf("inactive game mode did not reject tool: %v", err)
	}
	current, err := service.Acquire(t.Context(), gamemode.AcquireInput{SpaceID: "space-a", DeviceID: "device-a"})
	if err != nil {
		t.Fatal(err)
	}
	if err := guard.AuthorizeGameHostExecution(t.Context(), "space-a", "device-a", "game.move"); err != nil {
		t.Fatalf("owner target was rejected: %v", err)
	}
	if err := guard.AuthorizeGameHostExecution(t.Context(), "space-a", "device-b", "game.move"); err == nil || !strings.Contains(err.Error(), "owned by device device-a") {
		t.Fatalf("non-owner target was not rejected: %v", err)
	}
	if _, err := service.MarkUnknown(t.Context(), "space-a", "device-a"); err != nil {
		t.Fatal(err)
	}
	if err := guard.AuthorizeGameHostExecution(t.Context(), "space-a", "device-a", "game.move"); err == nil || !strings.Contains(err.Error(), "unknown") {
		t.Fatalf("unknown owner was not rejected: %v", err)
	}
	if current.DeviceID != "device-a" {
		t.Fatalf("unexpected initial owner: %+v", current)
	}
}
