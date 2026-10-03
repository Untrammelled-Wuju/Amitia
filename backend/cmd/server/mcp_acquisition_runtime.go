package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/u-ai/backend/internal/extension/kernel/capability"
	"github.com/u-ai/backend/internal/extension/kernel/capability/acquisition"
	kernelmcp "github.com/u-ai/backend/internal/extension/kernel/mcp"
	"github.com/u-ai/backend/internal/mcp/discovery"
)

// mcpAcquisitionRuntime bridges capability acquisition to the canonical MCP
// connection registries. Installation state remains owned by MCPLifecycle;
// this bridge owns the real transport handshake and tools/list discovery.
type mcpAcquisitionRuntime struct {
	stdio  *kernelmcp.CanonicalStdioRegistry
	remote *kernelmcp.CanonicalRemoteRegistry
}

func newMCPAcquisitionRuntime(stdio *kernelmcp.CanonicalStdioRegistry, remote *kernelmcp.CanonicalRemoteRegistry) acquisition.MCPRuntimeConnectPort {
	return &mcpAcquisitionRuntime{stdio: stdio, remote: remote}
}

func (r *mcpAcquisitionRuntime) ConnectAndDiscover(ctx context.Context, req acquisition.MCPRuntimeConnectRequest) ([]capability.MCPToolDescriptor, error) {
	if req.ServerID == "" {
		return nil, fmt.Errorf("MCP runtime: serverId is required")
	}

	var tools []discovery.Tool
	transport := strings.ToLower(strings.TrimSpace(req.Transport))
	switch transport {
	case "streamable_http", "sse", "remote", "http", "https":
		if r.remote == nil {
			return nil, fmt.Errorf("MCP runtime: remote registry not configured")
		}
		if req.Command == "" {
			return nil, fmt.Errorf("MCP runtime: remote endpoint is required")
		}
		conn, err := r.remote.StartOrGet(ctx, kernelmcp.MCPRemoteSpec{
			ServerID:        req.ServerID,
			Endpoint:        req.Command,
			AllowLoopback:   false,
			AllowPrivate:    false,
			AllowPublicHTTP: false,
			MaxRedirects:    3,
		})
		if err != nil {
			return nil, fmt.Errorf("MCP runtime remote connect: %w", err)
		}
		tools, err = conn.ListTools(ctx)
		if err != nil {
			_ = r.remote.Close(ctx, req.ServerID)
			return nil, fmt.Errorf("MCP runtime remote discovery: %w", err)
		}
	default:
		if r.stdio == nil {
			return nil, fmt.Errorf("MCP runtime: stdio registry not configured")
		}
		if req.Command == "" {
			return nil, fmt.Errorf("MCP runtime: stdio command is required")
		}
		conn, err := r.stdio.StartOrGet(ctx, kernelmcp.MCPStdioSpec{
			ServerID: req.ServerID,
			Command:  req.Command,
			Args:     req.Args,
			Env:      req.Env,
		})
		if err != nil {
			return nil, fmt.Errorf("MCP runtime stdio connect: %w", err)
		}
		tools, err = conn.ListTools(ctx)
		if err != nil {
			_ = r.stdio.Close(ctx, req.ServerID)
			return nil, fmt.Errorf("MCP runtime stdio discovery: %w", err)
		}
	}

	descriptors := make([]capability.MCPToolDescriptor, 0, len(tools))
	for _, item := range tools {
		annotations := map[string]any{}
		if len(item.Annotations) > 0 {
			_ = json.Unmarshal(item.Annotations, &annotations)
		}
		descriptors = append(descriptors, capability.MCPToolDescriptor{
			ServerID:     req.ServerID,
			ServerName:   req.ServerID,
			Name:         item.Name,
			Title:        item.Title,
			Description:  item.Description,
			InputSchema:  item.InputSchema,
			OutputSchema: item.OutputSchema,
			Annotations:  annotations,
		})
	}
	return descriptors, nil
}

func (r *mcpAcquisitionRuntime) Disconnect(ctx context.Context, serverID string) error {
	var firstErr error
	if r.stdio != nil {
		if err := r.stdio.Close(ctx, serverID); err != nil {
			firstErr = err
		}
	}
	if r.remote != nil {
		if err := r.remote.Close(ctx, serverID); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}
