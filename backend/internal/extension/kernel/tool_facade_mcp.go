package kernel

import (
	"context"
	"fmt"

	"github.com/u-ai/backend/internal/extension/kernel/capability"
	"github.com/u-ai/backend/internal/extension/kernel/scope"
)

type MCPToolSyncResult struct {
	ServerID   string
	Registered int
	Updated    int
	Removed    int
	Total      int
}

type MCPDescriptorSource interface {
	ListMCPTools(ctx context.Context, serverID string) ([]capability.MCPToolDescriptor, error)
}

func (f *ToolFacade) SyncMCPTools(ctx context.Context, serverID string, descriptors []capability.MCPToolDescriptor) (*MCPToolSyncResult, error) {
	result := &MCPToolSyncResult{ServerID: serverID, Total: len(descriptors)}
	if f.toolRegistry == nil {
		return result, nil
	}
	if f.capabilityProviders == nil {
		return result, fmt.Errorf("MCP capability provider registry unavailable")
	}
	if f.scopeManager == nil {
		return result, fmt.Errorf("MCP scope manager unavailable")
	}
	f.mcpSyncMu.Lock()
	defer f.mcpSyncMu.Unlock()
	adapter := capability.NewMCPToolAdapter()
	previous := f.previousMCPRevisions(ctx, serverID)
	seen := map[string]bool{}
	for _, descriptor := range descriptors {
		uniqueKey := capability.BuildMCPUniqueKey(descriptor.ServerID, descriptor.Name, descriptor.ExtensionID, descriptor.ModuleID)
		if seen[uniqueKey] {
			f.counters.IncMCPDuplicateDetected()
			continue
		}
		seen[uniqueKey] = true
		def := adapter.AdaptTool(descriptor)
		def.Enabled = true
		_, exists := previous[def.ID]
		if exists {
			result.Updated++
		} else {
			result.Registered++
		}
		if err := f.ensureMCPToolAuthority(ctx, def, descriptor); err != nil {
			return result, fmt.Errorf("failed to register MCP tool authority %s: %w", def.ID, err)
		}
		if err := f.toolRegistry.Replace(ctx, def); err != nil {
			if !exists {
				_ = f.removeMCPToolAuthority(ctx, def.ID)
			}
			return result, fmt.Errorf("failed to register MCP tool %s: %w", def.ID, err)
		}
		f.counters.IncMCPToolSync()
	}
	for toolID := range previous {
		if !stillExists(descriptors, toolID) {
			if err := f.toolRegistry.Unregister(ctx, toolID); err != nil {
				return result, fmt.Errorf("failed to unregister stale MCP tool %s: %w", toolID, err)
			}
			if err := f.removeMCPToolAuthority(ctx, toolID); err != nil {
				return result, fmt.Errorf("failed to remove stale MCP tool authority %s: %w", toolID, err)
			}
			result.Removed++
		}
	}
	return result, nil
}

func (f *ToolFacade) UnregisterMCPTools(ctx context.Context, serverID string) ([]string, error) {
	if f.toolRegistry == nil {
		return nil, nil
	}
	removed := make([]string, 0)
	defs := f.toolRegistry.List(ctx, capability.ToolFilter{Source: capability.ToolSourceMCP})
	for _, def := range defs {
		sid, _ := def.Metadata["mcpServerId"].(string)
		if sid != serverID {
			continue
		}
		if err := f.toolRegistry.Unregister(ctx, def.ID); err != nil {
			return removed, fmt.Errorf("unregister MCP tool %s: %w", def.ID, err)
		}
		if err := f.removeMCPToolAuthority(ctx, def.ID); err != nil {
			return removed, fmt.Errorf("remove MCP tool authority %s: %w", def.ID, err)
		}
		removed = append(removed, def.ID)
	}
	return removed, nil
}

func (f *ToolFacade) ensureMCPToolAuthority(ctx context.Context, def capability.ToolDefinition, descriptor capability.MCPToolDescriptor) error {
	providerID := mcpProviderID(def.ID)
	providerDefinition := capability.CapabilityProviderDefinition{
		ID:           providerID,
		CapabilityID: capability.CapabilityID(def.ID),
		Kind:         capability.ProviderKindMCP,
		Placement:    capability.ProviderPlacementCore,
		RoutingMode:  capability.RoutingModeProviderRequired,
		Runtime:      def.Runtime,
		Priority:     100,
		Metadata: map[string]any{
			"mcpServerId": descriptor.ServerID,
			"mcpToolName": descriptor.Name,
			"toolId":      def.ID,
			"modelName":   def.ModelName,
		},
	}
	if err := f.capabilityProviders.RegisterDefinition(providerDefinition); err != nil {
		return fmt.Errorf("register provider definition: %w", err)
	}
	instance := capability.CapabilityProviderInstance{
		ID:           mcpProviderInstanceID(providerID),
		ProviderID:   providerID,
		CapabilityID: capability.CapabilityID(def.ID),
		Placement:    capability.ProviderPlacementCore,
		Health:       capability.HealthReady,
		Availability: capability.ProviderAvailabilityAvailable,
		Metadata: map[string]any{
			"mcpServerId": descriptor.ServerID,
			"mcpToolName": descriptor.Name,
		},
	}
	if err := f.capabilityProviders.RegisterInstance(instance); err != nil {
		return fmt.Errorf("register provider instance: %w", err)
	}
	bindings, err := f.scopeManager.ListBindings(ctx, scope.ScopeBindingFilter{
		SubjectType: scope.SubjectTool,
		SubjectID:   def.ID,
	})
	if err != nil {
		return fmt.Errorf("list scope bindings: %w", err)
	}
	for _, binding := range bindings {
		if binding.IsActive() && binding.Scope.Type == scope.ScopeGlobal {
			return nil
		}
	}
	if _, err := f.scopeManager.Bind(ctx, scope.ScopeBindRequest{
		SubjectType: scope.SubjectTool,
		SubjectID:   def.ID,
		Scope:       scope.NewGlobalScope(),
		Source:      scope.SourceSystem,
	}); err != nil {
		return fmt.Errorf("bind scope: %w", err)
	}
	return nil
}

func (f *ToolFacade) removeMCPToolAuthority(ctx context.Context, toolID string) error {
	if f.capabilityProviders != nil {
		if _, err := f.capabilityProviders.DeregisterDefinitionCascade(mcpProviderID(toolID)); err != nil {
			return fmt.Errorf("deregister provider: %w", err)
		}
	}
	if f.scopeManager != nil {
		if err := deleteScopeBindings(ctx, f.scopeManager, scope.SubjectTool, toolID); err != nil {
			return fmt.Errorf("delete scope bindings: %w", err)
		}
	}
	return nil
}

func mcpProviderID(toolID string) capability.ProviderID {
	return capability.ProviderID("mcp-provider/" + toolID)
}

func mcpProviderInstanceID(providerID capability.ProviderID) capability.ProviderInstanceID {
	return capability.ProviderInstanceID(string(providerID) + "/instance")
}

func (f *ToolFacade) previousMCPRevisions(ctx context.Context, serverID string) map[string]string {
	previous := map[string]string{}
	if f.toolRegistry == nil {
		return previous
	}
	defs := f.toolRegistry.List(ctx, capability.ToolFilter{Source: capability.ToolSourceMCP})
	for _, def := range defs {
		if sid, _ := def.Metadata["mcpServerId"].(string); sid == serverID {
			previous[def.ID] = def.Version
		}
	}
	return previous
}

func stillExists(descriptors []capability.MCPToolDescriptor, toolID string) bool {
	for _, d := range descriptors {
		if string(capability.BuildMCPCapabilityID(d.ServerID, d.Name)) == toolID {
			return true
		}
	}
	return false
}
