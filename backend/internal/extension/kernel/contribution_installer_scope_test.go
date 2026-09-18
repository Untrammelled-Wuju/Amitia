package kernel

import (
	"context"
	"testing"

	"github.com/u-ai/backend/internal/extension/kernel/capability"
	"github.com/u-ai/backend/internal/extension/kernel/domain"
	"github.com/u-ai/backend/internal/extension/kernel/scope"
)

func TestActivateToolCreatesAndDeactivateToolRemovesScopeBinding(t *testing.T) {
	ctx := context.Background()
	store := scope.NewMemoryScopeStore()
	manager := scope.NewScopeManager(store, scope.NewScopeEvaluator(store, nil))
	installer := &TypedContributionInstaller{container: &Container{
		ToolRegistry: capability.NewToolRegistry(),
		ScopeManager: manager,
	}}
	contribution := domain.ContributionDefinition{
		ID:          "ui-connect",
		ExtensionID: "com.example.minecraft",
		ModuleID:    "minecraft-runtime",
		Kind:        domain.ContributionKindTool,
		Name:        domain.LocalizedText{Default: "UI Connect"},
		Definition: map[string]any{
			"toolId":       "ui-connect",
			"inputSchema":  map[string]any{"type": "object"},
			"outputSchema": map[string]any{"type": "object"},
		},
	}
	if err := installer.activateTool(ctx, contribution); err != nil {
		t.Fatal(err)
	}
	bindings, err := manager.ListBindings(ctx, scope.ScopeBindingFilter{SubjectType: scope.SubjectTool, SubjectID: "ui-connect"})
	if err != nil {
		t.Fatal(err)
	}
	if len(bindings) != 1 || bindings[0].Scope.Type != scope.ScopeExtension || bindings[0].Scope.ExtensionID != "com.example.minecraft" {
		t.Fatalf("unexpected bindings: %+v", bindings)
	}
	if err := installer.deactivateTool(ctx, contribution); err != nil {
		t.Fatal(err)
	}
	bindings, err = manager.ListBindings(ctx, scope.ScopeBindingFilter{SubjectType: scope.SubjectTool, SubjectID: "ui-connect"})
	if err != nil {
		t.Fatal(err)
	}
	if len(bindings) != 0 {
		t.Fatalf("expected no bindings after deactivation, got %+v", bindings)
	}
}
