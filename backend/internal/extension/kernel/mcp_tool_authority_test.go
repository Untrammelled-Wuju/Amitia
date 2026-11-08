package kernel

import (
	"context"
	"testing"

	"github.com/u-ai/backend/internal/extension/kernel/capability"
	"github.com/u-ai/backend/internal/extension/kernel/scope"
)

func TestSyncMCPToolsRegistersExecutableAuthorityAndCleansUp(t *testing.T) {
	ctx := context.Background()
	toolRegistry := capability.NewToolRegistry()
	providerRegistry := capability.NewProviderRegistry()
	scopeStore := scope.NewMemoryScopeStore()
	scopeManager := scope.NewScopeManager(scopeStore, scope.NewScopeEvaluator(scopeStore, nil))
	facade := NewToolFacade(toolRegistry, nil)
	facade.SetCapabilityProviders(providerRegistry)
	facade.SetScopeManager(scopeManager)

	const serverID = "qa-server"
	const toolName = "echo"
	descriptors := []capability.MCPToolDescriptor{{
		ServerID:     serverID,
		ServerName:   "QA Server",
		Name:         toolName,
		Title:        "Echo",
		InputSchema:  []byte(`{"type":"object","properties":{"text":{"type":"string"}},"required":["text"]}`),
		OutputSchema: []byte(`{}`),
	}}

	result, err := facade.SyncMCPTools(ctx, serverID, descriptors)
	if err != nil {
		t.Fatalf("SyncMCPTools() error = %v", err)
	}
	if result.Registered != 1 {
		t.Fatalf("registered = %d, want 1", result.Registered)
	}

	toolID := string(capability.BuildMCPCapabilityID(serverID, toolName))
	tool, ok := toolRegistry.Get(ctx, toolID)
	if !ok {
		t.Fatalf("MCP tool %s was not registered", toolID)
	}
	if tool.CapabilityID != capability.CapabilityID(toolID) {
		t.Fatalf("MCP tool capability ID = %q, want %q", tool.CapabilityID, toolID)
	}
	if len(providerRegistry.ListByCapability(capability.CapabilityID(toolID))) != 1 {
		t.Fatalf("MCP capability provider definition was not registered")
	}
	if providerRegistry.CountExecutableInstances(capability.CapabilityID(toolID)) != 1 {
		t.Fatalf("MCP capability provider instance is not executable")
	}
	decision := scopeManager.Evaluate(ctx, scope.ScopeEvaluationRequest{
		SubjectType: scope.SubjectTool,
		SubjectID:   toolID,
		CharacterID: "qa-character",
	})
	if !decision.Allowed {
		t.Fatalf("MCP tool scope denied: %+v", decision.Reasons)
	}

	result, err = facade.SyncMCPTools(ctx, serverID, nil)
	if err != nil {
		t.Fatalf("SyncMCPTools(nil) error = %v", err)
	}
	if result.Removed != 1 {
		t.Fatalf("removed = %d, want 1", result.Removed)
	}
	if _, ok := toolRegistry.Get(ctx, toolID); ok {
		t.Fatalf("MCP tool %s remained registered", toolID)
	}
	if len(providerRegistry.ListByCapability(capability.CapabilityID(toolID))) != 0 {
		t.Fatalf("MCP capability provider definition remained registered")
	}
	if providerRegistry.CountInstancesByCapability(capability.CapabilityID(toolID)) != 0 {
		t.Fatalf("MCP capability provider instance remained registered")
	}
	decision = scopeManager.Evaluate(ctx, scope.ScopeEvaluationRequest{
		SubjectType: scope.SubjectTool,
		SubjectID:   toolID,
		CharacterID: "qa-character",
	})
	if decision.Allowed {
		t.Fatalf("MCP tool scope remained authorized after removal")
	}
}
