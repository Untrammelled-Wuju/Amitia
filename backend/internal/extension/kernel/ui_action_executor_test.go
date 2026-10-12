package kernel

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/capability"
	"github.com/u-ai/backend/internal/extension/kernel/permission"
	"github.com/u-ai/backend/internal/extension/kernel/scope"
)

func TestUIActionExecutorResolvesNamespacedGameHostTool(t *testing.T) {
	ctx := context.Background()
	registry := capability.NewToolRegistry()
	if err := registry.Register(ctx, capability.ToolDefinition{
		ID:          "com.amitiax/minecraft/ui-connect",
		ExtensionID: "com.amitiax/minecraft",
		Name:        "Minecraft UI Connect",
		Enabled:     true,
	}); err != nil {
		t.Fatal(err)
	}
	executor := &UIActionExecutor{toolRegistry: registry}
	if got := executor.resolveToolID(ctx, "com.amitiax/minecraft", "ui-connect"); got != "com.amitiax/minecraft/ui-connect" {
		t.Fatalf("resolved tool id = %q", got)
	}
}

func TestUIActionExecutorDoesNotUseAnotherExtensionsTool(t *testing.T) {
	ctx := context.Background()
	registry := capability.NewToolRegistry()
	if err := registry.Register(ctx, capability.ToolDefinition{
		ID:          "com.example/other/ui-connect",
		ExtensionID: "com.example/other",
		Name:        "Other UI Connect",
		Enabled:     true,
	}); err != nil {
		t.Fatal(err)
	}
	executor := &UIActionExecutor{toolRegistry: registry}
	if got := executor.resolveToolID(ctx, "com.amitiax/minecraft", "ui-connect"); got != "ui-connect" {
		t.Fatalf("resolved tool id = %q", got)
	}
}

func TestUIActionExecutorEnsuresCurrentExtensionsToolScope(t *testing.T) {
	ctx := context.Background()
	registry := capability.NewToolRegistry()
	if err := registry.Register(ctx, capability.ToolDefinition{
		ID:          "com.amitiax/minecraft/ui-connect",
		ExtensionID: "com.amitiax/minecraft",
		Name:        "Minecraft UI Connect",
		Enabled:     true,
	}); err != nil {
		t.Fatal(err)
	}
	store := scope.NewMemoryScopeStore()
	manager := scope.NewScopeManager(store, scope.NewScopeEvaluator(store, nil))
	executor := &UIActionExecutor{toolRegistry: registry, scopeManager: manager}
	toolID := executor.resolveToolID(ctx, "com.amitiax/minecraft", "ui-connect")
	if err := executor.ensureToolScope(ctx, "com.amitiax/minecraft", toolID); err != nil {
		t.Fatal(err)
	}
	decision := manager.Evaluate(ctx, scope.ScopeEvaluationRequest{
		SubjectType: scope.SubjectTool,
		SubjectID:   toolID,
		ExtensionID: "com.amitiax/minecraft",
	})
	if !decision.Allowed {
		t.Fatalf("expected tool scope to be allowed, got %+v", decision.Reasons)
	}
}

func TestUIActionExecutorResolvesToolOwningModuleWithinExtension(t *testing.T) {
	ctx := context.Background()
	registry := capability.NewToolRegistry()
	if err := registry.Register(ctx, capability.ToolDefinition{
		ID:          "com.amitia/emote/command",
		ExtensionID: "com.amitia/emote",
		ModuleID:    "emote-runtime",
		Name:        "Emote Command",
		Enabled:     true,
	}); err != nil {
		t.Fatal(err)
	}
	executor := &UIActionExecutor{toolRegistry: registry}
	if got := executor.resolveToolModule(ctx, "com.amitia/emote", "com.amitia/emote/command", "emote-ui"); got != "emote-runtime" {
		t.Fatalf("resolved module = %q", got)
	}
}

func TestUIActionSnapshotDeriverSwitchesModuleWithinExtension(t *testing.T) {
	ctx := context.Background()
	scopeStore := scope.NewMemoryScopeStore()
	permissionStore := permission.NewMemoryPermissionSnapshotStore()
	registry := permission.NewPermissionDefinitionRegistry()
	registry.Register(permission.PermissionDefinition{ID: "tool.invoke"})
	validator := permission.NewPermissionIDValidator(registry)
	sourceScope := scope.ScopeSnapshot{
		SnapshotID: "scope-source",
		ResolvedScopes: []scope.ScopeRef{
			scope.NewExtensionScope("com.amitia/emote"),
			scope.NewModuleScope("com.amitia/emote", "emote-ui"),
		},
		ExtensionID: "com.amitia/emote",
		ModuleID:    "emote-ui",
		Generation:  2,
		CreatedAt:   time.Now().UTC(),
	}
	if err := scopeStore.SaveSnapshot(ctx, sourceScope); err != nil {
		t.Fatal(err)
	}
	expiresAt := time.Now().UTC().Add(time.Hour)
	sourcePermission := permission.NewPermissionSnapshot(permission.PermissionSnapshotRequest{
		SessionID:    "session-1",
		ExtensionID:  "com.amitia/emote",
		ModuleID:     "emote-ui",
		Generation:   2,
		GrantedPerms: []string{"tool.invoke"},
		Lifetime:     time.Hour,
		ExecutionContext: permission.PermissionExecutionContext{
			Placement:   permission.ExecutionPlacementDevice,
			SpaceID:     "1",
			DeviceID:    "device-1",
			RuntimeID:   "runtime-1",
			ExtensionID: "com.amitia/emote",
			ModuleID:    "emote-ui",
			Source:      "ui_action",
		},
	})
	sourcePermission.ExpiresAt = &expiresAt
	if err := permissionStore.SaveSnapshot(ctx, sourcePermission); err != nil {
		t.Fatal(err)
	}
	derive := newUIActionSnapshotDeriver(scopeStore, permissionStore, validator)
	scopeID, permissionID, cleanup, err := derive(ctx, sourceScope.SnapshotID, sourcePermission.SnapshotID, "com.amitia/emote", "emote-runtime")
	if err != nil {
		t.Fatal(err)
	}
	derivedPermission, err := permissionStore.GetSnapshot(ctx, permissionID)
	if err != nil {
		t.Fatal(err)
	}
	if derivedPermission.ModuleID != "emote-runtime" {
		t.Fatalf("derived permission module = %q", derivedPermission.ModuleID)
	}
	derivedScope, err := scopeStore.GetSnapshot(ctx, scopeID)
	if err != nil {
		t.Fatal(err)
	}
	if derivedScope.ModuleID != "emote-runtime" {
		t.Fatalf("derived scope module = %q", derivedScope.ModuleID)
	}
	cleanup()
	if _, err := permissionStore.GetSnapshot(ctx, permissionID); err == nil {
		t.Fatal("derived permission snapshot was not cleaned up")
	}
}

func TestUIActionToolInputDropsUndeclaredRendererMeta(t *testing.T) {
	ctx := context.Background()
	registry := capability.NewToolRegistry()
	if err := registry.Register(ctx, capability.ToolDefinition{
		ID:          "com.amitia/proactive/command",
		ExtensionID: "com.amitia/proactive",
		Name:        "Proactive Command",
		Enabled:     true,
		InputSchema: json.RawMessage(`{"type":"object","properties":{"action":{"type":"string"},"payload":{"type":"object"}},"additionalProperties":false}`),
	}); err != nil {
		t.Fatal(err)
	}
	executor := &UIActionExecutor{toolRegistry: registry}

	got, err := executor.normalizeToolInput(
		ctx,
		"com.amitia/proactive/command",
		json.RawMessage(`{"action":"ui.dashboard","node_id":"lifecycle:command","form_state":{"enabled":true}}`),
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}

	var payload map[string]json.RawMessage
	if err := json.Unmarshal(got, &payload); err != nil {
		t.Fatal(err)
	}
	if string(payload["action"]) != `"ui.dashboard"` {
		t.Fatalf("expected action to survive, got %s", got)
	}
	if _, exists := payload["node_id"]; exists {
		t.Fatalf("undeclared node_id should be stripped, got %s", got)
	}
	if _, exists := payload["form_state"]; exists {
		t.Fatalf("undeclared form_state should be stripped, got %s", got)
	}
}

func TestUIActionToolInputMergesDeclaredRendererMetaFromMetaChannel(t *testing.T) {
	ctx := context.Background()
	registry := capability.NewToolRegistry()
	if err := registry.Register(ctx, capability.ToolDefinition{
		ID:          "com.amitia/emote/command",
		ExtensionID: "com.amitia/emote",
		Name:        "Emote Command",
		Enabled:     true,
		InputSchema: json.RawMessage(`{"type":"object","properties":{"action":{"type":"string"},"payload":{"type":"object"},"node_id":{"type":"string"},"form_state":{"type":"object"}},"additionalProperties":false}`),
	}); err != nil {
		t.Fatal(err)
	}
	executor := &UIActionExecutor{toolRegistry: registry}

	got, err := executor.normalizeToolInput(
		ctx,
		"com.amitia/emote/command",
		json.RawMessage(`{"action":"ui.composer"}`),
		json.RawMessage(`{"node_id":"lifecycle:command","form_state":{"category":"sticker"},"local_state":{"ignored":true}}`),
	)
	if err != nil {
		t.Fatal(err)
	}

	var payload map[string]json.RawMessage
	if err := json.Unmarshal(got, &payload); err != nil {
		t.Fatal(err)
	}
	if string(payload["node_id"]) != `"lifecycle:command"` {
		t.Fatalf("declared node_id should be merged from meta, got %s", got)
	}
	if string(payload["form_state"]) != `{"category":"sticker"}` {
		t.Fatalf("declared form_state should be merged from meta, got %s", got)
	}
	if _, exists := payload["local_state"]; exists {
		t.Fatalf("undeclared local_state should stay out of tool input, got %s", got)
	}
}

func TestUIActionToolInputKeepsMetaWhenSchemaUnavailable(t *testing.T) {
	ctx := context.Background()
	executor := &UIActionExecutor{}
	input := json.RawMessage(`{"action":"ui.dashboard","node_id":"lifecycle:command","form_state":{}}`)

	got, err := executor.normalizeToolInput(ctx, "com.amitia/unknown/command", input, nil)
	if err != nil {
		t.Fatal(err)
	}
	var payload map[string]json.RawMessage
	if err := json.Unmarshal(got, &payload); err != nil {
		t.Fatal(err)
	}
	if len(payload) != 3 || string(payload["node_id"]) != `"lifecycle:command"` || string(payload["action"]) != `"ui.dashboard"` {
		t.Fatalf("input should pass through when no schema is known, got %s", got)
	}
}
