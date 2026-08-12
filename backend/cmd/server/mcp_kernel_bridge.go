package main

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/capability"
	legacymcp "github.com/u-ai/backend/internal/mcp"
	"github.com/u-ai/backend/internal/extension/kernel/mcp"
	mcpclient "github.com/u-ai/backend/internal/mcp/client"
	mcpmanager "github.com/u-ai/backend/internal/mcp/manager"
)

// MCPConnectionCaller is the abstract interface for MCP tool calls.
// Both Legacy adapter and Canonical stdio connector implement this interface.
type MCPConnectionCaller interface {
	Call(
		ctx context.Context,
		serverID string,
		method string,
		params any,
	) (json.RawMessage, error)
}

// LegacyMCPCallerAdapter wraps Legacy mcpmanager.Manager to implement MCPConnectionCaller.
// migration-only: temporary compatibility adapter
// remove at step 65 cutover
type LegacyMCPCallerAdapter struct {
	mgr *mcpmanager.Manager
}

// NewLegacyMCPCallerAdapter creates a new adapter for Legacy MCP Manager.
func NewLegacyMCPCallerAdapter(mgr *mcpmanager.Manager) *LegacyMCPCallerAdapter {
	return &LegacyMCPCallerAdapter{mgr: mgr}
}

// Call implements MCPConnectionCaller.
func (a *LegacyMCPCallerAdapter) Call(ctx context.Context, serverID string, method string, params any) (json.RawMessage, error) {
	if a.mgr == nil {
		return nil, fmt.Errorf("MCP manager not configured")
	}
	return a.mgr.Call(ctx, serverID, method, params, mcpclient.CallOptions{})
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
// The caller parameter abstracts over Legacy and Canonical implementations.
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

// makeLegacyMCPHealth creates an MCPHealthFunc from Legacy Manager (for backward compatibility).
func makeLegacyMCPHealth(mgr *mcpmanager.Manager) capability.MCPHealthFunc {
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

type mcpRemoteTask struct {
	TaskID        string          `json:"taskId"`
	Status        string          `json:"status"`
	StatusMessage string          `json:"statusMessage"`
	Result        json.RawMessage `json:"result"`
	ExpiresAt     string          `json:"expiresAt"`
}

type mcpRemoteCallResult struct {
	Content []json.RawMessage `json:"content"`
	IsError bool              `json:"isError"`
	Task    *mcpRemoteTask    `json:"task,omitempty"`
}

type mcpPostProcessor struct {
	repo *legacymcp.Repository
}

func newMCPPostProcessor(repo *legacymcp.Repository) *mcpPostProcessor {
	return &mcpPostProcessor{repo: repo}
}

func (p *mcpPostProcessor) AfterExecute(ctx context.Context, serverID string, invocation capability.ToolInvocationContext, raw json.RawMessage) {
	if p.repo == nil {
		return
	}

	p.handleRemoteTask(ctx, serverID, invocation, raw)
}

func (p *mcpPostProcessor) handleRemoteTask(ctx context.Context, serverID string, invocation capability.ToolInvocationContext, raw json.RawMessage) {
	if len(raw) == 0 {
		return
	}

	var remoteResult mcpRemoteCallResult
	if json.Unmarshal(raw, &remoteResult) != nil {
		return
	}

	if remoteResult.Task == nil || remoteResult.Task.TaskID == "" || !validMCPRemoteTaskStatus(remoteResult.Task.Status) {
		return
	}

	enabled, _, capabilityErr := p.repo.ServerCapabilityEnabled(ctx, serverID, "tasks")
	if capabilityErr != nil || !enabled {
		return
	}

	taskResult := remoteResult.Task.Result
	if len(taskResult) == 0 {
		taskResult = json.RawMessage(`{}`)
	}
	if len(taskResult) > 2<<20 {
		taskResult = json.RawMessage(`{"truncated":true}`)
	}

	expires := time.Now().Add(24 * time.Hour)
	if parsed, parseErr := time.Parse(time.RFC3339Nano, remoteResult.Task.ExpiresAt); parseErr == nil && parsed.After(time.Now()) && parsed.Before(time.Now().Add(7*24*time.Hour)) {
		expires = parsed
	}

	_ = p.repo.UpsertTask(ctx, legacymcp.Task{
		ServerID:      serverID,
		RemoteTaskID:  remoteResult.Task.TaskID,
		CharacterID:   invocation.CharacterID,
		RunID:         invocation.InvocationID,
		Status:        remoteResult.Task.Status,
		StatusMessage: remoteResult.Task.StatusMessage,
		ResultJSON:    string(taskResult),
		ExpiresAt:     expires.UTC().Format(time.RFC3339Nano),
	})
}

func validMCPRemoteTaskStatus(value string) bool {
	return value == "working" || value == "input_required" || value == "completed" || value == "failed" || value == "cancelled"
}
