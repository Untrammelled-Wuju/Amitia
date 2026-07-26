package mcp

import (
	"context"

	"github.com/u-ai/backend/internal/extension/kernel/resource"
)

type MCPResourceIntegration struct {
	ownership resource.ResourceOwnershipService
}

func NewMCPResourceIntegration(ownership resource.ResourceOwnershipService) *MCPResourceIntegration {
	return &MCPResourceIntegration{ownership: ownership}
}

func (i *MCPResourceIntegration) RegisterServer(ctx context.Context, serverID, extensionID string) error {
	if i.ownership == nil {
		return nil
	}
	owner := resource.NewSystemOwner()
	if extensionID != "" {
		owner = resource.NewExtensionOwner(extensionID)
	}
	return i.ownership.Register(ctx, &resource.ResourceRecord{
		ResourceID:   serverID,
		ResourceType: resource.ResourceMCPServer,
		Owner:        owner,
		State:        resource.StateActive,
	})
}

func (i *MCPResourceIntegration) RegisterTool(ctx context.Context, toolID, serverID string) error {
	if i.ownership == nil {
		return nil
	}
	_ = i.ownership.Register(ctx, &resource.ResourceRecord{
		ResourceID:   toolID,
		ResourceType: resource.ResourceMCPTool,
		Owner:        resource.NewExtensionOwner(serverID),
		State:        resource.StateActive,
	})
	_ = i.ownership.AddReference(ctx, resource.ResourceReference{
		ReferenceID:      "mcp_tool_ref_" + toolID,
		SourceResourceID: serverID,
		TargetResourceID: toolID,
		ReferenceType:    resource.RefContains,
		Required:         true,
		OwnershipEffect:  resource.EffectCascadeDelete,
	})
	return nil
}

func (i *MCPResourceIntegration) RegisterProcess(ctx context.Context, processID, serverID string) error {
	if i.ownership == nil {
		return nil
	}
	_ = i.ownership.Register(ctx, &resource.ResourceRecord{
		ResourceID:   processID,
		ResourceType: resource.ResourceProcess,
		Owner:        resource.NewExtensionOwner(serverID),
		State:        resource.StateActive,
	})
	_ = i.ownership.AddReference(ctx, resource.ResourceReference{
		ReferenceID:      "mcp_proc_ref_" + processID,
		SourceResourceID: serverID,
		TargetResourceID: processID,
		ReferenceType:    resource.RefOwnedBy,
		Required:         false,
		OwnershipEffect:  resource.EffectRetainTarget,
	})
	return nil
}

func (i *MCPResourceIntegration) RegisterConnection(ctx context.Context, connectionID, serverID string) error {
	if i.ownership == nil {
		return nil
	}
	_ = i.ownership.Register(ctx, &resource.ResourceRecord{
		ResourceID:   connectionID,
		ResourceType: resource.ResourceConnection,
		Owner:        resource.NewExtensionOwner(serverID),
		State:        resource.StateActive,
	})
	return nil
}

func (i *MCPResourceIntegration) DeregisterServer(ctx context.Context, serverID string) error {
	if i.ownership == nil {
		return nil
	}
	return i.ownership.UpdateState(ctx, serverID, resource.StateDeleted)
}

func (i *MCPResourceIntegration) DeregisterConnection(ctx context.Context, connectionID string) error {
	if i.ownership == nil {
		return nil
	}
	return i.ownership.UpdateState(ctx, connectionID, resource.StateDeleted)
}
