package workspace

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/capability"
)

const (
	ToolDomain = "workspace"
)

var workspaceRuntime = capability.RuntimeBinding{
	RuntimeType: capability.RuntimeTypeWorkspace,
	RuntimeID:   "default",
}

func BuildWorkspaceTools() []capability.ToolDefinition {
	return []capability.ToolDefinition{
		buildListTool(),
		buildStatTool(),
		buildReadTool(),
		buildWriteTool(),
		buildMkdirTool(),
		buildRenameTool(),
		buildMoveTool(),
		buildCopyTool(),
		buildDeleteTool(),
		buildSearchTool(),
		buildPrecisePatchTool(),
		buildPreciseDiffTool(),
		buildPreciseReplaceTool(),
	}
}

func buildListTool() capability.ToolDefinition {
	inputSchema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"uri": {"type": "string"},
			"limit": {"type": "integer", "minimum": 1, "maximum": 500},
			"cursor": {"type": "string"}
		},
		"required": ["uri"],
		"additionalProperties": false
	}`)
	outputSchema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"entries": {"type": "array", "items": {"type": "object"}},
			"nextCursor": {"type": "string"},
			"hasMore": {"type": "boolean"}
		}
	}`)
	return capability.ToolDefinition{
		ID:           "workspace.list",
		ModelName:    "workspace.list",
		Source:       capability.ToolSourceBuiltin,
		Name:         "Workspace List",
		Description:  "List entries in a workspace directory.",
		InputSchema:  inputSchema,
		OutputSchema: outputSchema,
		Permissions: []capability.PermissionRequirement{
			{Capability: "workspace.read", Risk: "low"},
		},
		RiskLevel:      capability.RiskLow,
		SideEffect:     capability.SideEffectReadOnly,
		HasSideEffects: false,
		Idempotent:     true,
		Retryable:      true,
		TimeoutMS:      10000,
		ToolVersion:    capability.ToolVersion{SchemaVersion: 1, Revision: "b55-workspace-v1"},
		ExecutionPolicy: capability.ToolExecutionPolicy{
			Timeout:          10 * time.Second,
			MaxConcurrency:   10,
			Idempotent:       true,
			ApprovalRequired: false,
			AllowBackground:  true,
			MaxDepth:         0,
		},
		ResultPolicy: capability.ToolResultPolicy{
			SanitizeError:  true,
			MaxOutputBytes: 65536,
			Streaming:      capability.ToolStreamingPolicy{Enabled: false},
		},
		Runtime: capability.RuntimeBinding{
			RuntimeType: workspaceRuntime.RuntimeType,
			RuntimeID:   workspaceRuntime.RuntimeID,
			HandlerName: "workspace.list",
		},
		Enabled: true,
	}
}

func buildStatTool() capability.ToolDefinition {
	inputSchema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"uri": {"type": "string"}
		},
		"required": ["uri"],
		"additionalProperties": false
	}`)
	outputSchema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"entry": {"type": "object"}
		}
	}`)
	return capability.ToolDefinition{
		ID:           "workspace.stat",
		ModelName:    "workspace.stat",
		Source:       capability.ToolSourceBuiltin,
		Name:         "Workspace Stat",
		Description:  "Get metadata of a workspace entry.",
		InputSchema:  inputSchema,
		OutputSchema: outputSchema,
		Permissions: []capability.PermissionRequirement{
			{Capability: "workspace.read", Risk: "low"},
		},
		RiskLevel:      capability.RiskLow,
		SideEffect:     capability.SideEffectReadOnly,
		HasSideEffects: false,
		Idempotent:     true,
		Retryable:      true,
		TimeoutMS:      5000,
		ToolVersion:    capability.ToolVersion{SchemaVersion: 1, Revision: "b55-workspace-v1"},
		ExecutionPolicy: capability.ToolExecutionPolicy{
			Timeout:          5 * time.Second,
			MaxConcurrency:   10,
			Idempotent:       true,
			ApprovalRequired: false,
			AllowBackground:  true,
			MaxDepth:         0,
		},
		ResultPolicy: capability.ToolResultPolicy{
			SanitizeError:  true,
			MaxOutputBytes: 4096,
			Streaming:      capability.ToolStreamingPolicy{Enabled: false},
		},
		Runtime: capability.RuntimeBinding{
			RuntimeType: workspaceRuntime.RuntimeType,
			RuntimeID:   workspaceRuntime.RuntimeID,
			HandlerName: "workspace.stat",
		},
		Enabled: true,
	}
}

func buildReadTool() capability.ToolDefinition {
	inputSchema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"uri": {"type": "string"},
			"offset": {"type": "integer", "minimum": 0},
			"maxBytes": {"type": "integer", "minimum": 1}
		},
		"required": ["uri"],
		"additionalProperties": false
	}`)
	outputSchema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"content": {"type": "string"},
			"isText": {"type": "boolean"},
			"resource": {"type": "string"}
		}
	}`)
	return capability.ToolDefinition{
		ID:           "workspace.read",
		ModelName:    "workspace.read",
		Source:       capability.ToolSourceBuiltin,
		Name:         "Workspace Read",
		Description:  "Read content from a workspace file.",
		InputSchema:  inputSchema,
		OutputSchema: outputSchema,
		Permissions: []capability.PermissionRequirement{
			{Capability: "workspace.read", Risk: "low"},
		},
		RiskLevel:      capability.RiskLow,
		SideEffect:     capability.SideEffectReadOnly,
		HasSideEffects: false,
		Idempotent:     true,
		Retryable:      true,
		TimeoutMS:      15000,
		ToolVersion:    capability.ToolVersion{SchemaVersion: 1, Revision: "b55-workspace-v1"},
		ExecutionPolicy: capability.ToolExecutionPolicy{
			Timeout:          15 * time.Second,
			MaxConcurrency:   5,
			Idempotent:       true,
			ApprovalRequired: false,
			AllowBackground:  true,
			MaxDepth:         0,
		},
		ResultPolicy: capability.ToolResultPolicy{
			SanitizeError:  true,
			MaxOutputBytes: 1048576,
			Streaming:      capability.ToolStreamingPolicy{Enabled: false},
		},
		Runtime: capability.RuntimeBinding{
			RuntimeType: workspaceRuntime.RuntimeType,
			RuntimeID:   workspaceRuntime.RuntimeID,
			HandlerName: "workspace.read",
		},
		Enabled: true,
	}
}

func buildWriteTool() capability.ToolDefinition {
	inputSchema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"uri": {"type": "string"},
			"text": {"type": "string"},
			"sourceUri": {"type": "string"},
			"overwrite": {"type": "boolean"}
		},
		"required": ["uri"],
		"additionalProperties": false
	}`)
	outputSchema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"entry": {"type": "object"}
		}
	}`)
	return capability.ToolDefinition{
		ID:           "workspace.write",
		ModelName:    "workspace.write",
		Source:       capability.ToolSourceBuiltin,
		Name:         "Workspace Write",
		Description:  "Write content to a workspace file.",
		InputSchema:  inputSchema,
		OutputSchema: outputSchema,
		Permissions: []capability.PermissionRequirement{
			{Capability: "workspace.write", Risk: "medium"},
		},
		RiskLevel:      capability.RiskMedium,
		SideEffect:     capability.SideEffectWrite,
		HasSideEffects: true,
		Idempotent:     false,
		Retryable:      false,
		TimeoutMS:      30000,
		ToolVersion:    capability.ToolVersion{SchemaVersion: 1, Revision: "b55-workspace-v1"},
		ExecutionPolicy: capability.ToolExecutionPolicy{
			Timeout:          30 * time.Second,
			MaxConcurrency:   3,
			Idempotent:       false,
			ApprovalRequired: true,
			AllowBackground:  false,
			MaxDepth:         0,
		},
		ResultPolicy: capability.ToolResultPolicy{
			SanitizeError:  true,
			MaxOutputBytes: 4096,
			Streaming:      capability.ToolStreamingPolicy{Enabled: false},
		},
		Runtime: capability.RuntimeBinding{
			RuntimeType: workspaceRuntime.RuntimeType,
			RuntimeID:   workspaceRuntime.RuntimeID,
			HandlerName: "workspace.write",
		},
		Enabled: true,
	}
}

func buildMkdirTool() capability.ToolDefinition {
	inputSchema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"uri": {"type": "string"}
		},
		"required": ["uri"],
		"additionalProperties": false
	}`)
	outputSchema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"entry": {"type": "object"}
		}
	}`)
	return capability.ToolDefinition{
		ID:           "workspace.mkdir",
		ModelName:    "workspace.mkdir",
		Source:       capability.ToolSourceBuiltin,
		Name:         "Workspace Mkdir",
		Description:  "Create a directory in the workspace.",
		InputSchema:  inputSchema,
		OutputSchema: outputSchema,
		Permissions: []capability.PermissionRequirement{
			{Capability: "workspace.write", Risk: "medium"},
		},
		RiskLevel:      capability.RiskMedium,
		SideEffect:     capability.SideEffectWrite,
		HasSideEffects: true,
		Idempotent:     false,
		Retryable:      false,
		TimeoutMS:      10000,
		ToolVersion:    capability.ToolVersion{SchemaVersion: 1, Revision: "b55-workspace-v1"},
		ExecutionPolicy: capability.ToolExecutionPolicy{
			Timeout:          10 * time.Second,
			MaxConcurrency:   3,
			Idempotent:       false,
			ApprovalRequired: true,
			AllowBackground:  false,
			MaxDepth:         0,
		},
		ResultPolicy: capability.ToolResultPolicy{
			SanitizeError:  true,
			MaxOutputBytes: 4096,
			Streaming:      capability.ToolStreamingPolicy{Enabled: false},
		},
		Runtime: capability.RuntimeBinding{
			RuntimeType: workspaceRuntime.RuntimeType,
			RuntimeID:   workspaceRuntime.RuntimeID,
			HandlerName: "workspace.mkdir",
		},
		Enabled: true,
	}
}

func buildRenameTool() capability.ToolDefinition {
	inputSchema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"uri": {"type": "string"},
			"newName": {"type": "string"}
		},
		"required": ["uri", "newName"],
		"additionalProperties": false
	}`)
	outputSchema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"entry": {"type": "object"}
		}
	}`)
	return capability.ToolDefinition{
		ID:           "workspace.rename",
		ModelName:    "workspace.rename",
		Source:       capability.ToolSourceBuiltin,
		Name:         "Workspace Rename",
		Description:  "Rename a workspace entry.",
		InputSchema:  inputSchema,
		OutputSchema: outputSchema,
		Permissions: []capability.PermissionRequirement{
			{Capability: "workspace.write", Risk: "medium"},
		},
		RiskLevel:      capability.RiskMedium,
		SideEffect:     capability.SideEffectWrite,
		HasSideEffects: true,
		Idempotent:     false,
		Retryable:      false,
		TimeoutMS:      10000,
		ToolVersion:    capability.ToolVersion{SchemaVersion: 1, Revision: "b55-workspace-v1"},
		ExecutionPolicy: capability.ToolExecutionPolicy{
			Timeout:          10 * time.Second,
			MaxConcurrency:   3,
			Idempotent:       false,
			ApprovalRequired: true,
			AllowBackground:  false,
			MaxDepth:         0,
		},
		ResultPolicy: capability.ToolResultPolicy{
			SanitizeError:  true,
			MaxOutputBytes: 4096,
			Streaming:      capability.ToolStreamingPolicy{Enabled: false},
		},
		Runtime: capability.RuntimeBinding{
			RuntimeType: workspaceRuntime.RuntimeType,
			RuntimeID:   workspaceRuntime.RuntimeID,
			HandlerName: "workspace.rename",
		},
		Enabled: true,
	}
}

func buildMoveTool() capability.ToolDefinition {
	inputSchema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"sourceUri": {"type": "string"},
			"destinationDirUri": {"type": "string"}
		},
		"required": ["sourceUri", "destinationDirUri"],
		"additionalProperties": false
	}`)
	outputSchema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"entry": {"type": "object"}
		}
	}`)
	return capability.ToolDefinition{
		ID:           "workspace.move",
		ModelName:    "workspace.move",
		Source:       capability.ToolSourceBuiltin,
		Name:         "Workspace Move",
		Description:  "Move a workspace entry within the same mount.",
		InputSchema:  inputSchema,
		OutputSchema: outputSchema,
		Permissions: []capability.PermissionRequirement{
			{Capability: "workspace.write", Risk: "medium"},
		},
		RiskLevel:      capability.RiskMedium,
		SideEffect:     capability.SideEffectWrite,
		HasSideEffects: true,
		Idempotent:     false,
		Retryable:      false,
		TimeoutMS:      15000,
		ToolVersion:    capability.ToolVersion{SchemaVersion: 1, Revision: "b55-workspace-v1"},
		ExecutionPolicy: capability.ToolExecutionPolicy{
			Timeout:          15 * time.Second,
			MaxConcurrency:   3,
			Idempotent:       false,
			ApprovalRequired: true,
			AllowBackground:  false,
			MaxDepth:         0,
		},
		ResultPolicy: capability.ToolResultPolicy{
			SanitizeError:  true,
			MaxOutputBytes: 4096,
			Streaming:      capability.ToolStreamingPolicy{Enabled: false},
		},
		Runtime: capability.RuntimeBinding{
			RuntimeType: workspaceRuntime.RuntimeType,
			RuntimeID:   workspaceRuntime.RuntimeID,
			HandlerName: "workspace.move",
		},
		Enabled: true,
	}
}

func buildCopyTool() capability.ToolDefinition {
	inputSchema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"sourceUri": {"type": "string"},
			"destinationDirUri": {"type": "string"}
		},
		"required": ["sourceUri", "destinationDirUri"],
		"additionalProperties": false
	}`)
	outputSchema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"entry": {"type": "object"}
		}
	}`)
	return capability.ToolDefinition{
		ID:           "workspace.copy",
		ModelName:    "workspace.copy",
		Source:       capability.ToolSourceBuiltin,
		Name:         "Workspace Copy",
		Description:  "Copy a workspace entry within the same mount.",
		InputSchema:  inputSchema,
		OutputSchema: outputSchema,
		Permissions: []capability.PermissionRequirement{
			{Capability: "workspace.write", Risk: "medium"},
		},
		RiskLevel:      capability.RiskMedium,
		SideEffect:     capability.SideEffectWrite,
		HasSideEffects: true,
		Idempotent:     false,
		Retryable:      false,
		TimeoutMS:      15000,
		ToolVersion:    capability.ToolVersion{SchemaVersion: 1, Revision: "b55-workspace-v1"},
		ExecutionPolicy: capability.ToolExecutionPolicy{
			Timeout:          15 * time.Second,
			MaxConcurrency:   3,
			Idempotent:       false,
			ApprovalRequired: true,
			AllowBackground:  false,
			MaxDepth:         0,
		},
		ResultPolicy: capability.ToolResultPolicy{
			SanitizeError:  true,
			MaxOutputBytes: 4096,
			Streaming:      capability.ToolStreamingPolicy{Enabled: false},
		},
		Runtime: capability.RuntimeBinding{
			RuntimeType: workspaceRuntime.RuntimeType,
			RuntimeID:   workspaceRuntime.RuntimeID,
			HandlerName: "workspace.copy",
		},
		Enabled: true,
	}
}

func buildDeleteTool() capability.ToolDefinition {
	inputSchema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"uri": {"type": "string"},
			"recursive": {"type": "boolean"}
		},
		"required": ["uri"],
		"additionalProperties": false
	}`)
	outputSchema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"deleted": {"type": "boolean"}
		}
	}`)
	return capability.ToolDefinition{
		ID:           "workspace.delete",
		ModelName:    "workspace.delete",
		Source:       capability.ToolSourceBuiltin,
		Name:         "Workspace Delete",
		Description:  "Delete a workspace entry.",
		InputSchema:  inputSchema,
		OutputSchema: outputSchema,
		Permissions: []capability.PermissionRequirement{
			{Capability: "workspace.write", Risk: "high"},
		},
		RiskLevel:      capability.RiskHigh,
		SideEffect:     capability.SideEffectWrite,
		HasSideEffects: true,
		Idempotent:     false,
		Retryable:      false,
		TimeoutMS:      15000,
		ToolVersion:    capability.ToolVersion{SchemaVersion: 1, Revision: "b55-workspace-v1"},
		ExecutionPolicy: capability.ToolExecutionPolicy{
			Timeout:          15 * time.Second,
			MaxConcurrency:   3,
			Idempotent:       false,
			ApprovalRequired: true,
			AllowBackground:  false,
			MaxDepth:         0,
		},
		ResultPolicy: capability.ToolResultPolicy{
			SanitizeError:  true,
			MaxOutputBytes: 4096,
			Streaming:      capability.ToolStreamingPolicy{Enabled: false},
		},
		Runtime: capability.RuntimeBinding{
			RuntimeType: workspaceRuntime.RuntimeType,
			RuntimeID:   workspaceRuntime.RuntimeID,
			HandlerName: "workspace.delete",
		},
		Enabled: true,
	}
}

func buildSearchTool() capability.ToolDefinition {
	inputSchema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"workspaceId": {"type": "string"},
			"query": {"type": "string"},
			"regex": {"type": "boolean"},
			"includeGlobs": {"type": "array", "items": {"type": "string"}},
			"excludeGlobs": {"type": "array", "items": {"type": "string"}},
			"maxResults": {"type": "integer", "minimum": 1, "maximum": 1000},
			"contextBefore": {"type": "integer", "minimum": 0, "maximum": 10},
			"contextAfter": {"type": "integer", "minimum": 0, "maximum": 10}
		},
		"required": ["workspaceId", "query"],
		"additionalProperties": false
	}`)
	outputSchema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"matches": {"type": "array", "items": {"type": "object"}},
			"total": {"type": "integer"},
			"truncated": {"type": "boolean"}
		}
	}`)
	return capability.ToolDefinition{
		ID:           "workspace.search",
		ModelName:    "workspace.search",
		Source:       capability.ToolSourceBuiltin,
		Name:         "Workspace Search",
		Description:  "Search for literal or regex patterns across workspace files.",
		InputSchema:  inputSchema,
		OutputSchema: outputSchema,
		Permissions: []capability.PermissionRequirement{
			{Capability: "workspace.read", Risk: "low"},
		},
		RiskLevel:      capability.RiskLow,
		SideEffect:     capability.SideEffectReadOnly,
		HasSideEffects: false,
		Idempotent:     true,
		Retryable:      true,
		TimeoutMS:      30000,
		ToolVersion:    capability.ToolVersion{SchemaVersion: 1, Revision: "b55-workspace-v1"},
		ExecutionPolicy: capability.ToolExecutionPolicy{
			Timeout:          30 * time.Second,
			MaxConcurrency:   5,
			Idempotent:       true,
			ApprovalRequired: false,
			AllowBackground:  true,
			MaxDepth:         0,
		},
		ResultPolicy: capability.ToolResultPolicy{
			SanitizeError:  true,
			MaxOutputBytes: 131072,
			Streaming:      capability.ToolStreamingPolicy{Enabled: false},
		},
		Runtime: capability.RuntimeBinding{
			RuntimeType: workspaceRuntime.RuntimeType,
			RuntimeID:   workspaceRuntime.RuntimeID,
			HandlerName: "workspace.search",
		},
		Enabled: true,
	}
}

func buildPrecisePatchTool() capability.ToolDefinition {
	inputSchema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"workspaceId": {"type": "string"},
			"filePath": {"type": "string"},
			"baseSha256": {"type": "string"},
			"patch": {"type": "string"}
		},
		"required": ["workspaceId", "filePath", "patch"],
		"additionalProperties": false
	}`)
	outputSchema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"applied": {"type": "boolean"},
			"filePath": {"type": "string"},
			"newSha256": {"type": "string"}
		}
	}`)
	return capability.ToolDefinition{
		ID:           "workspace.patch",
		ModelName:    "workspace.patch",
		Source:       capability.ToolSourceBuiltin,
		Name:         "Workspace Patch",
		Description:  "Apply a unified-diff patch to a workspace file with integrity verification.",
		InputSchema:  inputSchema,
		OutputSchema: outputSchema,
		Permissions: []capability.PermissionRequirement{
			{Capability: "workspace.write", Risk: "medium"},
		},
		RiskLevel:      capability.RiskMedium,
		SideEffect:     capability.SideEffectWrite,
		HasSideEffects: true,
		Idempotent:     false,
		Retryable:      false,
		TimeoutMS:      30000,
		ToolVersion:    capability.ToolVersion{SchemaVersion: 1, Revision: "b55-workspace-v1"},
		ExecutionPolicy: capability.ToolExecutionPolicy{
			Timeout:          30 * time.Second,
			MaxConcurrency:   3,
			Idempotent:       false,
			ApprovalRequired: true,
			AllowBackground:  false,
			MaxDepth:         0,
		},
		ResultPolicy: capability.ToolResultPolicy{
			SanitizeError:  true,
			MaxOutputBytes: 4096,
			Streaming:      capability.ToolStreamingPolicy{Enabled: false},
		},
		Runtime: capability.RuntimeBinding{
			RuntimeType: workspaceRuntime.RuntimeType,
			RuntimeID:   workspaceRuntime.RuntimeID,
			HandlerName: "workspace.patch",
		},
		Enabled: true,
	}
}

func buildPreciseDiffTool() capability.ToolDefinition {
	inputSchema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"workspaceId": {"type": "string"},
			"beforeFiles": {"type": "object"},
			"afterFiles": {"type": "object"}
		},
		"required": ["workspaceId"],
		"additionalProperties": false
	}`)
	outputSchema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"changedFiles": {"type": "array", "items": {"type": "string"}},
			"unifiedDiff": {"type": "string"},
			"additions": {"type": "integer"},
			"deletions": {"type": "integer"}
		}
	}`)
	return capability.ToolDefinition{
		ID:           "workspace.diff",
		ModelName:    "workspace.diff",
		Source:       capability.ToolSourceBuiltin,
		Name:         "Workspace Diff",
		Description:  "Compute unified diff between before and after file snapshots.",
		InputSchema:  inputSchema,
		OutputSchema: outputSchema,
		Permissions: []capability.PermissionRequirement{
			{Capability: "workspace.read", Risk: "low"},
		},
		RiskLevel:      capability.RiskLow,
		SideEffect:     capability.SideEffectReadOnly,
		HasSideEffects: false,
		Idempotent:     true,
		Retryable:      true,
		TimeoutMS:      15000,
		ToolVersion:    capability.ToolVersion{SchemaVersion: 1, Revision: "b55-workspace-v1"},
		ExecutionPolicy: capability.ToolExecutionPolicy{
			Timeout:          15 * time.Second,
			MaxConcurrency:   5,
			Idempotent:       true,
			ApprovalRequired: false,
			AllowBackground:  true,
			MaxDepth:         0,
		},
		ResultPolicy: capability.ToolResultPolicy{
			SanitizeError:  true,
			MaxOutputBytes: 131072,
			Streaming:      capability.ToolStreamingPolicy{Enabled: false},
		},
		Runtime: capability.RuntimeBinding{
			RuntimeType: workspaceRuntime.RuntimeType,
			RuntimeID:   workspaceRuntime.RuntimeID,
			HandlerName: "workspace.diff",
		},
		Enabled: true,
	}
}

func buildPreciseReplaceTool() capability.ToolDefinition {
	inputSchema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"workspaceId": {"type": "string"},
			"filePath": {"type": "string"},
			"oldText": {"type": "string"},
			"newText": {"type": "string"},
			"expectedOccurrences": {"type": "integer", "minimum": 0}
		},
		"required": ["workspaceId", "filePath", "oldText"],
		"additionalProperties": false
	}`)
	outputSchema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"replaced": {"type": "boolean"},
			"actualOccurrences": {"type": "integer"},
			"filePath": {"type": "string"}
		}
	}`)
	return capability.ToolDefinition{
		ID:           "workspace.replace",
		ModelName:    "workspace.replace",
		Source:       capability.ToolSourceBuiltin,
		Name:         "Workspace Replace",
		Description:  "Perform exact text replacement in a workspace file with occurrence validation.",
		InputSchema:  inputSchema,
		OutputSchema: outputSchema,
		Permissions: []capability.PermissionRequirement{
			{Capability: "workspace.write", Risk: "medium"},
		},
		RiskLevel:      capability.RiskMedium,
		SideEffect:     capability.SideEffectWrite,
		HasSideEffects: true,
		Idempotent:     false,
		Retryable:      false,
		TimeoutMS:      30000,
		ToolVersion:    capability.ToolVersion{SchemaVersion: 1, Revision: "b55-workspace-v1"},
		ExecutionPolicy: capability.ToolExecutionPolicy{
			Timeout:          30 * time.Second,
			MaxConcurrency:   3,
			Idempotent:       false,
			ApprovalRequired: true,
			AllowBackground:  false,
			MaxDepth:         0,
		},
		ResultPolicy: capability.ToolResultPolicy{
			SanitizeError:  true,
			MaxOutputBytes: 4096,
			Streaming:      capability.ToolStreamingPolicy{Enabled: false},
		},
		Runtime: capability.RuntimeBinding{
			RuntimeType: workspaceRuntime.RuntimeType,
			RuntimeID:   workspaceRuntime.RuntimeID,
			HandlerName: "workspace.replace",
		},
		Enabled: true,
	}
}

type ToolDispatcher struct {
	service  *Service
	precise  PreciseEditingService
}

func NewToolDispatcher(service *Service) *ToolDispatcher {
	return &ToolDispatcher{service: service}
}

func (d *ToolDispatcher) SetPreciseService(precise PreciseEditingService) {
	d.precise = precise
}

func (d *ToolDispatcher) Dispatch(ctx context.Context, handlerName string, input json.RawMessage) (json.RawMessage, error) {
	switch handlerName {
	case "workspace.list":
		return d.handleList(ctx, input)
	case "workspace.stat":
		return d.handleStat(ctx, input)
	case "workspace.read":
		return d.handleRead(ctx, input)
	case "workspace.write":
		return d.handleWrite(ctx, input)
	case "workspace.mkdir":
		return d.handleMkdir(ctx, input)
	case "workspace.rename":
		return d.handleRename(ctx, input)
	case "workspace.move":
		return d.handleMove(ctx, input)
	case "workspace.copy":
		return d.handleCopy(ctx, input)
	case "workspace.delete":
		return d.handleDelete(ctx, input)
	case "workspace.search":
		return d.handleSearch(ctx, input)
	case "workspace.patch":
		return d.handlePrecisePatch(ctx, input)
	case "workspace.diff":
		return d.handlePreciseDiff(ctx, input)
	case "workspace.replace":
		return d.handlePreciseReplace(ctx, input)
	default:
		return nil, fmt.Errorf("unknown workspace tool: %s", handlerName)
	}
}

func (d *ToolDispatcher) handleList(ctx context.Context, input json.RawMessage) (json.RawMessage, error) {
	var req struct {
		URI    string `json:"uri"`
		Limit  int    `json:"limit"`
		Cursor string `json:"cursor"`
	}
	if err := json.Unmarshal(input, &req); err != nil {
		return nil, err
	}
	opts := ListOptions{Limit: 100}
	if req.Limit > 0 {
		opts.Limit = req.Limit
	}
	opts.Cursor = req.Cursor
	result, err := d.service.List(ctx, req.URI, opts)
	if err != nil {
		return nil, err
	}
	return json.Marshal(map[string]any{
		"entries":    result.Entries,
		"nextCursor": result.NextCursor,
		"hasMore":    result.HasMore,
	})
}

func (d *ToolDispatcher) handleStat(ctx context.Context, input json.RawMessage) (json.RawMessage, error) {
	var req struct {
		URI string `json:"uri"`
	}
	if err := json.Unmarshal(input, &req); err != nil {
		return nil, err
	}
	entry, err := d.service.Stat(ctx, req.URI)
	if err != nil {
		return nil, err
	}
	return json.Marshal(map[string]any{"entry": entry})
}

func (d *ToolDispatcher) handleRead(ctx context.Context, input json.RawMessage) (json.RawMessage, error) {
	var req struct {
		URI      string `json:"uri"`
		Offset   int64  `json:"offset"`
		MaxBytes int64  `json:"maxBytes"`
	}
	if err := json.Unmarshal(input, &req); err != nil {
		return nil, err
	}
	opts := ReadOptions{}
	if req.Offset > 0 {
		opts.Offset = req.Offset
	}
	if req.MaxBytes > 0 {
		opts.MaxBytes = req.MaxBytes
	}
	result, err := d.service.Read(ctx, req.URI, opts)
	if err != nil {
		return nil, err
	}
	return json.Marshal(map[string]any{
		"content":  string(result.Content),
		"isText":   result.IsText,
		"resource": result.Resource,
	})
}

func (d *ToolDispatcher) handleWrite(ctx context.Context, input json.RawMessage) (json.RawMessage, error) {
	var req struct {
		URI       string `json:"uri"`
		Text      string `json:"text"`
		SourceURI string `json:"sourceUri"`
		Overwrite bool   `json:"overwrite"`
	}
	if err := json.Unmarshal(input, &req); err != nil {
		return nil, err
	}
	opts := WriteOptions{Overwrite: req.Overwrite, Atomic: true}
	if req.SourceURI != "" {
		sourceResult, err := d.service.Read(ctx, req.SourceURI, ReadOptions{MaxBytes: MaxSingleWrite})
		if err != nil {
			return nil, fmt.Errorf("read source: %w", err)
		}
		entry, err := d.service.Write(ctx, req.URI, readerFromBytes(sourceResult.Content), opts)
		if err != nil {
			return nil, err
		}
		return json.Marshal(map[string]any{"entry": entry})
	}
	if req.Text != "" {
		entry, err := d.service.Write(ctx, req.URI, strings.NewReader(req.Text), opts)
		if err != nil {
			return nil, err
		}
		return json.Marshal(map[string]any{"entry": entry})
	}
	return nil, fmt.Errorf("either text or sourceUri is required")
}

func (d *ToolDispatcher) handleMkdir(ctx context.Context, input json.RawMessage) (json.RawMessage, error) {
	var req struct {
		URI string `json:"uri"`
	}
	if err := json.Unmarshal(input, &req); err != nil {
		return nil, err
	}
	entry, err := d.service.Mkdir(ctx, req.URI)
	if err != nil {
		return nil, err
	}
	return json.Marshal(map[string]any{"entry": entry})
}

func (d *ToolDispatcher) handleRename(ctx context.Context, input json.RawMessage) (json.RawMessage, error) {
	var req struct {
		URI     string `json:"uri"`
		NewName string `json:"newName"`
	}
	if err := json.Unmarshal(input, &req); err != nil {
		return nil, err
	}
	entry, err := d.service.Rename(ctx, req.URI, req.NewName)
	if err != nil {
		return nil, err
	}
	return json.Marshal(map[string]any{"entry": entry})
}

func (d *ToolDispatcher) handleMove(ctx context.Context, input json.RawMessage) (json.RawMessage, error) {
	var req struct {
		SourceURI  string `json:"sourceUri"`
		DestDirURI string `json:"destinationDirUri"`
	}
	if err := json.Unmarshal(input, &req); err != nil {
		return nil, err
	}
	entry, err := d.service.Move(ctx, req.SourceURI, req.DestDirURI)
	if err != nil {
		return nil, err
	}
	return json.Marshal(map[string]any{"entry": entry})
}

func (d *ToolDispatcher) handleCopy(ctx context.Context, input json.RawMessage) (json.RawMessage, error) {
	var req struct {
		SourceURI  string `json:"sourceUri"`
		DestDirURI string `json:"destinationDirUri"`
	}
	if err := json.Unmarshal(input, &req); err != nil {
		return nil, err
	}
	entry, err := d.service.Copy(ctx, req.SourceURI, req.DestDirURI)
	if err != nil {
		return nil, err
	}
	return json.Marshal(map[string]any{"entry": entry})
}

func (d *ToolDispatcher) handleDelete(ctx context.Context, input json.RawMessage) (json.RawMessage, error) {
	var req struct {
		URI       string `json:"uri"`
		Recursive bool   `json:"recursive"`
	}
	if err := json.Unmarshal(input, &req); err != nil {
		return nil, err
	}
	opts := DeleteOptions{Recursive: req.Recursive}
	err := d.service.Delete(ctx, req.URI, opts)
	if err != nil {
		return nil, err
	}
	return json.Marshal(map[string]any{"deleted": true})
}

func (d *ToolDispatcher) handleSearch(ctx context.Context, input json.RawMessage) (json.RawMessage, error) {
	if d.precise == nil {
		return nil, fmt.Errorf("precise editing service not available")
	}
	var req struct {
		WorkspaceID   string   `json:"workspaceId"`
		Query         string   `json:"query"`
		Regex         bool     `json:"regex"`
		IncludeGlobs  []string `json:"includeGlobs"`
		ExcludeGlobs  []string `json:"excludeGlobs"`
		MaxResults    int      `json:"maxResults"`
		ContextBefore int      `json:"contextBefore"`
		ContextAfter  int      `json:"contextAfter"`
	}
	if err := json.Unmarshal(input, &req); err != nil {
		return nil, err
	}
	result, err := d.precise.Search(ctx, SearchRequest{
		WorkspaceID:   req.WorkspaceID,
		Query:         req.Query,
		Regex:         req.Regex,
		IncludeGlobs:  req.IncludeGlobs,
		ExcludeGlobs:  req.ExcludeGlobs,
		MaxResults:    req.MaxResults,
		ContextBefore: req.ContextBefore,
		ContextAfter:  req.ContextAfter,
	})
	if err != nil {
		return nil, err
	}
	return json.Marshal(map[string]any{
		"matches":   result.Matches,
		"total":     result.Total,
		"truncated": result.Truncated,
	})
}

func (d *ToolDispatcher) handlePrecisePatch(ctx context.Context, input json.RawMessage) (json.RawMessage, error) {
	if d.precise == nil {
		return nil, fmt.Errorf("precise editing service not available")
	}
	var req struct {
		WorkspaceID string `json:"workspaceId"`
		FilePath    string `json:"filePath"`
		BaseSHA256  string `json:"baseSha256"`
		Patch       string `json:"patch"`
	}
	if err := json.Unmarshal(input, &req); err != nil {
		return nil, err
	}
	result, err := d.precise.Patch(ctx, PatchRequest{
		WorkspaceID: req.WorkspaceID,
		FilePath:    req.FilePath,
		BaseSHA256:  req.BaseSHA256,
		Patch:       req.Patch,
	})
	if err != nil {
		return nil, err
	}
	return json.Marshal(map[string]any{
		"applied":   result.Applied,
		"filePath":  result.FilePath,
		"newSha256": result.NewSHA256,
	})
}

func (d *ToolDispatcher) handlePreciseDiff(ctx context.Context, input json.RawMessage) (json.RawMessage, error) {
	if d.precise == nil {
		return nil, fmt.Errorf("precise editing service not available")
	}
	var req struct {
		WorkspaceID string            `json:"workspaceId"`
		BeforeFiles map[string]string `json:"beforeFiles"`
		AfterFiles  map[string]string `json:"afterFiles"`
	}
	if err := json.Unmarshal(input, &req); err != nil {
		return nil, err
	}
	result, err := d.precise.Diff(ctx, DiffRequest{
		WorkspaceID: req.WorkspaceID,
		BeforeFiles: req.BeforeFiles,
		AfterFiles:  req.AfterFiles,
	})
	if err != nil {
		return nil, err
	}
	return json.Marshal(map[string]any{
		"changedFiles": result.ChangedFiles,
		"unifiedDiff":  result.UnifiedDiff,
		"additions":    result.Additions,
		"deletions":    result.Deletions,
	})
}

func (d *ToolDispatcher) handlePreciseReplace(ctx context.Context, input json.RawMessage) (json.RawMessage, error) {
	if d.precise == nil {
		return nil, fmt.Errorf("precise editing service not available")
	}
	var req struct {
		WorkspaceID         string `json:"workspaceId"`
		FilePath            string `json:"filePath"`
		OldText             string `json:"oldText"`
		NewText             string `json:"newText"`
		ExpectedOccurrences int    `json:"expectedOccurrences"`
	}
	if err := json.Unmarshal(input, &req); err != nil {
		return nil, err
	}
	result, err := d.precise.Replace(ctx, ReplaceRequest{
		WorkspaceID:         req.WorkspaceID,
		FilePath:            req.FilePath,
		OldText:             req.OldText,
		NewText:             req.NewText,
		ExpectedOccurrences: req.ExpectedOccurrences,
	})
	if err != nil {
		return nil, err
	}
	return json.Marshal(map[string]any{
		"replaced":          result.Replaced,
		"actualOccurrences": result.ActualOccurrences,
		"filePath":          result.FilePath,
	})
}

func readerFromBytes(data []byte) io.Reader {
	return strings.NewReader(string(data))
}
