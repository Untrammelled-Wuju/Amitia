package repair_baseline

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel"
	"github.com/u-ai/backend/internal/extension/kernel/capability"
)

func TestBaseline_MCP_SyncErrorOnReplaceFailure(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	registry := capability.NewToolRegistry()

	existingDef := capability.ToolDefinition{
		ID:          "mcp/server/tool",
		Source:      capability.ToolSourceMCP,
		Enabled:     true,
		ExtensionID: "ext-existing",
	}
	if err := registry.Register(ctx, existingDef); err != nil {
		t.Fatalf("pre-register must succeed: %v", err)
	}

	facade := kernel.NewToolFacade(registry, nil, kernel.DefaultToolFacadeConfig())

	descriptors := []capability.MCPToolDescriptor{
		{
			ServerID:    "server",
			ServerName:  "server",
			Name:        "tool",
			Title:       "Tool",
			Description: "Test tool",
			InputSchema: json.RawMessage(`{"type":"object"}`),
			ExtensionID: "ext-new",
		},
	}

	_, err := facade.SyncMCPTools(ctx, "server", descriptors)
	if err == nil {
		t.Fatalf("SyncMCPTools must return error when Replace fails (owner conflict)")
	}
}

func TestBaseline_MCP_DuplicateCounterIncremented(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	registry := capability.NewToolRegistry()
	facade := kernel.NewToolFacade(registry, nil, kernel.DefaultToolFacadeConfig())

	descriptors := []capability.MCPToolDescriptor{
		{
			ServerID:    "server",
			ServerName:  "server",
			Name:        "tool",
			Title:       "Tool",
			Description: "Test tool",
			InputSchema: json.RawMessage(`{"type":"object"}`),
		},
		{
			ServerID:    "server",
			ServerName:  "server",
			Name:        "tool",
			Title:       "Tool",
			Description: "Test tool",
			InputSchema: json.RawMessage(`{"type":"object"}`),
		},
	}

	result, err := facade.SyncMCPTools(ctx, "server", descriptors)
	if err != nil {
		t.Fatalf("SyncMCPTools must succeed: %v", err)
	}
	if result.Registered != 1 {
		t.Fatalf("expected 1 registered tool, got %d", result.Registered)
	}

	snap := facade.Counters().Snapshot()
	if snap["mcp_duplicate_detected"] != 1 {
		t.Fatalf("expected mcp_duplicate_detected=1, got %d", snap["mcp_duplicate_detected"])
	}

	mcpTools := registry.List(ctx, capability.ToolFilter{Source: capability.ToolSourceMCP})
	if len(mcpTools) != 1 {
		t.Fatalf("expected 1 MCP tool in registry, got %d", len(mcpTools))
	}
}

func TestBaseline_MCP_StaleToolRemoved(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	registry := capability.NewToolRegistry()
	facade := kernel.NewToolFacade(registry, nil, kernel.DefaultToolFacadeConfig())

	initialDescriptors := []capability.MCPToolDescriptor{
		{
			ServerID:    "server",
			ServerName:  "server",
			Name:        "tool_a",
			Title:       "Tool A",
			Description: "First tool",
			InputSchema: json.RawMessage(`{"type":"object"}`),
		},
		{
			ServerID:    "server",
			ServerName:  "server",
			Name:        "tool_b",
			Title:       "Tool B",
			Description: "Second tool",
			InputSchema: json.RawMessage(`{"type":"object"}`),
		},
	}
	if _, err := facade.SyncMCPTools(ctx, "server", initialDescriptors); err != nil {
		t.Fatalf("initial SyncMCPTools must succeed: %v", err)
	}

	updatedDescriptors := []capability.MCPToolDescriptor{
		{
			ServerID:    "server",
			ServerName:  "server",
			Name:        "tool_a",
			Title:       "Tool A",
			Description: "First tool updated",
			InputSchema: json.RawMessage(`{"type":"object"}`),
		},
	}
	result, err := facade.SyncMCPTools(ctx, "server", updatedDescriptors)
	if err != nil {
		t.Fatalf("second SyncMCPTools must succeed: %v", err)
	}
	if result.Removed != 1 {
		t.Fatalf("expected 1 removed stale tool, got %d", result.Removed)
	}

	mcpTools := registry.List(ctx, capability.ToolFilter{Source: capability.ToolSourceMCP})
	if len(mcpTools) != 1 {
		t.Fatalf("expected 1 MCP tool after stale removal, got %d", len(mcpTools))
	}
}
