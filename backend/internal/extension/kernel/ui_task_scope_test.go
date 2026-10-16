package kernel

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/capability"
	"github.com/u-ai/backend/internal/extension/kernel/host_api"
	"github.com/u-ai/backend/internal/extension/kernel/permission"
	"github.com/u-ai/backend/internal/extension/kernel/runtime_supervisor"
	"github.com/u-ai/backend/internal/extension/kernel/scope"
	"github.com/u-ai/backend/internal/extension/kernel/ui_contribution"
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
	definitions.Register(permission.PermissionDefinition{ID: "proactive_task.create"})
	owner := json.RawMessage(`{"spaceId":"space","coreId":"cloud-core","initiatorDeviceId":"phone","targetDeviceId":"phone","roleId":"role","resourceOwnerId":"phone"}`)
	source := scope.ScopeSnapshot{SnapshotID: "ui-owner", ExtensionID: "com.amitia/proactive", ModuleID: "proactive-ui", CharacterID: "role", ConversationID: "conversation", Generation: 2, CreatedAt: time.Now().UTC(), OwnedExecutionScope: owner, ResolvedScopes: []scope.ScopeRef{scope.NewExtensionScope("com.amitia/proactive"), scope.NewModuleScope("com.amitia/proactive", "proactive-ui")}}
	if err := scopes.SaveSnapshot(t.Context(), source); err != nil {
		t.Fatal(err)
	}
	snapshot := permission.NewPermissionSnapshot(permission.PermissionSnapshotRequest{SessionID: "session", ExtensionID: source.ExtensionID, ModuleID: source.ModuleID, CharacterID: source.CharacterID, ConversationID: source.ConversationID, Generation: 2, Lifetime: time.Hour, GrantedPerms: []string{"proactive_task.create"}, ExecutionContext: permission.PermissionExecutionContext{Placement: permission.ExecutionPlacementDevice, SpaceID: "space", DeviceID: "phone", RuntimeID: "session", ExtensionID: source.ExtensionID, ModuleID: source.ModuleID}})
	if err := permissions.SaveSnapshot(t.Context(), snapshot); err != nil {
		t.Fatal(err)
	}
	derive := newUIActionSnapshotDeriver(scopes, permissions, permission.NewPermissionIDValidator(definitions))
	scopeID, permissionID, cleanup, err := derive(t.Context(), source.SnapshotID, snapshot.SnapshotID, source.ExtensionID, "proactive-runtime")
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	derived, err := scopes.GetSnapshot(t.Context(), scopeID)
	if err != nil {
		t.Fatal(err)
	}
	if string(derived.OwnedExecutionScope) != string(owner) || derived.CharacterID != "role" || derived.ConversationID != "conversation" || derived.ModuleID != "proactive-runtime" || derived.ResolvedScopes[1].ModuleID != "proactive-runtime" {
		t.Fatalf("cloud owner lost or reowned during runtime derivation: %#v", derived)
	}
	derivedPermission, err := permissions.GetSnapshot(t.Context(), permissionID)
	if err != nil {
		t.Fatal(err)
	}
	if derivedPermission.DeviceID != "phone" || derivedPermission.SpaceID != "space" || derivedPermission.CharacterID != "role" || derivedPermission.ConversationID != "conversation" || derivedPermission.ModuleID != "proactive-runtime" {
		t.Fatalf("cloud actor lost: %#v", derivedPermission)
	}
	original, err := scopes.GetSnapshot(t.Context(), source.SnapshotID)
	if err != nil || original.ResolvedScopes[1].ModuleID != "proactive-ui" {
		t.Fatalf("source UI snapshot mutated: %#v %v", original, err)
	}
}

func TestUITaskPluginManifestAndBothRenderersReachRuntimeScope(t *testing.T) {
	pluginPath := filepath.Join("..", "..", "..", "..", "plugins", "proactive")
	manifestBytes, err := os.ReadFile(filepath.Join(pluginPath, "amitia-extension.json"))
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		Modules []struct {
			ID            string `json:"id"`
			Contributions []struct {
				Spec struct {
					ToolID      string          `json:"toolId"`
					HandlerName string          `json:"handlerName"`
					InputSchema json.RawMessage `json:"inputSchema"`
				} `json:"spec"`
			} `json:"contributions"`
		} `json:"modules"`
	}
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		t.Fatal(err)
	}
	registry := capability.NewToolRegistry()
	toolID, moduleID := "", ""
	for _, module := range manifest.Modules {
		for _, contribution := range module.Contributions {
			if contribution.Spec.HandlerName != "command" {
				continue
			}
			toolID, moduleID = contribution.Spec.ToolID, module.ID
			if err := registry.Register(t.Context(), capability.ToolDefinition{ID: toolID, ExtensionID: "com.amitia/proactive", ModuleID: moduleID, Enabled: true, InputSchema: contribution.Spec.InputSchema}); err != nil {
				t.Fatal(err)
			}
		}
	}
	if toolID == "" || moduleID != "proactive-runtime" {
		t.Fatalf("runtime command not registered: %s %s", toolID, moduleID)
	}
	inputs := map[string]json.RawMessage{"static-forged-scope": json.RawMessage(`{"action":"settings.update","payload":{"maxPerDay":5,"scope":{"characterId":"forged","conversationId":"forged","spaceId":"forged"}}}`)}
	for _, platform := range []string{"desktop", "mobile"} {
		bytes, err := os.ReadFile(filepath.Join(pluginPath, "ui", "schema", platform+".json"))
		if err != nil {
			t.Fatal(err)
		}
		var schema struct {
			Lifecycle struct {
				OnMount []struct {
					Input json.RawMessage `json:"input"`
				} `json:"onMount"`
			} `json:"lifecycle"`
		}
		if err := json.Unmarshal(bytes, &schema); err != nil {
			t.Fatal(err)
		}
		if len(schema.Lifecycle.OnMount) != 1 {
			t.Fatalf("%s missing dashboard mount", platform)
		}
		inputs[platform+"-schema-mount"] = schema.Lifecycle.OnMount[0].Input
	}
	for name, input := range inputs {
		t.Run(name, func(t *testing.T) {
			gateway := host_api.NewDefaultGateway()
			gateway.SetPermissionChecker(host_api.PermissionCheckerFunc(func(_ context.Context, identity runtime_supervisor.RuntimeIdentity, _ []host_api.PermissionRequirement) error {
				if string(identity.ModuleID) != moduleID {
					t.Fatalf("permission identity remained UI module: %#v", identity)
				}
				return nil
			}))
			gateway.SetScopeChecker(host_api.ScopeCheckerFunc(func(_ context.Context, identity runtime_supervisor.RuntimeIdentity, snapshot string, _ host_api.ScopePolicy) error {
				if string(identity.ModuleID) != moduleID || snapshot != "runtime-owned-scope" {
					t.Fatalf("wrong runtime scope: %#v %s", identity, snapshot)
				}
				return nil
			}))
			called, cleaned := false, false
			if err := gateway.RegisterRoute(host_api.Route{Method: host_api.MethodToolExecute, Version: 1, Handler: func(_ context.Context, request host_api.CallRequest) (host_api.CallResult, error) {
				called = true
				var command struct {
					ToolID string `json:"toolId"`
					Input  struct {
						Action  string `json:"action"`
						Payload struct {
							Scope     map[string]string `json:"scope"`
							MaxPerDay int               `json:"maxPerDay"`
						} `json:"payload"`
					} `json:"input"`
				}
				if err := json.Unmarshal(request.Input, &command); err != nil {
					t.Fatal(err)
				}
				if command.ToolID != toolID || command.Input.Payload.Scope["characterId"] != "owned-role" || command.Input.Payload.Scope["conversationId"] != "owned-conversation" || len(command.Input.Payload.Scope) != 2 {
					t.Fatalf("renderer scope was trusted: %s", request.Input)
				}
				if name == "static-forged-scope" && command.Input.Payload.MaxPerDay != 5 {
					t.Fatalf("settings lost during injection: %s", request.Input)
				}
				if request.PermissionSnapshotID != "runtime-permission" || request.ExecutionContext.ModuleID != moduleID {
					t.Fatalf("permission snapshot/actor not derived: %#v", request)
				}
				return host_api.CallResult{Status: host_api.StatusSuccess, Output: json.RawMessage(`{"saved":true}`)}, nil
			}}); err != nil {
				t.Fatal(err)
			}
			executor := NewUIActionExecutor(gateway, nil, nil, nil, nil, registry, nil, func(_ context.Context, sourceScope, sourcePermission, extension, target string) (string, string, func(), error) {
				if sourceScope != "ui-owned-scope" || sourcePermission != "ui-permission" || extension != "com.amitia/proactive" || target != moduleID {
					t.Fatalf("incorrect module derivation: %s %s %s %s", sourceScope, sourcePermission, extension, target)
				}
				return "runtime-owned-scope", "runtime-permission", func() { cleaned = true }, nil
			})
			output, err := executor.Execute(t.Context(), UIActionExecContext{SessionID: "session", ExtensionID: "com.amitia/proactive", ModuleID: "proactive-ui", Generation: 1, ScopeSnapshotID: "ui-owned-scope", PermissionSnapshotID: "ui-permission", CharacterID: "owned-role", ConversationID: "owned-conversation", SpaceID: "space", DeviceID: "device"}, &ui_contribution.UIActionDefinition{ActionID: "command", Target: ui_contribution.UIActionTarget{Type: ui_contribution.ActionTargetTool, ToolID: toolID}}, input)
			if err != nil || !called || !cleaned || string(output) != `{"saved":true}` {
				t.Fatalf("UI runtime chain incomplete: called=%v cleaned=%v result=%s err=%v", called, cleaned, output, err)
			}
		})
	}
}
