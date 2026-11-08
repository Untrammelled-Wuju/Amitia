package kernel

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/u-ai/backend/internal/extension/kernel/capability"
)

type recordingGameHostGuard struct {
	spaceID        string
	targetDeviceID string
	toolID         string
	err            error
}

func (g *recordingGameHostGuard) AuthorizeGameHostExecution(_ context.Context, spaceID, targetDeviceID, toolID string) error {
	g.spaceID = spaceID
	g.targetDeviceID = targetDeviceID
	g.toolID = toolID
	return g.err
}

func TestGameHostExecutionGuardUsesResolvedTarget(t *testing.T) {
	guard := &recordingGameHostGuard{}
	facade := NewToolFacade(nil, nil)
	facade.SetLocalDeviceID("local-device")
	facade.SetGameHostExecutionGuard(guard)
	def := capability.ToolDefinition{ID: "vendor/game/move", Runtime: capability.RuntimeBinding{RuntimeType: capability.RuntimeTypeGameHost}}
	err := facade.authorizeGameHostExecution(
		t.Context(),
		def,
		InvocationScope{SpaceID: "space", DeviceID: "caller-device"},
		resolvedExecution{target: capability.InvocationExecutionTarget{DeviceID: "target-device"}},
	)
	if err != nil {
		t.Fatal(err)
	}
	if guard.spaceID != "space" || guard.targetDeviceID != "target-device" || guard.toolID != "vendor/game/move" {
		t.Fatalf("guard received wrong authority context: %+v", guard)
	}
}

func TestGameHostExecutionGuardRejectsDeniedTargetAndPreservesCause(t *testing.T) {
	original := errors.New("game mode owner is reconciling")
	guard := &recordingGameHostGuard{err: original}
	facade := NewToolFacade(nil, nil)
	facade.SetGameHostExecutionGuard(guard)
	def := capability.ToolDefinition{ID: "game.pause", Runtime: capability.RuntimeBinding{RuntimeType: capability.RuntimeTypeGameHost}}
	err := facade.authorizeGameHostExecution(
		t.Context(),
		def,
		InvocationScope{SpaceID: "space", DeviceID: "device-a"},
		resolvedExecution{target: capability.InvocationExecutionTarget{DeviceID: "device-a"}},
	)
	if !errors.Is(err, original) || !strings.Contains(err.Error(), "game.pause") || !strings.Contains(err.Error(), "reconciling") {
		t.Fatalf("guard denial was not preserved: %v", err)
	}
}

func TestGameHostExecutionGuardBypassesInternalCallsAndUnknownTargetFailsClosed(t *testing.T) {
	guard := &recordingGameHostGuard{}
	facade := NewToolFacade(nil, nil)
	facade.SetGameHostExecutionGuard(guard)
	def := capability.ToolDefinition{ID: "game.pause", Runtime: capability.RuntimeBinding{RuntimeType: capability.RuntimeTypeGameHost}}
	if err := facade.authorizeGameHostExecution(t.Context(), def, InvocationScope{SpaceID: "space", IsInternal: true}, resolvedExecution{}); err != nil {
		t.Fatalf("internal execution should bypass game mode lease: %v", err)
	}
	if err := facade.authorizeGameHostExecution(t.Context(), def, InvocationScope{SpaceID: "space"}, resolvedExecution{}); err == nil {
		t.Fatal("game host execution without a target device must fail closed")
	}
}
