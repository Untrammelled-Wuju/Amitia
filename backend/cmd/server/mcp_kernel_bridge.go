package main

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/capability"
	"github.com/u-ai/backend/internal/mcp"
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
	repo *mcp.Repository
}

func newMCPPostProcessor(repo *mcp.Repository) *mcpPostProcessor {
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

	_ = p.repo.UpsertTask(ctx, mcp.Task{
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
