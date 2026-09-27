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

	scope := InvocationScope{}
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

func TestB17AndroidUIAgentUsesOpenAICompatibleModelName(t *testing.T) {
	toolRegistry := capability.NewToolRegistry()
	if err := registerAndroidUIAgentTool(context.Background(), toolRegistry); err != nil {
		t.Fatalf("unexpected registerAndroidUIAgentTool error: %v", err)
	}

	definition, ok := toolRegistry.Get(context.Background(), "android.ui.agent.run")
	if !ok {
		t.Fatal("expected android.ui.agent.run to be registered")
	}
	if definition.ModelName != "android_ui_agent_run" {
		t.Fatalf("expected model name android_ui_agent_run, got %s", definition.ModelName)
	}

	facade := NewToolFacade(toolRegistry, &execution.ExecutionPipeline{})
	tools, err := facade.ModelTools(context.Background(), InvocationScope{})
	if err != nil {
		t.Fatalf("unexpected ModelTools error: %v", err)
	}
	found := false
	for _, item := range tools {
		if item.Function.Name == "android_ui_agent_run" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected android_ui_agent_run in model tools")
	}
}

func TestB17FullAccessUsesFullControlApprovalMode(t *testing.T) {
	if got := capabilityApprovalMode("full_access"); got != capability.ApprovalModeFullControl {
		t.Fatalf("expected full_control approval mode, got %s", got)
	}
	if got := capabilityApprovalMode("request_approval"); got != capability.ApprovalModeManual {
		t.Fatalf("expected manual approval mode, got %s", got)
	}
}

func TestB17ModelToolsSortByExposurePriority(t *testing.T) {
	toolRegistry := capability.NewToolRegistry()
	lowPriority := capability.ToolDefinition{
		ID:          "tool-low",
		ModelName:   "tool_low",
		Source:      capability.ToolSourceBuiltin,
		Name:        "Low",
		Description: "Low priority tool",
		Enabled:     true,
		InputSchema: []byte(`{"type":"object","properties":{}}`),
		ModelExposure: capability.ModelExposureRule{
			ExposedByDefault: true,
			Priority:         1,
		},
	}
	highPriority := capability.ToolDefinition{
		ID:          "tool-high",
		ModelName:   "tool_high",
		Source:      capability.ToolSourceBuiltin,
		Name:        "High",
		Description: "High priority tool",
		Enabled:     true,
		InputSchema: []byte(`{"type":"object","properties":{}}`),
		ModelExposure: capability.ModelExposureRule{
			ExposedByDefault: true,
			Priority:         100,
		},
	}
	if err := toolRegistry.Register(context.Background(), lowPriority); err != nil {
		t.Fatalf("unexpected low priority register error: %v", err)
	}
	if err := toolRegistry.Register(context.Background(), highPriority); err != nil {
		t.Fatalf("unexpected high priority register error: %v", err)
	}

	facade := NewToolFacade(toolRegistry, &execution.ExecutionPipeline{})
	tools, err := facade.ModelTools(context.Background(), InvocationScope{})
	if err != nil {
		t.Fatalf("unexpected ModelTools error: %v", err)
	}
	if len(tools) != 2 {
		t.Fatalf("expected 2 tools, got %d", len(tools))
	}
	if tools[0].Function.Name != "tool_high" {
		t.Fatalf("expected tool_high first, got %s", tools[0].Function.Name)
	}
}

func TestBuildModelToolsSkipsInvalidSchema(t *testing.T) {
	definitions := []capability.ToolDefinition{
		{
			ID:          "valid-tool",
			ModelName:   "valid_tool",
			Source:      capability.ToolSourceBuiltin,
			Name:        "Valid Tool",
			Description: "Valid tool",
			Enabled:     true,
			InputSchema: []byte(`{
				"type": "object",
				"properties": {
					"value": {"type": ["string", "number", "boolean", "null"]}
				}
			}`),
		},
		{
			ID:          "invalid-tool",
			ModelName:   "invalid_tool",
			Source:      capability.ToolSourceBuiltin,
			Name:        "Invalid Tool",
			Description: "Invalid tool",
			Enabled:     true,
			InputSchema: []byte(`{"type":"object","required":[1]}`),
		},
	}

	tools := buildModelToolsFromDefinitions(definitions, InvocationScope{})
	if len(tools) != 1 {
		t.Fatalf("expected invalid schema to be skipped, got %d tools", len(tools))
	}
	if tools[0].Function.Name != "valid_tool" {
		t.Fatalf("expected valid_tool, got %s", tools[0].Function.Name)
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
	scope := InvocationScope{}

	_, foundSearch := facade.ExecuteModelTool(context.Background(), "search", input, scope, "")
	if !foundSearch {
		t.Fatal("expected 'search' to be found")
	}

	_, foundSearch2 := facade.ExecuteModelTool(context.Background(), "search_2", input, scope, "")
	if !foundSearch2 {
		t.Fatal("expected 'search_2' to be found")
	}
}

func TestB17ResolveModelToolResolvesCanonicalID(t *testing.T) {
	toolRegistry := capability.NewToolRegistry()
	executionKernel := &execution.ExecutionPipeline{}
	facade := NewToolFacade(toolRegistry, executionKernel)

	def := capability.ToolDefinition{
		ID:          "builtin/browser/search",
		ModelName:   "browser_search",
		Source:      capability.ToolSourceBuiltin,
		Name:        "Browser Search",
		Description: "Search the web",
		Enabled:     true,
		InputSchema: []byte(`{"type":"object"}`),
	}
	if err := toolRegistry.Register(context.Background(), def); err != nil {
		t.Fatalf("unexpected register error: %v", err)
	}

	ref, err := facade.ResolveModelTool("browser_search")
	if err != nil {
		t.Fatalf("unexpected resolve error: %v", err)
	}
	if string(ref.ID) != "builtin/browser/search" {
		t.Fatalf("expected canonical ID builtin/browser/search, got %s", ref.ID)
	}
	if ref.ModelName != "browser_search" {
		t.Fatalf("expected model name browser_search, got %s", ref.ModelName)
	}
}

func TestB17ResolveModelToolReturnsErrorWhenRegistryMissing(t *testing.T) {
	toolRegistry := capability.NewToolRegistry()
	executionKernel := &execution.ExecutionPipeline{}
	facade := NewToolFacade(toolRegistry, executionKernel)

	_, err := facade.ResolveModelTool("nonexistent_tool")
	if err == nil {
		t.Fatal("expected error for missing tool, got nil")
	}
}

func TestB17ExecuteToolUsesCanonicalID(t *testing.T) {
	toolRegistry := capability.NewToolRegistry()
	executionKernel := &execution.ExecutionPipeline{}
	facade := NewToolFacade(toolRegistry, executionKernel)

	def := capability.ToolDefinition{
		ID:          "builtin/browser/search",
		ModelName:   "browser_search",
		Source:      capability.ToolSourceBuiltin,
		Name:        "Browser Search",
		Description: "Search the web",
		Enabled:     true,
		InputSchema: []byte(`{"type":"object"}`),
	}
	if err := toolRegistry.Register(context.Background(), def); err != nil {
		t.Fatalf("unexpected register error: %v", err)
	}

	input := []byte(`{"q":"hello"}`)
	scope := InvocationScope{ToolCallID: "scope_call_id"}
	result, ok := facade.ExecuteTool(
		context.Background(),
		capability.CapabilityID("builtin/browser/search"),
		input,
		scope,
		"external_call_123",
		"agent-action:action-id-001",
	)
	if !ok {
		t.Fatalf("expected ok=true for existing tool, got false (status=%s)", result.Status)
	}
	if facade.Counters().Snapshot()["execute_model_tool"] != 1 {
		t.Fatalf("expected IncExecuteModelTool called once, got %d", facade.Counters().Snapshot()["execute_model_tool"])
	}
}

func TestB17ExecuteToolReturnsFalseWhenToolNotFound(t *testing.T) {
	toolRegistry := capability.NewToolRegistry()
	executionKernel := &execution.ExecutionPipeline{}
	facade := NewToolFacade(toolRegistry, executionKernel)

	_, ok := facade.ExecuteTool(
		context.Background(),
		capability.CapabilityID("nonexistent/tool"),
		nil,
		InvocationScope{},
		"call_x",
		"key",
	)
	if ok {
		t.Fatal("expected ok=false for missing tool")
	}
	if facade.Counters().Snapshot()["pipeline_executions"] != 0 {
		t.Fatal("expected pipeline executions counter to remain 0 when tool not found")
	}
}

func TestB17ExecuteToolRejectsModelNameAsID(t *testing.T) {
	toolRegistry := capability.NewToolRegistry()
	executionKernel := &execution.ExecutionPipeline{}
	facade := NewToolFacade(toolRegistry, executionKernel)

	def := capability.ToolDefinition{
		ID:          "builtin/calendar/create",
		ModelName:   "create_event",
		Source:      capability.ToolSourceBuiltin,
		Name:        "Create Event",
		Description: "Create a calendar event",
		Enabled:     true,
		InputSchema: []byte(`{"type":"object"}`),
	}
	if err := toolRegistry.Register(context.Background(), def); err != nil {
		t.Fatalf("unexpected register error: %v", err)
	}

	scope := InvocationScope{ToolCallID: "scope_id"}
	_, ok := facade.ExecuteTool(
		context.Background(),
		capability.CapabilityID("create_event"),
		[]byte(`{}`),
		scope,
		"call_y",
		"key-y",
	)
	if ok {
		t.Fatal("ExecuteTool should not accept model name as ID; canonical ID required")
	}
}

func TestB17ExecuteToolNilRegistry(t *testing.T) {
	facade := NewToolFacade(nil, &execution.ExecutionPipeline{})

	result, ok := facade.ExecuteTool(
		context.Background(),
		capability.CapabilityID("any/tool"),
		nil,
		InvocationScope{},
		"call_z",
		"key-z",
	)
	if ok {
		t.Fatal("expected ok=false when registry nil")
	}
	if result.Status != "FAILED" {
		t.Fatalf("expected FAILED status, got %s", result.Status)
	}
	if result.Error == nil || result.Error.Code != "TOOL_REGISTRY_UNAVAILABLE" {
		t.Fatalf("expected TOOL_REGISTRY_UNAVAILABLE, got %+v", result.Error)
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
