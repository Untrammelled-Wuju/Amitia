package main

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/u-ai/backend/internal/extension/kernel/capability"
	"github.com/u-ai/backend/internal/extension/kernel/mcp"
)

// MCPConnectionCaller is the abstract interface for MCP tool calls.
type MCPConnectionCaller interface {
	Call(
		ctx context.Context,
		serverID string,
		method string,
		params any,
	) (json.RawMessage, error)
}

// CanonicalMCPCaller implements MCPConnectionCaller by dispatching to
// both the stdio and remote registries. It is the single production
// MCP caller wired into the Extension Kernel MCP adapter.
type CanonicalMCPCaller struct {
	stdio  *CanonicalStdioCaller
	remote *CanonicalRemoteCaller
}

// NewCanonicalMCPCaller creates a caller backed by both registries.
func NewCanonicalMCPCaller(stdio *mcp.CanonicalStdioRegistry, remote *mcp.CanonicalRemoteRegistry) *CanonicalMCPCaller {
	return &CanonicalMCPCaller{
		stdio:  &CanonicalStdioCaller{registry: stdio},
		remote: &CanonicalRemoteCaller{registry: remote},
	}
}

// Call implements MCPConnectionCaller. Tries stdio first, falls back to remote.
func (c *CanonicalMCPCaller) Call(ctx context.Context, serverID string, method string, params any) (json.RawMessage, error) {
	if c.stdio != nil {
		if conn, ok := c.stdio.registry.Get(serverID); ok {
			return conn.Call(ctx, method, params)
		}
	}
	if c.remote != nil {
		if conn, ok := c.remote.registry.Get(serverID); ok {
			return conn.Call(ctx, method, params)
		}
	}
	return nil, fmt.Errorf("MCP server not found: %s", serverID)
}

// CanonicalStdioCaller implements MCPConnectionCaller for Extension Kernel stdio connections.
type CanonicalStdioCaller struct {
	registry *mcp.CanonicalStdioRegistry
}

// NewCanonicalStdioCaller creates a new caller for Canonical stdio connections.
func NewCanonicalStdioCaller(registry *mcp.CanonicalStdioRegistry) *CanonicalStdioCaller {
	return &CanonicalStdioCaller{registry: registry}
}

// Call implements MCPConnectionCaller.
func (c *CanonicalStdioCaller) Call(ctx context.Context, serverID string, method string, params any) (json.RawMessage, error) {
	conn, ok := c.registry.Get(serverID)
	if !ok {
		return nil, fmt.Errorf("MCP server not found: %s", serverID)
	}
	return conn.Call(ctx, method, params)
}

// CanonicalRemoteCaller implements MCPConnectionCaller for Extension Kernel remote connections.
type CanonicalRemoteCaller struct {
	registry *mcp.CanonicalRemoteRegistry
}

// NewCanonicalRemoteCaller creates a new caller for Canonical remote connections.
func NewCanonicalRemoteCaller(registry *mcp.CanonicalRemoteRegistry) *CanonicalRemoteCaller {
	return &CanonicalRemoteCaller{registry: registry}
}

// Call implements MCPConnectionCaller.
func (c *CanonicalRemoteCaller) Call(ctx context.Context, serverID string, method string, params any) (json.RawMessage, error) {
	conn, ok := c.registry.Get(serverID)
	if !ok {
		return nil, fmt.Errorf("MCP server not found: %s", serverID)
	}
	return conn.Call(ctx, method, params)
}

// makeKernelMCPCaller creates an MCPCallFunc from the abstract MCPConnectionCaller.
func makeKernelMCPCaller(caller MCPConnectionCaller) capability.MCPCallFunc {
	return func(ctx context.Context, serverID string, toolName string, input json.RawMessage) (json.RawMessage, error) {
		if caller == nil {
			return nil, fmt.Errorf("MCP caller not configured")
		}

		var arguments any
		if err := json.Unmarshal(input, &arguments); err != nil {
			arguments = map[string]any{}
		}

		result, err := caller.Call(ctx, serverID, "tools/call", map[string]any{
			"name":      toolName,
			"arguments": arguments,
		})
		if err != nil {
			return nil, err
		}

		return result, nil
	}
}

// makeKernelMCPHealth creates an MCPHealthFunc from the abstract MCPConnectionCaller.
func makeKernelMCPHealth(caller MCPConnectionCaller) capability.MCPHealthFunc {
	return func(ctx context.Context, serverID string) capability.HealthStatus {
		if caller == nil {
			return capability.HealthUnknown
		}
		_, err := caller.Call(ctx, serverID, "ping", map[string]any{})
		if err != nil {
			return capability.HealthUnknown
		}
		return capability.HealthReady
	}
}
