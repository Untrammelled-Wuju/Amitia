package kernel

import (
	"context"
	"fmt"

	"github.com/u-ai/backend/internal/extension/kernel/capability"
	"github.com/u-ai/backend/internal/extension/kernel/capability/acquisition"
)

// acquisitionMCPToolSync is the acquisition-side canonical bridge from MCP
// discovery descriptors into the kernel ToolRegistry. It deliberately does not
// own MCP process state; the MCP runtime connector is the authoritative runtime.
type acquisitionMCPToolSync struct {
	registry *capability.ToolRegistry
}

func NewAcquisitionMCPToolSync(registry *capability.ToolRegistry) acquisition.MCPToolSyncPort {
	return &acquisitionMCPToolSync{registry: registry}
}

func (s *acquisitionMCPToolSync) SyncMCPTools(ctx context.Context, serverID string, descriptors []capability.MCPToolDescriptor) (*acquisition.MCPToolSyncResult, error) {
	result := &acquisition.MCPToolSyncResult{ServerID: serverID, Total: len(descriptors)}
	if s.registry == nil {
		return result, fmt.Errorf("MCP tool sync: tool registry not configured")
	}

	adapter := capability.NewMCPToolAdapter()
	previous := make(map[string]struct{})
	for _, def := range s.registry.List(ctx, capability.ToolFilter{Source: capability.ToolSourceMCP}) {
		if sid, _ := def.Metadata["mcpServerId"].(string); sid == serverID {
			previous[def.ID] = struct{}{}
		}
	}

	seen := make(map[string]struct{}, len(descriptors))
	for _, descriptor := range descriptors {
		descriptor.ServerID = serverID
		def := adapter.AdaptTool(descriptor)
		def.Enabled = true
		if _, exists := previous[def.ID]; exists {
			result.Updated++
		} else {
			result.Registered++
		}
		if err := s.registry.Replace(ctx, def); err != nil {
			return result, fmt.Errorf("MCP tool sync %s: %w", def.ID, err)
		}
		seen[def.ID] = struct{}{}
	}

	for toolID := range previous {
		if _, exists := seen[toolID]; exists {
			continue
		}
		if err := s.registry.Unregister(ctx, toolID); err != nil {
			return result, fmt.Errorf("MCP tool unsync %s: %w", toolID, err)
		}
		result.Removed++
	}
	return result, nil
}

func (s *acquisitionMCPToolSync) ListMCPTools(ctx context.Context, serverID string) ([]capability.MCPToolDescriptor, error) {
	// Discovery is owned by MCPRuntimeConnectPort. This method is retained for
	// the compatibility interface and intentionally does not synthesize tools
	// from registry state.
	return nil, nil
}
