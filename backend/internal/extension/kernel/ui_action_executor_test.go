package kernel

import (
	"context"
	"testing"

	"github.com/u-ai/backend/internal/extension/kernel/capability"
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
