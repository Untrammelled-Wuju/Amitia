package kernel

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/u-ai/backend/internal/extension/kernel/host_api"
)

type recordingConversationTurnsReader struct {
	conversationID string
	limit          int
	output         []json.RawMessage
	err            error
	called         bool
}

func (r *recordingConversationTurnsReader) ReadConversationTurns(_ context.Context, conversationID string, limit int) ([]json.RawMessage, error) {
	r.called = true
	r.conversationID = conversationID
	r.limit = limit
	return r.output, r.err
}

func TestConversationTurnsRouteUsesReadPolicy(t *testing.T) {
	method := host_api.MethodConversationTurns
	permissions := host_api.RoutePermissionForMethod(method)
	if len(permissions) != 1 || permissions[0].Name != "conversation.read" {
		t.Fatalf("unexpected permissions for %s: %+v", method, permissions)
	}
	if risk := host_api.RouteRiskForMethod(method); risk != host_api.RiskLow {
		t.Fatalf("unexpected risk for %s: %s", method, risk)
	}
	if effect := host_api.RouteSideEffectForMethod(method); effect != host_api.SideEffectReadOnly {
		t.Fatalf("unexpected side effect for %s: %s", method, effect)
	}
	if !host_api.IsDataRouteMethod(method) {
		t.Fatalf("expected data route for %s", method)
	}
	policy := host_api.RouteScopeForMethod(method)
	if len(policy.RequireRoles) != 1 || policy.RequireRoles[0] != "conversation" {
		t.Fatalf("unexpected scope policy for %s: %+v", method, policy)
	}
}

func TestConversationTurnsRouteForwardsRequest(t *testing.T) {
	reader := &recordingConversationTurnsReader{output: []json.RawMessage{json.RawMessage(`{"turnId":"turn-1","turnSequence":1,"blocks":[]}`)}}
	gateway := projectRouteGateway(t, HostAPIRouteDeps{ConversationTurns: reader})
	result := gateway.Call(context.Background(), projectRouteCall(host_api.MethodConversationTurns, `{"conversationId":"conversation-1","limit":500}`))
	if result.Status != host_api.StatusSuccess {
		t.Fatalf("expected success, got %s: %+v", result.Status, result.Error)
	}
	if !reader.called || reader.conversationID != "conversation-1" || reader.limit != 50 {
		t.Fatalf("unexpected turns read: %+v", reader)
	}
	var payload struct {
		ConversationID string            `json:"conversationId"`
		Turns          []json.RawMessage `json:"turns"`
	}
	if err := json.Unmarshal(result.Output, &payload); err != nil {
		t.Fatalf("decode output: %v", err)
	}
	if payload.ConversationID != "conversation-1" || len(payload.Turns) != 1 {
		t.Fatalf("unexpected output: %+v", payload)
	}
}

func TestConversationTurnsRouteKeepsExplicitLimit(t *testing.T) {
	reader := &recordingConversationTurnsReader{output: []json.RawMessage{}}
	gateway := projectRouteGateway(t, HostAPIRouteDeps{ConversationTurns: reader})
	result := gateway.Call(context.Background(), projectRouteCall(host_api.MethodConversationTurns, `{"conversationId":"conversation-1","limit":120}`))
	if result.Status != host_api.StatusSuccess {
		t.Fatalf("expected success, got %s: %+v", result.Status, result.Error)
	}
	if reader.limit != 120 {
		t.Fatalf("expected limit 120, got %d", reader.limit)
	}
}

func TestConversationTurnsRouteRequiresConversationID(t *testing.T) {
	reader := &recordingConversationTurnsReader{}
	gateway := projectRouteGateway(t, HostAPIRouteDeps{ConversationTurns: reader})
	result := gateway.Call(context.Background(), projectRouteCall(host_api.MethodConversationTurns, `{"limit":10}`))
	if result.Status == host_api.StatusSuccess || reader.called {
		t.Fatalf("expected input rejection, got %s", result.Status)
	}
}

type recordingConversationUpdater struct {
	request ConversationUpdateRequest
	output  json.RawMessage
	called  bool
}

func (r *recordingConversationUpdater) UpdateConversation(_ context.Context, request ConversationUpdateRequest) (json.RawMessage, error) {
	r.called = true
	r.request = request
	return r.output, nil
}

type recordingTurnSteerer struct {
	request ConversationTurnSteerRequest
	called  bool
}

func (r *recordingTurnSteerer) InterruptConversationTurn(_ context.Context, _ ConversationTurnInterruptRequest) (ConversationTurnInterruptResult, error) {
	return ConversationTurnInterruptResult{}, nil
}

func (r *recordingTurnSteerer) AwaitConversationTurnIdle(_ context.Context, _ ConversationTurnInterruptRequest) (ConversationTurnAwaitResult, error) {
	return ConversationTurnAwaitResult{}, nil
}

func (r *recordingTurnSteerer) SteerConversationTurn(_ context.Context, request ConversationTurnSteerRequest) (ConversationTurnSteerResult, error) {
	r.called = true
	r.request = request
	return ConversationTurnSteerResult{Accepted: true, TurnID: request.TurnID}, nil
}

func TestConversationUpdateRouteForwardsTitle(t *testing.T) {
	updater := &recordingConversationUpdater{output: json.RawMessage(`{"conversationId":"conversation-1","title":"新标题"}`)}
	gateway := projectRouteGateway(t, HostAPIRouteDeps{ConversationUpdater: updater})
	result := gateway.Call(context.Background(), projectRouteCall(host_api.MethodConversationUpdate, `{"conversationId":"conversation-1","title":"新标题","requestId":"rename-1"}`))
	if result.Status != host_api.StatusSuccess {
		t.Fatalf("expected success, got %s: %+v", result.Status, result.Error)
	}
	if !updater.called || updater.request.Title != "新标题" || updater.request.ConversationID != "conversation-1" || updater.request.RequestID != "rename-1" {
		t.Fatalf("unexpected update request: %+v", updater.request)
	}
}

func TestConversationUpdateRouteRejectsEmptyTitle(t *testing.T) {
	updater := &recordingConversationUpdater{output: json.RawMessage(`{}`)}
	gateway := projectRouteGateway(t, HostAPIRouteDeps{ConversationUpdater: updater})
	result := gateway.Call(context.Background(), projectRouteCall(host_api.MethodConversationUpdate, `{"conversationId":"conversation-1","title":"  "}`))
	if result.Status == host_api.StatusSuccess || updater.called {
		t.Fatalf("expected input rejection, got %s", result.Status)
	}
}

func TestConversationTurnSteerRouteForwardsInput(t *testing.T) {
	steerer := &recordingTurnSteerer{}
	gateway := projectRouteGateway(t, HostAPIRouteDeps{ConversationTurnController: steerer})
	result := gateway.Call(context.Background(), projectRouteCall(host_api.MethodConversationTurnSteer, `{"conversationId":"conversation-1","turnId":"turn-1","input":"先看日志"}`))
	if result.Status != host_api.StatusSuccess {
		t.Fatalf("expected success, got %s: %+v", result.Status, result.Error)
	}
	if !steerer.called || steerer.request.Input != "先看日志" || steerer.request.TurnID != "turn-1" {
		t.Fatalf("unexpected steer request: %+v", steerer.request)
	}
}

func TestConversationTurnSteerRouteRejectsEmptyInput(t *testing.T) {
	steerer := &recordingTurnSteerer{}
	gateway := projectRouteGateway(t, HostAPIRouteDeps{ConversationTurnController: steerer})
	result := gateway.Call(context.Background(), projectRouteCall(host_api.MethodConversationTurnSteer, `{"conversationId":"conversation-1","input":"  "}`))
	if result.Status == host_api.StatusSuccess || steerer.called {
		t.Fatalf("expected input rejection, got %s", result.Status)
	}
}
