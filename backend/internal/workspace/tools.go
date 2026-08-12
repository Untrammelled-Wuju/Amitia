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
	RuntimeType: capability.RuntimeTypeInternal,
	RuntimeID:   "workspace",
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
		ID:          "workspace.list",
		ModelName:   "workspace.list",
		Source:      capability.ToolSourceBuiltin,
		Name:        "Workspace List",
		Description: "List entries in a workspace directory.",
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
		ID:          "workspace.stat",
		ModelName:   "workspace.stat",
		Source:      capability.ToolSourceBuiltin,
		Name:        "Workspace Stat",
		Description: "Get metadata of a workspace entry.",
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
		ID:          "workspace.read",
		ModelName:   "workspace.read",
		Source:      capability.ToolSourceBuiltin,
		Name:        "Workspace Read",
		Description: "Read content from a workspace file.",
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
		ID:          "workspace.write",
		ModelName:   "workspace.write",
		Source:      capability.ToolSourceBuiltin,
		Name:        "Workspace Write",
		Description: "Write content to a workspace file.",
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
		ID:          "workspace.mkdir",
		ModelName:   "workspace.mkdir",
		Source:      capability.ToolSourceBuiltin,
		Name:        "Workspace Mkdir",
		Description: "Create a directory in the workspace.",
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
		ID:          "workspace.rename",
		ModelName:   "workspace.rename",
		Source:      capability.ToolSourceBuiltin,
		Name:        "Workspace Rename",
		Description: "Rename a workspace entry.",
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
		ID:          "workspace.move",
		ModelName:   "workspace.move",
		Source:      capability.ToolSourceBuiltin,
		Name:        "Workspace Move",
		Description: "Move a workspace entry within the same mount.",
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
		ID:          "workspace.copy",
		ModelName:   "workspace.copy",
		Source:      capability.ToolSourceBuiltin,
		Name:        "Workspace Copy",
		Description: "Copy a workspace entry within the same mount.",
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
		ID:          "workspace.delete",
		ModelName:   "workspace.delete",
		Source:      capability.ToolSourceBuiltin,
		Name:        "Workspace Delete",
		Description: "Delete a workspace entry.",
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

type ToolDispatcher struct {
	service *Service
}

func NewToolDispatcher(service *Service) *ToolDispatcher {
	return &ToolDispatcher{service: service}
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
		SourceURI string `json:"sourceUri"`
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
		SourceURI string `json:"sourceUri"`
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

func readerFromBytes(data []byte) io.Reader {
	return strings.NewReader(string(data))
}
