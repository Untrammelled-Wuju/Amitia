package main

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/u-ai/backend/internal/extension/kernel/capability"
	mcpclient "github.com/u-ai/backend/internal/mcp/client"
	mcpmanager "github.com/u-ai/backend/internal/mcp/manager"
)

func makeKernelMCPCaller(mgr *mcpmanager.Manager) capability.MCPCallFunc {
	return func(ctx context.Context, serverID string, toolName string, input json.RawMessage) (json.RawMessage, error) {
		if mgr == nil {
			return nil, fmt.Errorf("MCP manager not configured")
		}

		var arguments any
		if err := json.Unmarshal(input, &arguments); err != nil {
			arguments = map[string]any{}
		}

		result, err := mgr.Call(ctx, serverID, "tools/call", map[string]any{
			"name":      toolName,
			"arguments": arguments,
		}, mcpclient.CallOptions{})
		if err != nil {
			return nil, err
		}

		return result, nil
	}
}

func makeKernelMCPHealth(mgr *mcpmanager.Manager) capability.MCPHealthFunc {
	return func(ctx context.Context, serverID string) capability.HealthStatus {
		if mgr == nil {
			return capability.HealthUnknown
		}
		_, ok := mgr.Connection(serverID)
		if !ok {
			return capability.HealthUnknown
		}
		return capability.HealthReady
	}
}
