package kernel

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/u-ai/backend/internal/extension/kernel/domain"
	"github.com/u-ai/backend/internal/extension/kernel/host_api"
	"github.com/u-ai/backend/internal/extension/kernel/runtime_supervisor"
	"github.com/u-ai/backend/internal/extension/kernel/scope"
)

type fixedConversationScopeStore struct {
	snapshot *scope.ScopeSnapshot
}

func (s *fixedConversationScopeStore) Get(context.Context, string) (*scope.ScopeSnapshot, error) {
	return s.snapshot, nil
}

type recordingConversationMessageSender struct {
	request ConversationMessageRequest
	result  ConversationMessageResult
	err     error
	called  bool
}

type recordingConversationMessageAppender struct {
	request ConversationMessageAppendRequest
	result  ConversationMessageAppendResult
	err     error
	called  bool
}

func (a *recordingConversationMessageAppender) AppendConversationMessages(_ context.Context, request ConversationMessageAppendRequest) (ConversationMessageAppendResult, error) {
	a.called = true
	a.request = request
	return a.result, a.err
}

func (s *recordingConversationMessageSender) SendConversationMessage(_ context.Context, request ConversationMessageRequest) (ConversationMessageResult, error) {
	s.called = true
	s.request = request
	return s.result, s.err
}

func newConversationMessageTestGateway() *host_api.DefaultGateway {
	gateway := host_api.NewDefaultGateway()
	gateway.SetPermissionChecker(host_api.PermissionCheckerFunc(func(context.Context, runtime_supervisor.RuntimeIdentity, []host_api.PermissionRequirement) error {
		return nil
	}))
	gateway.SetScopeChecker(host_api.ScopeCheckerFunc(func(context.Context, runtime_supervisor.RuntimeIdentity, string, host_api.ScopePolicy) error {
		return nil
	}))
	return gateway
}

func TestConversationMessageRouteUsesSharedPermission(t *testing.T) {
	permissions := host_api.RoutePermissionForMethod(host_api.MethodConversationMessageSend)
	if len(permissions) != 1 {
		t.Fatalf("expected one permission, got %d", len(permissions))
	}
	if permissions[0].Name != "message.send" {
		t.Fatalf("expected message.send permission, got %s", permissions[0].Name)
	}
	if permissions[0].Resource != "conversation" {
		t.Fatalf("expected conversation resource, got %s", permissions[0].Resource)
	}
	policy := host_api.RouteScopeForMethod(host_api.MethodConversationMessageSend)
	if !policy.Namespaced {
		t.Fatal("expected namespaced scope policy")
	}
}

func TestConversationMessageRouteSendsWithResolvedScope(t *testing.T) {
	sender := &recordingConversationMessageSender{
		result: ConversationMessageResult{Content: "角色回复"},
	}
	gateway := newConversationMessageTestGateway()
	if err := setupDefaultHostAPIRoutes(gateway, HostAPIRouteDeps{
		ScopeSnapshotStore: &fixedConversationScopeStore{
			snapshot: &scope.ScopeSnapshot{
				SnapshotID:     "scope-1",
				SpaceID:        "user-1",
				CharacterID:    "char-1",
				ConversationID: "conv-1",
			},
		},
		ConversationMessageSender: sender,
	}); err != nil {
		t.Fatalf("setup routes: %v", err)
	}
	result := gateway.Call(context.Background(), host_api.CallRequest{
		CallID:          "call-message",
		RuntimeIdentity: runtime_supervisor.RuntimeIdentity{InstanceID: "runtime-1", ExtensionID: domain.ExtensionID("com.amitia/proactive"), ModuleID: "proactive-runtime"},
		Method:          host_api.MethodConversationMessageSend,
		Version:         1,
		Input:           json.RawMessage(`{"channel":"web","content":"  你好  ","requestId":"req-1"}`),
		ScopeSnapshotID: "scope-1",
	})
	if result.Status != host_api.StatusSuccess {
		t.Fatalf("expected success, got %s: %+v", result.Status, result.Error)
	}
	if !sender.called {
		t.Fatal("expected conversation message sender to be called")
	}
	if sender.request.SpaceID != "user-1" || sender.request.CharacterID != "char-1" || sender.request.ConversationID != "conv-1" {
		t.Fatalf("expected scope target, got %+v", sender.request)
	}
	if sender.request.Content != "你好" {
		t.Fatalf("expected trimmed content, got %q", sender.request.Content)
	}
	if sender.request.RequestID != "req-1" {
		t.Fatalf("expected request id req-1, got %q", sender.request.RequestID)
	}
	var output map[string]any
	if err := json.Unmarshal(result.Output, &output); err != nil {
		t.Fatalf("decode output: %v", err)
	}
	if output["content"] != "角色回复" {
		t.Fatalf("expected output content, got %v", output["content"])
	}
}

func TestConversationMessageRouteRejectsScopeMismatch(t *testing.T) {
	sender := &recordingConversationMessageSender{}
	gateway := newConversationMessageTestGateway()
	if err := setupDefaultHostAPIRoutes(gateway, HostAPIRouteDeps{
		ScopeSnapshotStore: &fixedConversationScopeStore{
			snapshot: &scope.ScopeSnapshot{
				SnapshotID:     "scope-1",
				SpaceID:        "user-1",
				CharacterID:    "char-1",
				ConversationID: "conv-1",
			},
		},
		ConversationMessageSender: sender,
	}); err != nil {
		t.Fatalf("setup routes: %v", err)
	}
	result := gateway.Call(context.Background(), host_api.CallRequest{
		CallID:          "call-message-mismatch",
		RuntimeIdentity: runtime_supervisor.RuntimeIdentity{InstanceID: "runtime-1", ExtensionID: domain.ExtensionID("com.amitia/proactive"), ModuleID: "proactive-runtime"},
		Method:          host_api.MethodConversationMessageSend,
		Version:         1,
		Input:           json.RawMessage(`{"conversationId":"conv-other","content":"你好"}`),
		ScopeSnapshotID: "scope-1",
	})
	if result.Status != host_api.StatusRejected {
		t.Fatalf("expected rejected, got %s: %+v", result.Status, result.Error)
	}
	if result.Error == nil || result.Error.Code != host_api.ErrorCodeScopeDenied {
		t.Fatalf("expected scope denied, got %+v", result.Error)
	}
	if sender.called {
		t.Fatal("sender must not be called on scope mismatch")
	}
}

func TestLegacyProactiveDispatchMethodIsNotRegistered(t *testing.T) {
	gateway := newConversationMessageTestGateway()
	if err := setupDefaultHostAPIRoutes(gateway, HostAPIRouteDeps{}); err != nil {
		t.Fatalf("setup routes: %v", err)
	}
	result := gateway.Call(context.Background(), host_api.CallRequest{
		CallID:          "call-legacy",
		RuntimeIdentity: runtime_supervisor.RuntimeIdentity{InstanceID: "runtime-1", ExtensionID: domain.ExtensionID("com.amitia/proactive"), ModuleID: "proactive-runtime"},
		Method:          host_api.Method("host.proactive.dispatch"),
		Version:         1,
		Input:           json.RawMessage(`{}`),
	})
	if result.Error == nil || result.Error.Code != host_api.ErrorCodeMethodNotFound {
		t.Fatalf("expected method_not_found, got %+v", result.Error)
	}
}

func TestConversationMessageAppendRouteUsesResolvedScope(t *testing.T) {
	appender := &recordingConversationMessageAppender{
		result: ConversationMessageAppendResult{
			MessageIDs:      []string{"message-1"},
			Sequences:       []int64{2},
			ResponseGroupID: "append-1",
			LastSequence:    2,
		},
	}
	gateway := newConversationMessageTestGateway()
	if err := setupDefaultHostAPIRoutes(gateway, HostAPIRouteDeps{
		ScopeSnapshotStore: &fixedConversationScopeStore{
			snapshot: &scope.ScopeSnapshot{
				SnapshotID:     "scope-1",
				SpaceID:        "user-1",
				CharacterID:    "char-1",
				ConversationID: "conv-1",
			},
		},
		ConversationMessageAppender: appender,
	}); err != nil {
		t.Fatalf("setup routes: %v", err)
	}
	result := gateway.Call(context.Background(), host_api.CallRequest{
		CallID:          "call-append",
		RuntimeIdentity: runtime_supervisor.RuntimeIdentity{InstanceID: "runtime-1", ExtensionID: domain.ExtensionID("com.example/media"), ModuleID: "runtime"},
		Method:          host_api.MethodConversationMessageAppend,
		Version:         1,
		Input:           json.RawMessage(`{"channel":"web","role":"user","source":"extension:com.example/media","requestId":"append-1","parts":[{"type":"image","url":"/api/extension/resources/image","altText":"图片"}]}`),
		ScopeSnapshotID: "scope-1",
	})
	if result.Status != host_api.StatusSuccess {
		t.Fatalf("expected success, got %s: %+v", result.Status, result.Error)
	}
	if !appender.called {
		t.Fatal("expected appender to be called")
	}
	if appender.request.SpaceID != "user-1" || appender.request.CharacterID != "char-1" || appender.request.ConversationID != "conv-1" {
		t.Fatalf("expected resolved scope, got %+v", appender.request)
	}
	if len(appender.request.Parts) != 1 || appender.request.Parts[0].Type != "image" {
		t.Fatalf("unexpected parts: %+v", appender.request.Parts)
	}
}
