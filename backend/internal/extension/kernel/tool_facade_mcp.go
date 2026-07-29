package kernel

import (
	"context"
	"fmt"

	"github.com/u-ai/backend/internal/extension/kernel/capability"
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
		if _, exists := previous[def.ID]; exists {
			result.Updated++
		} else {
			result.Registered++
		}
		if err := f.toolRegistry.Replace(ctx, def); err != nil {
			return result, fmt.Errorf("failed to register MCP tool %s: %w", def.ID, err)
		}
		f.counters.IncMCPToolSync()
	}
	for toolID := range previous {
		if !stillExists(descriptors, toolID) {
			if err := f.toolRegistry.Unregister(ctx, toolID); err != nil {
				return result, fmt.Errorf("failed to unregister stale MCP tool %s: %w", toolID, err)
			}
			result.Removed++
		}
	}
	return result, nil
}

func (f *ToolFacade) UnregisterMCPTools(ctx context.Context, serverID string) []string {
	if f.toolRegistry == nil {
		return nil
	}
	removed := make([]string, 0)
	defs := f.toolRegistry.List(ctx, capability.ToolFilter{Source: capability.ToolSourceMCP})
	for _, def := range defs {
		sid, _ := def.Metadata["mcpServerId"].(string)
		if sid != serverID {
			continue
		}
		if err := f.toolRegistry.Unregister(ctx, def.ID); err == nil {
			removed = append(removed, def.ID)
		}
	}
	return removed
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
