package kernel

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/u-ai/backend/internal/extension/kernel/domain"
	"github.com/u-ai/backend/internal/extension/kernel/host_api"
	"github.com/u-ai/backend/internal/extension/kernel/runtime_supervisor"
	"github.com/u-ai/backend/internal/extension/kernel/scope"
)

type recordingConversationCreator struct {
	request ConversationCreateRequest
	output  json.RawMessage
	err     error
	called  bool
}

type recordingConversationDeleter struct {
	request ConversationDeleteRequest
	output  json.RawMessage
	err     error
	called  bool
}

type recordingConversationLister struct {
	limit  int
	offset int
	output any
	err    error
	called bool
}

func (r *recordingConversationLister) ListConversations(_ context.Context, limit int, offset int, archivedOnly bool) (any, error) {
	r.called = true
	r.limit, r.offset = limit, offset
	return r.output, r.err
}

func (r *recordingConversationDeleter) DeleteConversation(_ context.Context, request ConversationDeleteRequest) (json.RawMessage, error) {
	r.called = true
	r.request = request
	return r.output, r.err
}

func (r *recordingConversationCreator) CreateConversation(_ context.Context, request ConversationCreateRequest) (json.RawMessage, error) {
	r.called = true
	r.request = request
	return r.output, r.err
}

func TestConversationCreateRouteUsesResolvedOwnerScope(t *testing.T) {
	permissions := host_api.RoutePermissionForMethod(host_api.MethodConversationCreate)
	if len(permissions) != 1 || permissions[0].Name != "conversation.manage" {
		t.Fatalf("unexpected permissions: %+v", permissions)
	}
	creator := &recordingConversationCreator{output: json.RawMessage(`{"conversationId":"conversation-new","title":"New chat"}`)}
	gateway := newConversationMessageTestGateway()
	targetStore := scope.NewMemoryScopeStore()
	if err := setupDefaultHostAPIRoutes(gateway, HostAPIRouteDeps{
		ScopeSnapshotStore: &fixedConversationScopeStore{snapshot: &scope.ScopeSnapshot{
			SnapshotID:     "scope-create",
			SpaceID:        "space-1",
			CharacterID:    "character-1",
			ConversationID: "conversation-current",
		}},
		ConversationCreator: creator,
		ScopeSnapshotWriter: targetStore,
	}); err != nil {
		t.Fatal(err)
	}
	result := gateway.Call(context.Background(), host_api.CallRequest{
		CallID:          "call-create",
		RuntimeIdentity: runtime_supervisor.RuntimeIdentity{InstanceID: "runtime-1", ExtensionID: domain.ExtensionID("com.amitia/dsh-compatible-host"), ModuleID: "dsh-compatible-runtime"},
		Method:          host_api.MethodConversationCreate,
		Version:         1,
		Input:           json.RawMessage(`{"title":"New chat","conversationId":"dsh-session-reserved","requestId":"create-1","channel":"web"}`),
		ScopeSnapshotID: "scope-create",
	})
	if result.Status != host_api.StatusSuccess {
		t.Fatalf("expected success, got %s: %+v", result.Status, result.Error)
	}
	if !creator.called {
		t.Fatal("creator was not called")
	}
	if creator.request != (ConversationCreateRequest{SpaceID: "space-1", CharacterID: "character-1", ConversationID: "dsh-session-reserved", RequestID: "create-1", Title: "New chat", Channel: "web"}) {
		t.Fatalf("unexpected create request: %+v", creator.request)
	}
	var output struct {
		ConversationID  string `json:"conversationId"`
		Title           string `json:"title"`
		ScopeSnapshotID string `json:"scopeSnapshotId"`
	}
	if err := json.Unmarshal(result.Output, &output); err != nil {
		t.Fatal(err)
	}
	if output.ConversationID != "conversation-new" || output.Title != "New chat" || output.ScopeSnapshotID == "" {
		t.Fatalf("unexpected output: %s", result.Output)
	}
	target, err := targetStore.GetSnapshot(context.Background(), output.ScopeSnapshotID)
	if err != nil {
		t.Fatal(err)
	}
	if target.SpaceID != "space-1" || target.CharacterID != "character-1" || target.ConversationID != "conversation-new" || target.ExtensionID != "com.amitia/dsh-compatible-host" || target.ModuleID != "dsh-compatible-runtime" {
		t.Fatalf("unexpected target snapshot: %+v", target)
	}
}

func TestConversationCreateRoutePreservesCreatorError(t *testing.T) {
	original := errors.New("conversation create fixture failed")
	creator := &recordingConversationCreator{err: original}
	gateway := newConversationMessageTestGateway()
	if err := setupDefaultHostAPIRoutes(gateway, HostAPIRouteDeps{
		ScopeSnapshotStore: &fixedConversationScopeStore{snapshot: &scope.ScopeSnapshot{
			SnapshotID:  "scope-create-error",
			SpaceID:     "space-1",
			CharacterID: "character-1",
		}},
		ConversationCreator: creator,
	}); err != nil {
		t.Fatal(err)
	}
	result := gateway.Call(context.Background(), host_api.CallRequest{
		CallID:          "call-create-error",
		RuntimeIdentity: runtime_supervisor.RuntimeIdentity{InstanceID: "runtime-1", ExtensionID: domain.ExtensionID("com.amitia/dsh-compatible-host"), ModuleID: "dsh-compatible-runtime"},
		Method:          host_api.MethodConversationCreate,
		Version:         1,
		Input:           json.RawMessage(`{"requestId":"create-error"}`),
		ScopeSnapshotID: "scope-create-error",
	})
	if result.Status != host_api.StatusFailed || result.Error == nil || result.Error.Message != original.Error() {
		t.Fatalf("expected original failure, got %s: %+v", result.Status, result.Error)
	}
}

func TestConversationListRouteUsesBoundedCatalogQuery(t *testing.T) {
	permissions := host_api.RoutePermissionForMethod(host_api.MethodConversationList)
	if len(permissions) != 1 || permissions[0].Name != "conversation.read" {
		t.Fatalf("unexpected permissions: %+v", permissions)
	}
	lister := &recordingConversationLister{output: map[string]any{"items": []any{}, "hasMore": false}}
	gateway := newConversationMessageTestGateway()
	if err := setupDefaultHostAPIRoutes(gateway, HostAPIRouteDeps{ConversationLister: lister}); err != nil {
		t.Fatal(err)
	}
	result := gateway.Call(context.Background(), host_api.CallRequest{
		CallID:          "call-list",
		RuntimeIdentity: runtime_supervisor.RuntimeIdentity{InstanceID: "runtime-1", ExtensionID: domain.ExtensionID("com.amitia/dsh-compatible-host"), ModuleID: "dsh-compatible-runtime"},
		Method:          host_api.MethodConversationList,
		Version:         1,
		Input:           json.RawMessage(`{"limit":500,"offset":12}`),
	})
	if result.Status != host_api.StatusSuccess {
		t.Fatalf("expected success, got %s: %+v", result.Status, result.Error)
	}
	if !lister.called || lister.limit != 128 || lister.offset != 12 {
		t.Fatalf("unexpected list request: %+v", lister)
	}
}

func TestConversationDeleteRouteUsesExactOwnerConversation(t *testing.T) {
	permissions := host_api.RoutePermissionForMethod(host_api.MethodConversationDelete)
	if len(permissions) != 1 || permissions[0].Name != "conversation.manage" {
		t.Fatalf("unexpected permissions: %+v", permissions)
	}
	deleter := &recordingConversationDeleter{output: json.RawMessage(`{"deleted":true,"conversationId":"conversation-created"}`)}
	gateway := newConversationMessageTestGateway()
	if err := setupDefaultHostAPIRoutes(gateway, HostAPIRouteDeps{ConversationDeleter: deleter}); err != nil {
		t.Fatal(err)
	}
	result := gateway.Call(context.Background(), host_api.CallRequest{
		CallID:          "call-delete",
		RuntimeIdentity: runtime_supervisor.RuntimeIdentity{InstanceID: "runtime-1", ExtensionID: domain.ExtensionID("com.amitia/dsh-compatible-host"), ModuleID: "dsh-compatible-runtime"},
		Method:          host_api.MethodConversationDelete,
		Version:         1,
		Input:           json.RawMessage(`{"spaceId":"space-1","characterId":"character-1","conversationId":"conversation-created","requestId":"delete-1"}`),
	})
	if result.Status != host_api.StatusSuccess {
		t.Fatalf("expected success, got %s: %+v", result.Status, result.Error)
	}
	if !deleter.called {
		t.Fatal("deleter was not called")
	}
	if deleter.request != (ConversationDeleteRequest{SpaceID: "space-1", CharacterID: "character-1", ConversationID: "conversation-created", RequestID: "delete-1"}) {
		t.Fatalf("unexpected delete request: %+v", deleter.request)
	}
}
