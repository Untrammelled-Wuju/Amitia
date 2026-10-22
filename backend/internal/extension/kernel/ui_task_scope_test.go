package kernel

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/capability"
	"github.com/u-ai/backend/internal/extension/kernel/permission"
	"github.com/u-ai/backend/internal/extension/kernel/scope"
)

func TestUITaskScopeComesFromSession(t *testing.T) {
	registry := capability.NewToolRegistry()
	if err := registry.Register(t.Context(), capability.ToolDefinition{ID: "task-command", ExtensionID: "plugin", Enabled: true, InputSchema: json.RawMessage(`{"type":"object","properties":{"payload":{"type":"object","properties":{"scope":{"type":"object"}}}}}`)}); err != nil {
		t.Fatal(err)
	}
	executor := &UIActionExecutor{toolRegistry: registry}
	input := json.RawMessage(`{"action":"settings.update","payload":{"maxPerDay":5,"scope":{"characterId":"forged","spaceId":"forged"}}}`)
	result, err := executor.injectToolUIScope(t.Context(), "task-command", input, UIActionExecContext{CharacterID: "role", ConversationID: "conversation"})
	if err != nil {
		t.Fatal(err)
	}
	var decoded struct {
		Payload struct {
			MaxPerDay int               `json:"maxPerDay"`
			Scope     map[string]string `json:"scope"`
		} `json:"payload"`
	}
	if err := json.Unmarshal(result, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Payload.MaxPerDay != 5 || decoded.Payload.Scope["characterId"] != "role" || decoded.Payload.Scope["conversationId"] != "conversation" || decoded.Payload.Scope["spaceId"] != "" {
		t.Fatalf("untrusted UI context survived: %s", result)
	}
	unchanged, err := executor.injectToolUIScope(t.Context(), "other-tool", input, UIActionExecContext{CharacterID: "role"})
	if err != nil || string(unchanged) != string(input) {
		t.Fatalf("unrelated tool input modified: %s %v", unchanged, err)
	}
	result, err = executor.injectToolUIScope(t.Context(), "task-command", json.RawMessage(`{"action":"ui.dashboard"}`), UIActionExecContext{CharacterID: "role"})
	if err != nil || json.Unmarshal(result, &decoded) != nil || decoded.Payload.Scope["characterId"] != "role" {
		t.Fatalf("schema dashboard omitted role: %s %v", result, err)
	}
}

func TestUITaskDerivedSnapshotRetainsCloudOwner(t *testing.T) {
	scopes := scope.NewMemoryScopeStore()
	permissions := permission.NewMemoryPermissionSnapshotStore()
	definitions := permission.NewPermissionDefinitionRegistry()
	definitions.Register(permission.PermissionDefinition{ID: "active_task.create"})
	owner := json.RawMessage(`{"spaceId":"space","coreId":"cloud-core","initiatorDeviceId":"phone","targetDeviceId":"phone","roleId":"role","resourceOwnerId":"phone"}`)
	source := scope.ScopeSnapshot{SnapshotID: "ui-owner", ExtensionID: "com.amitia.builtin.active-task", ModuleID: "active-task-ui", CharacterID: "role", ConversationID: "conversation", Generation: 2, CreatedAt: time.Now().UTC(), OwnedExecutionScope: owner, ResolvedScopes: []scope.ScopeRef{scope.NewExtensionScope("com.amitia.builtin.active-task"), scope.NewModuleScope("com.amitia.builtin.active-task", "active-task-ui")}}
	if err := scopes.SaveSnapshot(t.Context(), source); err != nil {
		t.Fatal(err)
	}
	snapshot := permission.NewPermissionSnapshot(permission.PermissionSnapshotRequest{SessionID: "session", ExtensionID: source.ExtensionID, ModuleID: source.ModuleID, CharacterID: source.CharacterID, ConversationID: source.ConversationID, Generation: 2, Lifetime: time.Hour, GrantedPerms: []string{"active_task.create"}, ExecutionContext: permission.PermissionExecutionContext{Placement: permission.ExecutionPlacementDevice, SpaceID: "space", DeviceID: "phone", RuntimeID: "session", ExtensionID: source.ExtensionID, ModuleID: source.ModuleID}})
	if err := permissions.SaveSnapshot(t.Context(), snapshot); err != nil {
		t.Fatal(err)
	}
	derive := newUIActionSnapshotDeriver(scopes, permissions, permission.NewPermissionIDValidator(definitions))
	scopeID, permissionID, cleanup, err := derive(t.Context(), source.SnapshotID, snapshot.SnapshotID, source.ExtensionID, "active-task-runtime")
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	derived, err := scopes.GetSnapshot(t.Context(), scopeID)
	if err != nil {
		t.Fatal(err)
	}
	if string(derived.OwnedExecutionScope) != string(owner) || derived.CharacterID != "role" || derived.ConversationID != "conversation" || derived.ModuleID != "active-task-runtime" || derived.ResolvedScopes[1].ModuleID != "active-task-runtime" {
		t.Fatalf("cloud owner lost or reowned during runtime derivation: %#v", derived)
	}
	derivedPermission, err := permissions.GetSnapshot(t.Context(), permissionID)
	if err != nil {
		t.Fatal(err)
	}
	if derivedPermission.DeviceID != "phone" || derivedPermission.SpaceID != "space" || derivedPermission.CharacterID != "role" || derivedPermission.ConversationID != "conversation" || derivedPermission.ModuleID != "active-task-runtime" {
		t.Fatalf("cloud actor lost: %#v", derivedPermission)
	}
	original, err := scopes.GetSnapshot(t.Context(), source.SnapshotID)
	if err != nil || original.ResolvedScopes[1].ModuleID != "active-task-ui" {
		t.Fatalf("source UI snapshot mutated: %#v %v", original, err)
	}
}
