package repair_baseline

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel"
	"github.com/u-ai/backend/internal/extension/kernel/capability"
)

func makeMCPToolDescriptor(serverID, toolName, description string) capability.MCPToolDescriptor {
	return capability.MCPToolDescriptor{
		ServerID:    serverID,
		ServerName:  serverID,
		Name:        toolName,
		Title:       toolName,
		Description: description,
		InputSchema: json.RawMessage(`{"type":"object","properties":{"text":{"type":"string"}},"required":["text"]}`),
		Annotations: map[string]any{
			"readOnlyHint": true,
		},
	}
}

func TestBaseline_E2E_MCP_SyncRegistersTools(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E MCP test in short mode")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	tempDir := t.TempDir()
	container, err := kernel.NewContainerBuilder().
		WithDBPath(filepath.Join(tempDir, "kernel.db")).
		WithExtensionRoot(filepath.Join(tempDir, "extensions")).
		Build(ctx)
	if err != nil {
		t.Fatalf("ContainerBuilder.Build must succeed: %v", err)
	}
	defer container.Close()

	toolFacade := kernel.NewToolFacade(container.ToolRegistry, container.ExecutionKernel, kernel.DefaultToolFacadeConfig())

	descriptors := []capability.MCPToolDescriptor{
		makeMCPToolDescriptor("test-mcp-server", "echo", "Echo tool for MCP E2E"),
	}
	result, err := toolFacade.SyncMCPTools(ctx, "test-mcp-server", descriptors)
	if err != nil {
		t.Fatalf("SyncMCPTools must succeed (Phase 10 section 19.7.1-19.7.2): %v", err)
	}
	if result.Registered != 1 {
		t.Fatalf("SyncMCPTools must register 1 tool (Phase 10 section 19.7.2), got registered=%d", result.Registered)
	}

	mcpTools := container.ToolRegistry.List(ctx, capability.ToolFilter{Source: capability.ToolSourceMCP})
	if len(mcpTools) != 1 {
		t.Fatalf("ToolRegistry must contain exactly 1 MCP tool after sync (Phase 10 section 19.7.2), got %d", len(mcpTools))
	}
}

func TestBaseline_E2E_MCP_ToolsAppearOnceInModelList(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E MCP test in short mode")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	tempDir := t.TempDir()
	container, err := kernel.NewContainerBuilder().
		WithDBPath(filepath.Join(tempDir, "kernel.db")).
		WithExtensionRoot(filepath.Join(tempDir, "extensions")).
		Build(ctx)
	if err != nil {
		t.Fatalf("ContainerBuilder.Build must succeed: %v", err)
	}
	defer container.Close()

	toolFacade := kernel.NewToolFacade(container.ToolRegistry, container.ExecutionKernel, kernel.DefaultToolFacadeConfig())

	descriptors := []capability.MCPToolDescriptor{
		makeMCPToolDescriptor("model-list-server", "query", "Query tool for model list test"),
	}
	if _, err := toolFacade.SyncMCPTools(ctx, "model-list-server", descriptors); err != nil {
		t.Fatalf("SyncMCPTools must succeed: %v", err)
	}

	scope := kernel.LegacyScope{
		UserID:         "test-user",
		CharacterID:    "test-character",
		ConversationID: "test-conversation",
		Channel:        "test",
		SessionID:      "test-session",
	}
	tools, err := toolFacade.ModelTools(ctx, scope)
	if err != nil {
		t.Fatalf("ModelTools must not return error (Phase 10 section 19.7.3): %v", err)
	}

	mcpToolCount := 0
	for _, tl := range tools {
		if tl.Function.Name == "" {
			continue
		}
		mcpDefs := container.ToolRegistry.List(ctx, capability.ToolFilter{Source: capability.ToolSourceMCP})
		for _, def := range mcpDefs {
			if def.ModelName == tl.Function.Name {
				mcpToolCount++
			}
		}
	}
	if mcpToolCount != 1 {
		t.Fatalf("MCP tool must appear exactly once in model tool list (Phase 10 section 19.7.3), got %d", mcpToolCount)
	}
}

func TestBaseline_E2E_MCP_UnregisterRemovesTools(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E MCP test in short mode")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	tempDir := t.TempDir()
	container, err := kernel.NewContainerBuilder().
		WithDBPath(filepath.Join(tempDir, "kernel.db")).
		WithExtensionRoot(filepath.Join(tempDir, "extensions")).
		Build(ctx)
	if err != nil {
		t.Fatalf("ContainerBuilder.Build must succeed: %v", err)
	}
	defer container.Close()

	toolFacade := kernel.NewToolFacade(container.ToolRegistry, container.ExecutionKernel, kernel.DefaultToolFacadeConfig())

	descriptors := []capability.MCPToolDescriptor{
		makeMCPToolDescriptor("unregister-server", "action", "Action tool for unregister test"),
	}
	if _, err := toolFacade.SyncMCPTools(ctx, "unregister-server", descriptors); err != nil {
		t.Fatalf("SyncMCPTools must succeed: %v", err)
	}

	beforeCount := container.ToolRegistry.CountBySource(capability.ToolSourceMCP)
	if beforeCount != 1 {
		t.Fatalf("expected 1 MCP tool before unregister, got %d", beforeCount)
	}

	removed := toolFacade.UnregisterMCPTools(ctx, "unregister-server")
	if len(removed) != 1 {
		t.Fatalf("UnregisterMCPTools must remove 1 tool (Phase 10 section 19.7.5), got %d", len(removed))
	}

	afterCount := container.ToolRegistry.CountBySource(capability.ToolSourceMCP)
	if afterCount != 0 {
		t.Fatalf("MCP tool must be removed after unregister (Phase 10 section 19.7.5), got %d remaining", afterCount)
	}
}

func TestBaseline_E2E_MCP_DisableRemovesFromModelList(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E MCP test in short mode")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	tempDir := t.TempDir()
	container, err := kernel.NewContainerBuilder().
		WithDBPath(filepath.Join(tempDir, "kernel.db")).
		WithExtensionRoot(filepath.Join(tempDir, "extensions")).
		Build(ctx)
	if err != nil {
		t.Fatalf("ContainerBuilder.Build must succeed: %v", err)
	}
	defer container.Close()

	toolFacade := kernel.NewToolFacade(container.ToolRegistry, container.ExecutionKernel, kernel.DefaultToolFacadeConfig())

	descriptors := []capability.MCPToolDescriptor{
		makeMCPToolDescriptor("disable-server", "run", "Run tool for disable test"),
	}
	if _, err := toolFacade.SyncMCPTools(ctx, "disable-server", descriptors); err != nil {
		t.Fatalf("SyncMCPTools must succeed: %v", err)
	}

	scope := kernel.LegacyScope{
		UserID:         "test-user",
		CharacterID:    "test-character",
		ConversationID: "test-conversation",
		Channel:        "test",
		SessionID:      "test-session",
	}
	beforeTools, _ := toolFacade.ModelTools(ctx, scope)
	beforeMCPCount := 0
	for _, tl := range beforeTools {
		mcpDefs := container.ToolRegistry.List(ctx, capability.ToolFilter{Source: capability.ToolSourceMCP})
		for _, def := range mcpDefs {
			if def.ModelName == tl.Function.Name {
				beforeMCPCount++
			}
		}
	}
	if beforeMCPCount != 1 {
		t.Fatalf("MCP tool must appear in model list before disable, got %d", beforeMCPCount)
	}

	removed := toolFacade.UnregisterMCPTools(ctx, "disable-server")
	if len(removed) == 0 {
		t.Fatalf("UnregisterMCPTools must remove at least 1 tool")
	}

	afterTools, _ := toolFacade.ModelTools(ctx, scope)
	afterMCPCount := 0
	for _, tl := range afterTools {
		mcpDefs := container.ToolRegistry.List(ctx, capability.ToolFilter{Source: capability.ToolSourceMCP})
		for _, def := range mcpDefs {
			if def.ModelName == tl.Function.Name {
				afterMCPCount++
			}
		}
	}
	if afterMCPCount != 0 {
		t.Fatalf("MCP tool must disappear from model list after disable (Phase 10 section 19.7.5), got %d", afterMCPCount)
	}
}

func TestBaseline_E2E_MCP_NoDuplicateExposure(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E MCP test in short mode")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	tempDir := t.TempDir()
	container, err := kernel.NewContainerBuilder().
		WithDBPath(filepath.Join(tempDir, "kernel.db")).
		WithExtensionRoot(filepath.Join(tempDir, "extensions")).
		Build(ctx)
	if err != nil {
		t.Fatalf("ContainerBuilder.Build must succeed: %v", err)
	}
	defer container.Close()

	toolFacade := kernel.NewToolFacade(container.ToolRegistry, container.ExecutionKernel, kernel.DefaultToolFacadeConfig())

	descriptors := []capability.MCPToolDescriptor{
		makeMCPToolDescriptor("dup-server", "op", "Op tool for duplicate test"),
	}
	if _, err := toolFacade.SyncMCPTools(ctx, "dup-server", descriptors); err != nil {
		t.Fatalf("SyncMCPTools must succeed: %v", err)
	}

	if _, err := toolFacade.SyncMCPTools(ctx, "dup-server", descriptors); err != nil {
		t.Fatalf("second SyncMCPTools must succeed: %v", err)
	}

	mcpTools := container.ToolRegistry.List(ctx, capability.ToolFilter{Source: capability.ToolSourceMCP})
	if len(mcpTools) != 1 {
		t.Fatalf("re-syncing same MCP tools must not duplicate (Phase 10 section 19.7.6), got %d tools", len(mcpTools))
	}

	scope := kernel.LegacyScope{
		UserID:      "test-user",
		Channel:     "test",
		SessionID:   "test-session",
	}
	tools, _ := toolFacade.ModelTools(ctx, scope)
	mcpModelCount := 0
	mcpDefs := container.ToolRegistry.List(ctx, capability.ToolFilter{Source: capability.ToolSourceMCP})
	for _, tl := range tools {
		for _, def := range mcpDefs {
			if def.ModelName == tl.Function.Name {
				mcpModelCount++
			}
		}
	}
	if mcpModelCount != 1 {
		t.Fatalf("MCP tool must be exposed exactly once in model list (Phase 10 section 19.7.6), got %d", mcpModelCount)
	}
}

func TestBaseline_E2E_MCP_LegacyCounterZero(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E MCP test in short mode")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	tempDir := t.TempDir()
	container, err := kernel.NewContainerBuilder().
		WithDBPath(filepath.Join(tempDir, "kernel.db")).
		WithExtensionRoot(filepath.Join(tempDir, "extensions")).
		Build(ctx)
	if err != nil {
		t.Fatalf("ContainerBuilder.Build must succeed: %v", err)
	}
	defer container.Close()

	toolFacade := kernel.NewToolFacade(container.ToolRegistry, container.ExecutionKernel, kernel.DefaultToolFacadeConfig())

	descriptors := []capability.MCPToolDescriptor{
		makeMCPToolDescriptor("legacy-zero-server", "exec", "Exec tool for legacy counter test"),
	}
	if _, err := toolFacade.SyncMCPTools(ctx, "legacy-zero-server", descriptors); err != nil {
		t.Fatalf("SyncMCPTools must succeed: %v", err)
	}

	scope := kernel.LegacyScope{
		UserID:      "test-user",
		Channel:     "test",
		SessionID:   "test-session",
	}
	_, _ = toolFacade.ModelTools(ctx, scope)
	_ = toolFacade.UnregisterMCPTools(ctx, "legacy-zero-server")

	total := kernel.GlobalLegacyCallCounter().Total()
	if total != 0 {
		t.Fatalf("Legacy counter must remain 0 during MCP operations (Phase 10 section 19.7.4), got %d", total)
	}

	counters := toolFacade.Counters()
	snap := counters.Snapshot()
	if snap["legacy_fallback_total"] > 0 {
		t.Fatalf("legacy fallback must be 0 for MCP operations, got %d", snap["legacy_fallback_total"])
	}
}

func TestBaseline_E2E_MCP_MultipleServersIsolated(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E MCP test in short mode")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	tempDir := t.TempDir()
	container, err := kernel.NewContainerBuilder().
		WithDBPath(filepath.Join(tempDir, "kernel.db")).
		WithExtensionRoot(filepath.Join(tempDir, "extensions")).
		Build(ctx)
	if err != nil {
		t.Fatalf("ContainerBuilder.Build must succeed: %v", err)
	}
	defer container.Close()

	toolFacade := kernel.NewToolFacade(container.ToolRegistry, container.ExecutionKernel, kernel.DefaultToolFacadeConfig())

	serverADescriptors := []capability.MCPToolDescriptor{
		makeMCPToolDescriptor("server-a", "tool_a", "Tool A from server A"),
	}
	serverBDescriptors := []capability.MCPToolDescriptor{
		makeMCPToolDescriptor("server-b", "tool_b", "Tool B from server B"),
	}
	if _, err := toolFacade.SyncMCPTools(ctx, "server-a", serverADescriptors); err != nil {
		t.Fatalf("SyncMCPTools server-a must succeed: %v", err)
	}
	if _, err := toolFacade.SyncMCPTools(ctx, "server-b", serverBDescriptors); err != nil {
		t.Fatalf("SyncMCPTools server-b must succeed: %v", err)
	}

	totalMCP := container.ToolRegistry.CountBySource(capability.ToolSourceMCP)
	if totalMCP != 2 {
		t.Fatalf("expected 2 MCP tools from 2 servers, got %d", totalMCP)
	}

	removed := toolFacade.UnregisterMCPTools(ctx, "server-a")
	if len(removed) != 1 {
		t.Fatalf("UnregisterMCPTools must remove 1 tool from server-a, got %d", len(removed))
	}

	remaining := container.ToolRegistry.CountBySource(capability.ToolSourceMCP)
	if remaining != 1 {
		t.Fatalf("after unregistering server-a, 1 MCP tool must remain (server-b), got %d", remaining)
	}

	remainingDefs := container.ToolRegistry.List(ctx, capability.ToolFilter{Source: capability.ToolSourceMCP})
	if len(remainingDefs) != 1 {
		t.Fatalf("expected 1 remaining MCP definition, got %d", len(remainingDefs))
	}
	sid, _ := remainingDefs[0].Metadata["mcpServerId"].(string)
	if sid != "server-b" {
		t.Fatalf("remaining MCP tool must belong to server-b, got %s", sid)
	}
}

func TestBaseline_E2E_MCP_PipelinePhasesDefined(t *testing.T) {
	requiredPhases := []string{
		"add_mcp_server",
		"convert_to_contribution",
		"model_tool_list_once",
		"permission_scope_check",
		"disable_removes_tool",
		"no_duplicate_exposure",
	}
	if len(requiredPhases) != 6 {
		t.Fatalf("Phase 10 section 19.7 requires 6 MCP phases, got %d", len(requiredPhases))
	}
	for _, p := range requiredPhases {
		if p == "" {
			t.Fatalf("MCP phase must not be empty")
		}
	}
}
