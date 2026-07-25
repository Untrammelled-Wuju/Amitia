// Deprecated: Legacy extension architecture.
// Do not add new capabilities. This implementation is retained only for
// compatibility, maintenance, testing, and migration to Extension Kernel.

package skill

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/u-ai/backend/internal/extension"
	"github.com/u-ai/backend/internal/mcp"
	"github.com/u-ai/backend/internal/mcp/client"
)

var sensitiveValuePattern = regexp.MustCompile(`(?i)(bearer\s+[a-z0-9._~+/-]{12,}|(?:api[_-]?key|access[_-]?token|refresh[_-]?token|password|secret)["'\s:=]+[a-z0-9._~+/-]{8,})`)

type Caller interface {
	Call(context.Context, string, string, any, client.CallOptions) (json.RawMessage, error)
}

type Runtime struct {
	repository *mcp.Repository
	caller     Caller
	extensions *extension.Runtime
}

type toolCallResult struct {
	Content           []contentItem   `json:"content"`
	StructuredContent json.RawMessage `json:"structuredContent"`
	IsError           bool            `json:"isError"`
	Task              *remoteTask     `json:"task,omitempty"`
}

type remoteTask struct {
	TaskID        string          `json:"taskId"`
	Status        string          `json:"status"`
	StatusMessage string          `json:"statusMessage"`
	Result        json.RawMessage `json:"result"`
	ExpiresAt     string          `json:"expiresAt"`
}

type contentItem struct {
	Type     string          `json:"type"`
	Text     string          `json:"text,omitempty"`
	Data     string          `json:"data,omitempty"`
	MIMEType string          `json:"mimeType,omitempty"`
	URI      string          `json:"uri,omitempty"`
	Resource json.RawMessage `json:"resource,omitempty"`
}

func New(repository *mcp.Repository, caller Caller, extensions *extension.Runtime) *Runtime {
	return &Runtime{repository: repository, caller: caller, extensions: extensions}
}

func (r *Runtime) RegisterAll(ctx context.Context) error {
	servers, err := r.repository.ListServers(ctx)
	if err != nil {
		return err
	}
	for _, server := range servers {
		if err := r.RegisterServer(ctx, server.ID); err != nil {
			return err
		}
	}
	return nil
}

func (r *Runtime) RegisterServer(ctx context.Context, serverID string) error {
	server, err := r.repository.GetServer(ctx, serverID)
	if err != nil {
		return err
	}
	tools, err := r.repository.ListTools(ctx, serverID, false)
	if err != nil {
		return err
	}
	registered, err := r.extensions.Registry.List(ctx, extension.SkillFilter{Source: extension.SkillSourceMCP, IncludeInternal: true})
	if err != nil {
		return err
	}
	desired := map[string]bool{}
	for _, tool := range tools {
		desired[tool.SkillID] = true
	}
	for _, current := range registered {
		if strings.HasPrefix(current.Definition.ID, "mcp."+skillSegment(serverID)+".") && !desired[current.Definition.ID] {
			_ = r.extensions.Registry.Unregister(ctx, current.Definition.ID)
		}
	}
	for _, tool := range tools {
		if _, getErr := r.extensions.Registry.Get(ctx, tool.SkillID); getErr == nil {
			_ = r.extensions.Registry.Unregister(ctx, tool.SkillID)
		}
		definition, handler, buildErr := r.build(server, tool)
		if buildErr != nil {
			return buildErr
		}
		if err := r.extensions.Registry.Register(ctx, definition, handler); err != nil {
			return err
		}
		if err := r.extensions.Registry.SetEnabled(ctx, definition.ID, tool.Enabled == 1); err != nil {
			return err
		}
	}
	return nil
}

func (r *Runtime) build(server mcp.Server, tool mcp.ToolDefinition) (extension.SkillDefinition, extension.SkillHandler, error) {
	input := json.RawMessage(tool.InputSchemaJSON)
	output := json.RawMessage(tool.OutputSchemaJSON)
	if len(output) == 0 {
		output = json.RawMessage(`{}`)
	}
	capabilities, sideEffects, idempotent := capabilities(server, tool)
	triggers := []extension.SkillTrigger{extension.TriggerLLM, extension.TriggerManual}
	name := tool.Title
	if strings.TrimSpace(name) == "" {
		name = tool.RemoteName
	}
	manifest := extension.Manifest{Schema: "https://schemas.amitia.dev/extensions/v1/manifest.schema.json", APIVersion: "extensions.amitia.dev/v1alpha1", Kind: "Skill", Metadata: extension.ManifestMetadata{ID: tool.SkillID, Name: name, Version: "1.0.0", Description: tool.Description, Author: "MCP Server", License: "LicenseRef-MCP-Remote"}, Compatibility: extension.ManifestCompatibility{EngineMin: "1.0.0", EngineMaxExclusive: "2.0.0"}, Entry: extension.SkillEntry{Kind: "mcp", Name: tool.RemoteName}, Capabilities: capabilities, Triggers: triggers, Execution: extension.ManifestExecution{TimeoutMS: 30000, HasSideEffects: sideEffects, Retryable: idempotent && !sideEffects, Idempotent: idempotent}, InputSchema: input, OutputSchema: output, ConfigSchema: json.RawMessage(`{"$schema":"https://json-schema.org/draft/2020-12/schema","type":"object","additionalProperties":false,"properties":{}}`), DefaultConfig: json.RawMessage(`{}`), Enabled: tool.Enabled == 1, AllowLLM: true, AllowManual: true}
	manifestRaw, err := json.Marshal(manifest)
	if err != nil {
		return extension.SkillDefinition{}, nil, err
	}
	definition := extension.SkillDefinition{ID: tool.SkillID, ModelName: modelName(server.ID, tool.RemoteName), Name: name, Description: tool.Description, Version: "1.0.0", Source: extension.SkillSourceMCP, Entry: manifest.Entry, InputSchema: input, OutputSchema: output, ConfigSchema: manifest.ConfigSchema, DefaultConfig: manifest.DefaultConfig, Capabilities: capabilities, Triggers: triggers, Timeout: 30 * time.Second, TimeoutMS: 30000, HasSideEffects: sideEffects, Retryable: manifest.Execution.Retryable, Idempotent: idempotent, Enabled: tool.Enabled == 1, Compatible: true, Author: manifest.Metadata.Author, License: manifest.Metadata.License, Manifest: manifestRaw}
	handler := func(ctx context.Context, request extension.ExecuteSkillRequest) (result extension.SkillResult, runErr error) {
		started := time.Now()
		defer func() {
			status := "succeeded"
			errorCode := ""
			if runErr != nil {
				status = "failed"
				errorCode = "MCP_TOOL_CALL_FAILED"
			}
			summary, _ := json.Marshal(map[string]any{"inputBytes": len(request.Input), "outputBytes": len(result.Output), "sideEffects": result.SideEffects != nil})
			_ = r.repository.AddAuditLog(context.Background(), mcp.AuditLog{ServerID: server.ID, Operation: "tools/call", ToolName: tool.RemoteName, CharacterID: request.Scope.CharacterID, ConversationID: request.Scope.ConversationID, Channel: request.Scope.Channel, TraceID: request.Scope.TraceID, OperationID: request.Scope.RequestID, Status: status, DurationMS: time.Since(started).Milliseconds(), ErrorCode: errorCode, SummaryJSON: string(summary)})
		}()
		enabled, _, scopeErr := r.repository.ResolveScopeEnabled(ctx, server.ID, request.Scope.CharacterID)
		if scopeErr != nil {
			return extension.SkillResult{}, scopeErr
		}
		if !enabled {
			return extension.SkillResult{}, extension.NewExtensionError(extension.ErrSkillPermissionDenied, "MCP Server is not enabled for this role", server.ID, false, nil)
		}
		current, currentErr := r.repository.GetToolBySkillID(ctx, tool.SkillID)
		if currentErr != nil {
			return extension.SkillResult{}, extension.NewExtensionError(extension.ErrSkillNotFound, "MCP Tool not found", tool.SkillID, false, currentErr)
		}
		if current.Enabled != 1 {
			return extension.SkillResult{}, extension.NewExtensionError(extension.ErrSkillDisabled, "MCP Tool is disabled", tool.SkillID, false, nil)
		}
		var arguments any
		if json.Unmarshal(request.Input, &arguments) != nil {
			return extension.SkillResult{}, extension.NewExtensionError(extension.ErrSkillInputInvalid, "MCP Tool input is invalid", "", false, nil)
		}
		raw, callErr := r.caller.Call(ctx, server.ID, "tools/call", map[string]any{"name": current.RemoteName, "arguments": arguments}, client.CallOptions{})
		if callErr != nil {
			return extension.SkillResult{}, extension.NewExtensionError(extension.ErrSkillExecutionFailed, "MCP Tool remote error", safeError(callErr), true, callErr)
		}
		result, remoteError, normalizeErr := normalizeResult(raw)
		if normalizeErr != nil {
			return extension.SkillResult{}, normalizeErr
		}
		if remoteError {
			return extension.SkillResult{}, extension.NewExtensionError(extension.ErrSkillExecutionFailed, "MCP Tool reported an error", result.VisibleText, false, nil)
		}
		var remoteResult toolCallResult
		if json.Unmarshal(raw, &remoteResult) == nil && remoteResult.Task != nil && remoteResult.Task.TaskID != "" && validTaskStatus(remoteResult.Task.Status) {
			enabled, _, capabilityErr := r.repository.ServerCapabilityEnabled(ctx, server.ID, "tasks")
			if capabilityErr == nil && enabled {
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
				_ = r.repository.UpsertTask(ctx, mcp.Task{ServerID: server.ID, RemoteTaskID: remoteResult.Task.TaskID, CharacterID: request.Scope.CharacterID, RunID: request.Scope.RequestID, Status: remoteResult.Task.Status, StatusMessage: remoteResult.Task.StatusMessage, ResultJSON: string(taskResult), ExpiresAt: expires.UTC().Format(time.RFC3339Nano)})
			}
		}
		result.SideEffects = sideEffectRecords(capabilities, sideEffects)
		return result, nil
	}
	return definition, handler, nil
}

func normalizeResult(raw json.RawMessage) (extension.SkillResult, bool, error) {
	if len(raw) > 4<<20 {
		return extension.SkillResult{}, false, extension.NewExtensionError(extension.ErrSkillOutputInvalid, "MCP Tool output is too large", "", false, nil)
	}
	var response toolCallResult
	if err := json.Unmarshal(raw, &response); err != nil {
		return extension.SkillResult{}, false, extension.NewExtensionError(extension.ErrSkillOutputInvalid, "MCP Tool output is invalid", "", false, err)
	}
	if len(response.Content) > 32 {
		return extension.SkillResult{}, false, extension.NewExtensionError(extension.ErrSkillOutputInvalid, "MCP Tool returned too many content items", "", false, nil)
	}
	visible := strings.Builder{}
	for index := range response.Content {
		item := &response.Content[index]
		if len(item.Text) > 256<<10 || len(item.Data) > 2<<20 || len(item.Resource) > 512<<10 {
			return extension.SkillResult{}, false, extension.NewExtensionError(extension.ErrSkillOutputInvalid, "MCP Tool content is too large", "", false, nil)
		}
		if item.Type != "text" && item.Type != "image" && item.Type != "audio" && item.Type != "resource_link" && item.Type != "resource" {
			return extension.SkillResult{}, false, extension.NewExtensionError(extension.ErrSkillOutputInvalid, "MCP Tool content type is invalid", item.Type, false, nil)
		}
		if item.Type == "text" && item.Text != "" {
			if visible.Len() > 0 {
				visible.WriteByte('\n')
			}
			visible.WriteString(item.Text)
		}
	}
	visibleText := redact(visible.String())
	output := response.StructuredContent
	if len(output) == 0 {
		output, _ = json.Marshal(map[string]any{"content": response.Content})
	}
	if sensitiveValuePattern.Match(output) {
		output = json.RawMessage(`{"redacted":true}`)
	}
	return extension.SkillResult{Status: extension.RunSucceeded, Output: output, VisibleText: visibleText}, response.IsError, nil
}

func capabilities(server mcp.Server, tool mcp.ToolDefinition) ([]string, bool, bool) {
	base := normalize(server.ID)
	name := strings.ToLower(tool.RemoteName + " " + tool.Description)
	result := []string{"mcp.invoke", "mcp.server." + base, "mcp.tool." + base + "." + normalize(tool.RemoteName)}
	if server.Transport == "streamable_http" {
		result = append(result, "network.remote")
	}
	sideEffects := tool.RiskLevel != "low"
	idempotent := strings.Contains(tool.CapabilityHintsJSON, "idempotent")
	switch {
	case strings.Contains(name, "delete") || strings.Contains(name, "remove"):
		result = append(result, "data.delete")
		sideEffects = true
	case strings.Contains(name, "send") || strings.Contains(name, "message") || strings.Contains(name, "publish"):
		result = append(result, "message.send")
		sideEffects = true
	case strings.Contains(name, "pay") || strings.Contains(name, "purchase") || strings.Contains(name, "transfer"):
		result = append(result, "financial.action")
		sideEffects = true
	case strings.Contains(name, "write_file") || strings.Contains(name, "save_file"):
		result = append(result, "filesystem.write")
		sideEffects = true
	case strings.Contains(name, "read_file") || strings.Contains(name, "list_file"):
		result = append(result, "filesystem.read")
	case sideEffects:
		result = append(result, "external.account.write")
	default:
		result = append(result, "external.account.read")
	}
	return result, sideEffects, idempotent
}

func sideEffectRecords(capabilities []string, sideEffects bool) []extension.SideEffectRecord {
	if !sideEffects {
		return nil
	}
	records := []extension.SideEffectRecord{}
	for _, capability := range capabilities {
		if capability == "data.delete" || capability == "message.send" || capability == "financial.action" || capability == "filesystem.write" || capability == "external.account.write" {
			records = append(records, extension.SideEffectRecord{Type: capability, Confirmed: true})
		}
	}
	return records
}
func modelName(serverID, toolName string) string {
	value := "mcp_" + normalize(serverID) + "_" + normalize(toolName)
	if len(value) > 64 {
		hash := sha256.Sum256([]byte(value))
		value = value[:55] + "_" + fmt.Sprintf("%x", hash[:4])
	}
	return value
}
func normalize(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var result strings.Builder
	for _, item := range value {
		if item >= 'a' && item <= 'z' || item >= '0' && item <= '9' {
			result.WriteRune(item)
		} else if result.Len() > 0 && !strings.HasSuffix(result.String(), "_") {
			result.WriteByte('_')
		}
	}
	return strings.Trim(result.String(), "_")
}
func skillSegment(value string) string { return strings.ReplaceAll(normalize(value), "_", "-") }
func redact(value string) string       { return sensitiveValuePattern.ReplaceAllString(value, "[REDACTED]") }
func safeError(err error) string       { return redact(strings.TrimSpace(err.Error())) }
func validTaskStatus(value string) bool {
	return value == "working" || value == "input_required" || value == "completed" || value == "failed" || value == "cancelled"
}
