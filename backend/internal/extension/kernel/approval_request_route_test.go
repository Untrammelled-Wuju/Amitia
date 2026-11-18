package kernel

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/domain"
	"github.com/u-ai/backend/internal/extension/kernel/execution"
	"github.com/u-ai/backend/internal/extension/kernel/host_api"
	"github.com/u-ai/backend/internal/extension/kernel/runtime_supervisor"
	"github.com/u-ai/backend/internal/extension/kernel/scope"
)

func approvalScopeSnapshot() *scope.ScopeSnapshot {
	return &scope.ScopeSnapshot{
		SnapshotID:     conversationTargetSnapshotPrefix + "approval",
		SpaceID:        "space-1",
		CharacterID:    "character-1",
		ConversationID: "conversation-1",
		ExtensionID:    "com.example/test",
		ModuleID:       "main",
		Generation:     1,
		ResolvedScopes: []scope.ScopeRef{
			scope.NewExtensionScope("com.example/test"),
			scope.NewModuleScope("com.example/test", "main"),
			scope.NewCharacterScope("character-1"),
			scope.NewConversationScope("conversation-1"),
		},
	}
}

func approvalGateway(t *testing.T, snapshot *scope.ScopeSnapshot, broker *execution.ApprovalBroker) *host_api.DefaultGateway {
	t.Helper()
	gateway := newConversationMessageTestGateway()
	if err := setupDefaultHostAPIRoutes(gateway, HostAPIRouteDeps{
		ScopeSnapshotStore: &fixedConversationScopeStore{snapshot: snapshot},
		ApprovalBroker:     broker,
	}); err != nil {
		t.Fatalf("setup routes: %v", err)
	}
	return gateway
}

func approvalCallRequest() host_api.CallRequest {
	return host_api.CallRequest{
		CallID:                "call-approval",
		RuntimeIdentity:       runtime_supervisor.RuntimeIdentity{InstanceID: "runtime-1", ExtensionID: domain.ExtensionID("com.example/test"), ModuleID: "main", Generation: 1},
		Method:                host_api.MethodApprovalRequest,
		Version:               1,
		Input:                 json.RawMessage(`{"toolName":"amitia_probe","riskLevel":"medium","timeoutMs":5000,"conversationId":"conversation-1"}`),
		ScopeSnapshotID:       conversationTargetSnapshotPrefix + "approval",
		TargetScopeSnapshotID: conversationTargetSnapshotPrefix + "approval",
	}
}

func decodeApprovalOutput(t *testing.T, result host_api.CallResult) bool {
	t.Helper()
	if result.Status != host_api.StatusSuccess {
		t.Fatalf("expected success, got %s: %+v", result.Status, result.Error)
	}
	var out struct {
		Approved bool `json:"approved"`
	}
	if err := json.Unmarshal(result.Output, &out); err != nil {
		t.Fatalf("decode approval output: %v", err)
	}
	return out.Approved
}

func TestHostApprovalRequestResolvesApproved(t *testing.T) {
	broker := execution.NewApprovalBroker()
	ids := make(chan string, 1)
	broker.AddObservers(func(request execution.ApprovalRequest) error {
		ids <- request.ID
		return nil
	}, nil)
	gateway := approvalGateway(t, approvalScopeSnapshot(), broker)
	go func() {
		select {
		case id := <-ids:
			if err := broker.Resolve(id, true); err != nil {
				t.Errorf("resolve approval: %v", err)
			}
		case <-time.After(5 * time.Second):
		}
	}()
	if approved := decodeApprovalOutput(t, gateway.Call(context.Background(), approvalCallRequest())); !approved {
		t.Fatal("expected approved result")
	}
}

func TestHostApprovalRequestResolvesRejected(t *testing.T) {
	broker := execution.NewApprovalBroker()
	ids := make(chan string, 1)
	broker.AddObservers(func(request execution.ApprovalRequest) error {
		ids <- request.ID
		return nil
	}, nil)
	gateway := approvalGateway(t, approvalScopeSnapshot(), broker)
	go func() {
		select {
		case id := <-ids:
			if err := broker.Resolve(id, false); err != nil {
				t.Errorf("resolve approval: %v", err)
			}
		case <-time.After(5 * time.Second):
		}
	}()
	if approved := decodeApprovalOutput(t, gateway.Call(context.Background(), approvalCallRequest())); approved {
		t.Fatal("expected rejected result")
	}
}

func TestHostApprovalRequestForwardsBrokerFailure(t *testing.T) {
	gateway := approvalGateway(t, approvalScopeSnapshot(), nil)
	result := gateway.Call(context.Background(), approvalCallRequest())
	if result.Status != host_api.StatusFailed || result.Error == nil || result.Error.Code != host_api.ErrorCodeHostUnavailable {
		t.Fatalf("expected host unavailable, got %s: %+v", result.Status, result.Error)
	}
	if result.Error.Message != "approval broker not configured" {
		t.Fatalf("unexpected message %q", result.Error.Message)
	}
}

func TestHostApprovalRequestRequiresConversationScope(t *testing.T) {
	snapshot := approvalScopeSnapshot()
	snapshot.ConversationID = ""
	snapshot.ResolvedScopes = []scope.ScopeRef{
		scope.NewExtensionScope("com.amitia/dsh-compatible-host"),
		scope.NewModuleScope("com.amitia/dsh-compatible-host", "dsh-compatible-runtime"),
	}
	gateway := approvalGateway(t, snapshot, execution.NewApprovalBroker())
	result := gateway.Call(context.Background(), approvalCallRequest())
	if result.Status != host_api.StatusFailed || result.Error == nil || result.Error.Code != host_api.ErrorCodeInputInvalid {
		t.Fatalf("expected input invalid, got %s: %+v", result.Status, result.Error)
	}
}

func TestHostApprovalRequestRejectsEmptyToolName(t *testing.T) {
	gateway := approvalGateway(t, approvalScopeSnapshot(), execution.NewApprovalBroker())
	request := approvalCallRequest()
	request.Input = json.RawMessage(`{"toolName":"  ","conversationId":"conversation-1"}`)
	result := gateway.Call(context.Background(), request)
	if result.Status != host_api.StatusFailed || result.Error == nil || result.Error.Code != host_api.ErrorCodeInputInvalid {
		t.Fatalf("expected input invalid, got %s: %+v", result.Status, result.Error)
	}
}
