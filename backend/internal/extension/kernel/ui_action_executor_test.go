package kernel

import (
	"context"
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
			UserID:      "1",
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
