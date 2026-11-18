package main

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/auth"
	"github.com/u-ai/backend/internal/conversationstream"
	"github.com/u-ai/backend/internal/devicemesh/business"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/extension/kernel"
	"github.com/u-ai/backend/internal/extension/kernel/host_registry"
)

func TestInterruptOwnedRuntimeTurnCancelsActiveExecution(t *testing.T) {
	manager := conversationstream.NewManager()
	scope := coordination.ExecutionScope{SpaceID: "space-1", CoreID: "core-1", ResourceOwnerID: "device-1", RoleID: "character-1"}
	runtimeKey := business.RuntimeKey(scope, "conversation-1")
	if _, err := manager.Publish(context.Background(), conversationstream.AgentUIEvent{
		ConversationID: runtimeKey,
		TurnID:         "turn-1",
		RequestID:      "request-1",
		ExecutionID:    "execution-1",
		Type:           "turn.started",
		Status:         "running",
	}, false); err != nil {
		t.Fatalf("publish active turn: %v", err)
	}
	executionContext, cancel, started := manager.BeginExecution(runtimeKey, "turn-1")
	if !started {
		t.Fatal("expected active execution")
	}
	defer cancel()
	result, err := interruptOwnedRuntimeTurn(context.Background(), manager, runtimeKey, "conversation-1", "turn-1")
	if err != nil {
		t.Fatalf("interrupt active turn: %v", err)
	}
	if !result.Interrupted || result.TurnID != "turn-1" || result.RequestID != "request-1" || result.ExecutionID != "execution-1" {
		t.Fatalf("unexpected result: %+v", result)
	}
	select {
	case <-executionContext.Done():
	case <-time.After(time.Second):
		t.Fatal("active execution was not cancelled")
	}
}

func TestInterruptOwnedRuntimeTurnRejectsMissingOrMismatchedTurn(t *testing.T) {
	manager := conversationstream.NewManager()
	scope := coordination.ExecutionScope{SpaceID: "space-1", CoreID: "core-1", ResourceOwnerID: "device-1", RoleID: "character-1"}
	runtimeKey := business.RuntimeKey(scope, "conversation-1")
	if _, err := interruptOwnedRuntimeTurn(context.Background(), manager, runtimeKey, "conversation-1", ""); !errors.Is(err, kernel.ErrConversationTurnUnavailable) {
		t.Fatalf("expected unavailable turn, got %v", err)
	}
	if _, err := manager.Publish(context.Background(), conversationstream.AgentUIEvent{
		ConversationID: runtimeKey,
		TurnID:         "turn-1",
		Type:           "turn.started",
		Status:         "running",
	}, false); err != nil {
		t.Fatalf("publish active turn: %v", err)
	}
	if _, err := interruptOwnedRuntimeTurn(context.Background(), manager, runtimeKey, "conversation-1", "turn-2"); !errors.Is(err, kernel.ErrConversationTurnUnavailable) {
		t.Fatalf("expected mismatched turn failure, got %v", err)
	}
}

func TestAwaitOwnedRuntimeTurnIdleWaitsForTerminalEvent(t *testing.T) {
	manager := conversationstream.NewManager()
	scope := coordination.ExecutionScope{SpaceID: "space-1", CoreID: "core-1", ResourceOwnerID: "device-1", RoleID: "character-1"}
	runtimeKey := business.RuntimeKey(scope, "conversation-1")
	if _, err := manager.Publish(context.Background(), conversationstream.AgentUIEvent{
		ConversationID: runtimeKey,
		TurnID:         "turn-1",
		Type:           "turn.started",
		Status:         "running",
	}, false); err != nil {
		t.Fatalf("publish active turn: %v", err)
	}
	done := make(chan error, 1)
	go func() {
		result, err := awaitOwnedRuntimeTurnIdle(context.Background(), manager, runtimeKey, "conversation-1", "turn-1")
		if err == nil && !result.Idle {
			err = errors.New("expected idle result")
		}
		done <- err
	}()
	time.Sleep(10 * time.Millisecond)
	if _, err := manager.Publish(context.Background(), conversationstream.AgentUIEvent{
		ConversationID: runtimeKey,
		TurnID:         "turn-1",
		Type:           "turn.interrupted",
		Status:         "interrupted",
	}, false); err != nil {
		t.Fatalf("publish terminal turn: %v", err)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("turn await did not observe terminal event")
	}
}

func TestConversationTurnControllerResolvesOwnerScopeAndInterrupts(t *testing.T) {
	port := setupMeshLocalDataPort(t)
	database := port.services.KernelContainer.DeviceRegistry.Database()
	if _, err := port.services.KernelContainer.DeviceRegistry.EnsureDevice(t.Context(), host_registry.DeviceRecord{SpaceID: "core", DeviceID: "device-a", TrustState: host_registry.DeviceTrustTrusted}); err != nil {
		t.Fatal(err)
	}
	engine := business.NewEngine(coordination.NewService(database), port, nil)
	ctx := auth.WithActor(t.Context(), &auth.ActorContext{PrincipalType: auth.PrincipalTest, SpaceID: "core", DeviceID: "device-a", Permissions: auth.OwnerDevicePermissions()})
	conversationID := "turn-controller-" + t.Name()
	ctx = kernel.WithHostAPIScope(ctx, kernel.HostAPIScopeContext{SpaceID: "core", CharacterID: "one", ConversationID: conversationID})
	view, err := engine.Query(ctx, business.Request{SpaceID: "core", DeviceID: "device-a", CoreID: "core", RoleID: "one", ConversationID: conversationID}, coordination.DataQuery{Management: true, ResourceKind: "conversation", ConversationID: conversationID})
	if err != nil {
		t.Fatal(err)
	}
	scope := view.Scope
	scope.RequestID = "seed-turn-controller"
	if _, err := port.store.Apply(ctx, coordination.Commit{Scope: scope, Mutations: []coordination.Mutation{{Kind: "conversation", ID: conversationID, RoleID: "one", Body: json.RawMessage(`{"id":"` + conversationID + `","characterId":"one","channel":"web"}`)}}}); err != nil {
		t.Fatal(err)
	}
	manager := business.RuntimeManager()
	runtimeKey := business.RuntimeKey(view.Scope, conversationID)
	if _, err := manager.Publish(ctx, conversationstream.AgentUIEvent{
		ConversationID: runtimeKey,
		TurnID:         "turn-1",
		RequestID:      "request-1",
		ExecutionID:    "execution-1",
		Type:           "turn.started",
		Status:         "running",
	}, false); err != nil {
		t.Fatal(err)
	}
	executionContext, cancel, started := manager.BeginExecution(runtimeKey, "turn-1")
	if !started {
		t.Fatal("expected active execution")
	}
	defer func() {
		cancel()
		manager.ClearExecution(runtimeKey, "turn-1")
	}()
	controller := &conversationTurnControllerAdapter{engine: engine, coreID: "core"}
	result, err := controller.InterruptConversationTurn(ctx, kernel.ConversationTurnInterruptRequest{
		SpaceID:        "core",
		CharacterID:    "one",
		ConversationID: conversationID,
		TurnID:         "turn-1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Interrupted || result.TurnID != "turn-1" || result.RequestID != "request-1" || result.ExecutionID != "execution-1" {
		t.Fatalf("unexpected result: %+v", result)
	}
	select {
	case <-executionContext.Done():
	case <-time.After(time.Second):
		t.Fatal("resolved owner execution was not cancelled")
	}
}
