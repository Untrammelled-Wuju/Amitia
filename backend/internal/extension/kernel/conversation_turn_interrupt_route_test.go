package kernel

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/u-ai/backend/internal/extension/kernel/domain"
	"github.com/u-ai/backend/internal/extension/kernel/host_api"
	"github.com/u-ai/backend/internal/extension/kernel/runtime_supervisor"
	"github.com/u-ai/backend/internal/extension/kernel/scope"
)

type recordingConversationTurnController struct {
	request     ConversationTurnInterruptRequest
	result      ConversationTurnInterruptResult
	await       ConversationTurnAwaitResult
	err         error
	awaitErr    error
	called      bool
	awaitCalled bool
}

func (r *recordingConversationTurnController) InterruptConversationTurn(_ context.Context, request ConversationTurnInterruptRequest) (ConversationTurnInterruptResult, error) {
	r.called = true
	r.request = request
	return r.result, r.err
}

func (r *recordingConversationTurnController) AwaitConversationTurnIdle(_ context.Context, request ConversationTurnInterruptRequest) (ConversationTurnAwaitResult, error) {
	r.awaitCalled = true
	r.request = request
	return r.await, r.awaitErr
}

func conversationTurnInterruptGateway(t *testing.T, controller ConversationTurnController) *host_api.DefaultGateway {
	t.Helper()
	gateway := newConversationMessageTestGateway()
	if err := setupDefaultHostAPIRoutes(gateway, HostAPIRouteDeps{
		ScopeSnapshotStore: &fixedConversationScopeStore{
			snapshot: &scope.ScopeSnapshot{
				SnapshotID:     "scope-1",
				SpaceID:        "space-1",
				CharacterID:    "character-1",
				ConversationID: "conversation-1",
			},
		},
		ConversationTurnController: controller,
	}); err != nil {
		t.Fatalf("setup routes: %v", err)
	}
	return gateway
}

func TestConversationTurnInterruptRouteUsesResolvedScope(t *testing.T) {
	permissions := host_api.RoutePermissionForMethod(host_api.MethodConversationTurnInterrupt)
	if len(permissions) != 1 || permissions[0].Name != "message.interrupt" || permissions[0].Resource != "conversation" {
		t.Fatalf("unexpected permissions: %+v", permissions)
	}
	policy := host_api.RouteScopeForMethod(host_api.MethodConversationTurnInterrupt)
	if !policy.Namespaced {
		t.Fatal("expected namespaced scope policy")
	}
	controller := &recordingConversationTurnController{
		result: ConversationTurnInterruptResult{
			Interrupted: true,
			TurnID:      "turn-1",
			RequestID:   "request-1",
			ExecutionID: "execution-1",
		},
	}
	gateway := conversationTurnInterruptGateway(t, controller)
	result := gateway.Call(context.Background(), host_api.CallRequest{
		CallID:          "call-interrupt",
		RuntimeIdentity: runtime_supervisor.RuntimeIdentity{InstanceID: "runtime-1", ExtensionID: domain.ExtensionID("com.amitia/dsh-compatible-host"), ModuleID: "dsh-compatible-runtime"},
		Method:          host_api.MethodConversationTurnInterrupt,
		Version:         1,
		Input:           json.RawMessage(`{"turnId":"turn-1"}`),
		ScopeSnapshotID: "scope-1",
	})
	if result.Status != host_api.StatusSuccess {
		t.Fatalf("expected success, got %s: %+v", result.Status, result.Error)
	}
	if !controller.called {
		t.Fatal("expected controller to be called")
	}
	if controller.request != (ConversationTurnInterruptRequest{SpaceID: "space-1", CharacterID: "character-1", ConversationID: "conversation-1", TurnID: "turn-1"}) {
		t.Fatalf("unexpected controller request: %+v", controller.request)
	}
	var output ConversationTurnInterruptResult
	if err := json.Unmarshal(result.Output, &output); err != nil {
		t.Fatalf("decode output: %v", err)
	}
	if !output.Interrupted || output.TurnID != "turn-1" || output.RequestID != "request-1" || output.ExecutionID != "execution-1" {
		t.Fatalf("unexpected output: %+v", output)
	}
}

func TestConversationTurnInterruptRouteRejectsScopeMismatch(t *testing.T) {
	controller := &recordingConversationTurnController{result: ConversationTurnInterruptResult{Interrupted: true}}
	gateway := conversationTurnInterruptGateway(t, controller)
	result := gateway.Call(context.Background(), host_api.CallRequest{
		CallID:          "call-interrupt-mismatch",
		RuntimeIdentity: runtime_supervisor.RuntimeIdentity{InstanceID: "runtime-1", ExtensionID: domain.ExtensionID("com.amitia/dsh-compatible-host"), ModuleID: "dsh-compatible-runtime"},
		Method:          host_api.MethodConversationTurnInterrupt,
		Version:         1,
		Input:           json.RawMessage(`{"conversationId":"conversation-other"}`),
		ScopeSnapshotID: "scope-1",
	})
	if result.Status != host_api.StatusRejected || result.Error == nil || result.Error.Code != host_api.ErrorCodeScopeDenied {
		t.Fatalf("expected scope denied, got %s: %+v", result.Status, result.Error)
	}
	if controller.called {
		t.Fatal("controller must not be called on scope mismatch")
	}
}

func TestConversationTurnInterruptRouteAcceptsTargetCapability(t *testing.T) {
	controller := &recordingConversationTurnController{result: ConversationTurnInterruptResult{Interrupted: true, TurnID: "turn-target"}}
	snapshots := scope.NewMemoryScopeStore()
	parent := scope.ScopeSnapshot{
		SnapshotID:     "scope-parent",
		SpaceID:        "space-1",
		CharacterID:    "character-1",
		ConversationID: "conversation-parent",
	}
	target := scope.ScopeSnapshot{
		SnapshotID:     conversationTargetSnapshotPrefix + "fixture",
		SpaceID:        "space-1",
		CharacterID:    "character-1",
		ConversationID: "conversation-target",
		ExtensionID:    "com.amitia/dsh-compatible-host",
		ModuleID:       "dsh-compatible-runtime",
	}
	if err := snapshots.SaveSnapshot(t.Context(), parent); err != nil {
		t.Fatal(err)
	}
	if err := snapshots.SaveSnapshot(t.Context(), target); err != nil {
		t.Fatal(err)
	}
	gateway := newConversationMessageTestGateway()
	if err := setupDefaultHostAPIRoutes(gateway, HostAPIRouteDeps{
		ScopeSnapshotStore:         host_api.NewSnapshotStoreAdapter(snapshots),
		ScopeSnapshotWriter:        snapshots,
		ConversationTurnController: controller,
	}); err != nil {
		t.Fatal(err)
	}
	result := gateway.Call(context.Background(), host_api.CallRequest{
		CallID:                "call-interrupt-target",
		RuntimeIdentity:       runtime_supervisor.RuntimeIdentity{InstanceID: "runtime-1", ExtensionID: domain.ExtensionID("com.amitia/dsh-compatible-host"), ModuleID: "dsh-compatible-runtime"},
		Method:                host_api.MethodConversationTurnInterrupt,
		Version:               1,
		Input:                 json.RawMessage(`{"conversationId":"conversation-target"}`),
		ScopeSnapshotID:       "scope-parent",
		TargetScopeSnapshotID: target.SnapshotID,
	})
	if result.Status != host_api.StatusSuccess {
		t.Fatalf("expected success, got %s: %+v", result.Status, result.Error)
	}
	if controller.request.ConversationID != "conversation-target" {
		t.Fatalf("unexpected target conversation: %+v", controller.request)
	}
}

func TestConversationTurnAwaitRouteUsesResolvedScope(t *testing.T) {
	permissions := host_api.RoutePermissionForMethod(host_api.MethodConversationTurnAwait)
	if len(permissions) != 1 || permissions[0].Name != "conversation.read" || permissions[0].Resource != "conversation" {
		t.Fatalf("unexpected permissions: %+v", permissions)
	}
	policy := host_api.RouteScopeForMethod(host_api.MethodConversationTurnAwait)
	if !policy.Namespaced {
		t.Fatal("expected namespaced scope policy")
	}
	controller := &recordingConversationTurnController{await: ConversationTurnAwaitResult{Idle: true}}
	gateway := conversationTurnInterruptGateway(t, controller)
	result := gateway.Call(context.Background(), host_api.CallRequest{
		CallID:          "call-await",
		RuntimeIdentity: runtime_supervisor.RuntimeIdentity{InstanceID: "runtime-1", ExtensionID: domain.ExtensionID("com.amitia/dsh-compatible-host"), ModuleID: "dsh-compatible-runtime"},
		Method:          host_api.MethodConversationTurnAwait,
		Version:         1,
		Input:           json.RawMessage(`{}`),
		ScopeSnapshotID: "scope-1",
	})
	if result.Status != host_api.StatusSuccess {
		t.Fatalf("expected success, got %s: %+v", result.Status, result.Error)
	}
	if !controller.awaitCalled {
		t.Fatal("expected awaiter to be called")
	}
	if controller.request != (ConversationTurnInterruptRequest{SpaceID: "space-1", CharacterID: "character-1", ConversationID: "conversation-1"}) {
		t.Fatalf("unexpected controller request: %+v", controller.request)
	}
	var output ConversationTurnAwaitResult
	if err := json.Unmarshal(result.Output, &output); err != nil {
		t.Fatalf("decode output: %v", err)
	}
	if !output.Idle {
		t.Fatalf("unexpected output: %+v", output)
	}
}

func TestConversationTurnInterruptRoutePreservesUnavailableCause(t *testing.T) {
	controller := &recordingConversationTurnController{
		err: fmt.Errorf("%w: conversation conversation-1 has no active turn", ErrConversationTurnUnavailable),
	}
	gateway := conversationTurnInterruptGateway(t, controller)
	result := gateway.Call(context.Background(), host_api.CallRequest{
		CallID:          "call-interrupt-unavailable",
		RuntimeIdentity: runtime_supervisor.RuntimeIdentity{InstanceID: "runtime-1", ExtensionID: domain.ExtensionID("com.amitia/dsh-compatible-host"), ModuleID: "dsh-compatible-runtime"},
		Method:          host_api.MethodConversationTurnInterrupt,
		Version:         1,
		Input:           json.RawMessage(`{}`),
		ScopeSnapshotID: "scope-1",
	})
	if result.Status != host_api.StatusRejected || result.Error == nil || result.Error.Code != host_api.ErrorCodeStateConflict {
		t.Fatalf("expected state conflict, got %s: %+v", result.Status, result.Error)
	}
	if !strings.Contains(result.Error.Message, "no active turn") {
		t.Fatalf("expected original error detail, got %q", result.Error.Message)
	}
}
