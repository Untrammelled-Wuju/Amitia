package capability

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

type MCPCallFunc func(ctx context.Context, serverID string, toolName string, input json.RawMessage) (json.RawMessage, error)
type MCPHealthFunc func(ctx context.Context, serverID string) HealthStatus

var mcpSensitiveValuePattern = regexp.MustCompile(`(?i)(bearer\s+[a-z0-9._~+/-]{12,}|(?:api[_-]?key|access[_-]?token|refresh[_-]?token|password|secret)["'\s:=]+[a-z0-9._~+/-]{8,})`)

type MCPPostProcessor interface {
	AfterExecute(ctx context.Context, serverID string, invocation ToolInvocationContext, raw json.RawMessage)
}

type MCPRuntimeAdapter struct {
	caller        MCPCallFunc
	health        MCPHealthFunc
	postProcessor MCPPostProcessor
}

func NewMCPRuntimeAdapter(caller MCPCallFunc, health MCPHealthFunc) *MCPRuntimeAdapter {
	return &MCPRuntimeAdapter{caller: caller, health: health}
}

func (a *MCPRuntimeAdapter) SetPostProcessor(p MCPPostProcessor) {
	a.postProcessor = p
}

func (a *MCPRuntimeAdapter) Supports(binding RuntimeBinding) bool {
	return binding.RuntimeType == RuntimeTypeMCP
}

type mcpContentItem struct {
	Type     string          `json:"type"`
	Text     string          `json:"text,omitempty"`
	Data     string          `json:"data,omitempty"`
	MIMEType string          `json:"mimeType,omitempty"`
	URI      string          `json:"uri,omitempty"`
	Resource json.RawMessage `json:"resource,omitempty"`
}

type mcpCallResult struct {
	Content           []mcpContentItem `json:"content"`
	StructuredContent json.RawMessage  `json:"structuredContent"`
	IsError           bool             `json:"isError"`
}

func (a *MCPRuntimeAdapter) Execute(
	ctx context.Context,
	binding RuntimeBinding,
	invocation ToolInvocationContext,
	input json.RawMessage,
) UnifiedToolResult {
	if a.caller == nil {
		return UnifiedToolResult{
			InvocationID: invocation.InvocationID,
			Status:       ToolResultStatusFailed,
			Error: &ToolError{
				Code:        ErrorCodeRuntimeUnavailable,
				Message:     "MCP caller not configured",
				UserVisible: false,
			},
		}
	}

	output, err := a.caller(ctx, binding.RuntimeID, binding.HandlerName, input)
	if err != nil {
		code := ErrorCodeExecutionFailed
		userVisible := false
		switch {
		case contains(err.Error(), "connection"):
			code = ErrorCodeConnectionLost
		case contains(err.Error(), "timeout"):
			code = ErrorCodeTimeout
		}
		return UnifiedToolResult{
			InvocationID: invocation.InvocationID,
			Status:       ToolResultStatusFailed,
			Error: &ToolError{
				Code:        code,
				Message:     mcpRedact(err.Error()),
				UserVisible: userVisible,
			},
		}
	}

	if a.postProcessor != nil {
		a.postProcessor.AfterExecute(ctx, binding.RuntimeID, invocation, output)
	}

	return a.normalizeMCPResult(output, invocation.InvocationID)
}

func (a *MCPRuntimeAdapter) normalizeMCPResult(raw json.RawMessage, invocationID string) UnifiedToolResult {
	if len(raw) > 4<<20 {
		return UnifiedToolResult{
			InvocationID: invocationID,
			Status:       ToolResultStatusFailed,
			Error: &ToolError{
				Code:    ErrorCodeInvalidResult,
				Message: "MCP Tool output is too large",
			},
		}
	}

	var response mcpCallResult
	if err := json.Unmarshal(raw, &response); err != nil {
		return UnifiedToolResult{
			InvocationID: invocationID,
			Status:       ToolResultStatusFailed,
			Error: &ToolError{
				Code:    ErrorCodeInvalidResult,
				Message: "MCP Tool output is invalid",
			},
		}
	}

	if len(response.Content) > 32 {
		return UnifiedToolResult{
			InvocationID: invocationID,
			Status:       ToolResultStatusFailed,
			Error: &ToolError{
				Code:    ErrorCodeInvalidResult,
				Message: "MCP Tool returned too many content items",
			},
		}
	}

	contentItems := make([]ToolContent, 0, len(response.Content))
	visibleBuilder := strings.Builder{}

	for index := range response.Content {
		item := &response.Content[index]
		if len(item.Text) > 256<<10 || len(item.Data) > 2<<20 || len(item.Resource) > 512<<10 {
			return UnifiedToolResult{
				InvocationID: invocationID,
				Status:       ToolResultStatusFailed,
				Error: &ToolError{
					Code:    ErrorCodeInvalidResult,
					Message: "MCP Tool content is too large",
				},
			}
		}
		if item.Type != "text" && item.Type != "image" && item.Type != "audio" && item.Type != "resource_link" && item.Type != "resource" {
			return UnifiedToolResult{
				InvocationID: invocationID,
				Status:       ToolResultStatusFailed,
				Error: &ToolError{
					Code:    ErrorCodeInvalidResult,
					Message: fmt.Sprintf("MCP Tool content type is invalid: %s", item.Type),
				},
			}
		}
		if item.Type == "text" && item.Text != "" {
			if visibleBuilder.Len() > 0 {
				visibleBuilder.WriteByte('\n')
			}
			visibleBuilder.WriteString(item.Text)
		}
		contentItems = append(contentItems, ToolContent{
			Type:     ToolContentText,
			Text:     mcpRedact(item.Text),
			MIMEType: item.MIMEType,
			URI:      item.URI,
		})
	}

	visibleText := mcpRedact(visibleBuilder.String())

	structured := response.StructuredContent
	if len(structured) == 0 {
		structured, _ = json.Marshal(map[string]any{"content": response.Content})
	}
	if mcpSensitiveValuePattern.Match(structured) {
		structured = json.RawMessage(`{"redacted":true}`)
	}

	status := ToolResultStatusSuccess
	if response.IsError {
		status = ToolResultStatusFailed
	}

	return UnifiedToolResult{
		InvocationID: invocationID,
		Status:       status,
		Content:      contentItems,
		Structured:   structured,
		Metadata: map[string]any{
			"visibleText": visibleText,
		},
	}
}

func mcpRedact(value string) string {
	return mcpSensitiveValuePattern.ReplaceAllString(value, "[REDACTED]")
}

func (a *MCPRuntimeAdapter) Health(ctx context.Context, binding RuntimeBinding) HealthStatus {
	if a.health != nil {
		return a.health(ctx, binding.RuntimeID)
	}
	return HealthUnknown
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
