package kernel

import (
	"context"
	"testing"

	"github.com/u-ai/backend/internal/extension/kernel/capability"
	"github.com/u-ai/backend/internal/extension/kernel/execution"
)

func TestB17ToolFacadeModelNameConflictResolved(t *testing.T) {
	toolRegistry := capability.NewToolRegistry()
	executionKernel := &execution.ExecutionPipeline{}
	facade := NewToolFacade(toolRegistry, executionKernel)

	toolA := capability.ToolDefinition{
		ID:          "tool-a",
		ModelName:   "search",
		Source:      capability.ToolSourceBuiltin,
		Name:        "Tool A",
		Description: "Tool A description",
		Enabled:     true,
		InputSchema: []byte(`{"type":"object","properties":{}}`),
	}

	toolB := capability.ToolDefinition{
		ID:          "tool-b",
		ModelName:   "search",
		Source:      capability.ToolSourceBuiltin,
		Name:        "Tool B",
		Description: "Tool B description",
		Enabled:     true,
		InputSchema: []byte(`{"type":"object","properties":{}}`),
	}

	if err := toolRegistry.Register(context.Background(), toolA); err != nil {
		t.Fatalf("unexpected Register error for tool-a: %v", err)
	}

	if err := toolRegistry.Register(context.Background(), toolB); err != nil {
		t.Fatalf("unexpected Register error for tool-b: %v", err)
	}

	scope := LegacyScope{}
	tools, err := facade.ModelTools(context.Background(), scope)
	if err != nil {
		t.Fatalf("unexpected ModelTools error: %v", err)
	}

	if len(tools) != 2 {
		t.Fatalf("expected 2 tools, got %d", len(tools))
	}

	nameSet := make(map[string]string)
	for _, tl := range tools {
		nameSet[tl.Function.Name] = tl.Function.Description
	}

	if desc, ok := nameSet["search"]; !ok || desc != "Tool A description" {
		t.Fatalf("expected 'search' tool with description 'Tool A description', got map: %v", nameSet)
	}

	if desc, ok := nameSet["search_2"]; !ok || desc != "Tool B description" {
		t.Fatalf("expected 'search_2' tool with description 'Tool B description', got map: %v", nameSet)
	}

	if _, ok := nameSet["search_3"]; ok {
		t.Fatalf("unexpected 'search_3' tool found: %v", nameSet)
	}
}

func TestB17ToolFacadeExecuteModelToolResolved(t *testing.T) {
	toolRegistry := capability.NewToolRegistry()
	executionKernel := &execution.ExecutionPipeline{}
	facade := NewToolFacade(toolRegistry, executionKernel)

	toolA := capability.ToolDefinition{
		ID:          "tool-a",
		ModelName:   "search",
		Source:      capability.ToolSourceBuiltin,
		Name:        "Tool A",
		Description: "Tool A description",
		Enabled:     true,
		InputSchema: []byte(`{"type":"object","properties":{}}`),
	}

	toolB := capability.ToolDefinition{
		ID:          "tool-b",
		ModelName:   "search",
		Source:      capability.ToolSourceBuiltin,
		Name:        "Tool B",
		Description: "Tool B description",
		Enabled:     true,
		InputSchema: []byte(`{"type":"object","properties":{}}`),
	}

	if err := toolRegistry.Register(context.Background(), toolA); err != nil {
		t.Fatalf("unexpected Register error for tool-a: %v", err)
	}

	if err := toolRegistry.Register(context.Background(), toolB); err != nil {
		t.Fatalf("unexpected Register error for tool-b: %v", err)
	}

	input := []byte(`{}`)
	scope := LegacyScope{}

	_, foundSearch := facade.ExecuteModelTool(context.Background(), "search", input, scope, "")
	if !foundSearch {
		t.Fatal("expected 'search' to be found")
	}

	_, foundSearch2 := facade.ExecuteModelTool(context.Background(), "search_2", input, scope, "")
	if !foundSearch2 {
		t.Fatal("expected 'search_2' to be found")
	}
}

func TestB17ToolModelToolViewConsistentWithResolvedName(t *testing.T) {
	toolA := capability.ToolDefinition{
		ID:          "tool-a",
		ModelName:   "search",
		Source:      capability.ToolSourceBuiltin,
		Name:        "Tool A",
		Description: "Tool A description",
		Enabled:     true,
	}

	toolB := capability.ToolDefinition{
		ID:          "tool-b",
		ModelName:   "search",
		Source:      capability.ToolSourceBuiltin,
		Name:        "Tool B",
		Description: "Tool B description",
		Enabled:     true,
	}

	reg := capability.NewToolRegistry()
	reg.Register(context.Background(), toolA)
	reg.Register(context.Background(), toolB)

	all := reg.List(context.Background(), capability.ToolFilter{})
	for _, def := range all {
		view := def.ModelToolView()
		if view.Name != def.ModelName {
			t.Fatalf("ModelToolView.Name (%s) != ToolDefinition.ModelName (%s) for tool %s",
				view.Name, def.ModelName, def.ID)
		}
	}
}
