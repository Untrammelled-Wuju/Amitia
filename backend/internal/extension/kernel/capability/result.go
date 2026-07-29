package capability

import (
	"encoding/json"
)

type ToolResultStatus string

const (
	ToolResultStatusSuccess   ToolResultStatus = "success"
	ToolResultStatusFailed    ToolResultStatus = "failed"
	ToolResultStatusCancelled ToolResultStatus = "cancelled"
	ToolResultStatusTimedOut  ToolResultStatus = "timed_out"
)

type ToolContentType string

const (
	ToolContentText              ToolContentType = "text"
	ToolContentStructured        ToolContentType = "structured"
	ToolContentBinaryReference   ToolContentType = "binary_reference"
	ToolContentResourceReference ToolContentType = "resource_reference"
	ToolContentUIContent         ToolContentType = "ui_content"
	ToolContentStream            ToolContentType = "stream"
	ToolContentTaskReference     ToolContentType = "task_reference"
)

type ToolContent struct {
	Type     ToolContentType `json:"type"`
	Text     string          `json:"text,omitempty"`
	MIMEType string          `json:"mimeType,omitempty"`
	URI      string          `json:"uri,omitempty"`
	Data     json.RawMessage `json:"data,omitempty"`
}

type ToolError struct {
	Code        string         `json:"code"`
	Message     string         `json:"message"`
	Retryable   bool           `json:"retryable"`
	UserVisible bool           `json:"userVisible"`
	Details     map[string]any `json:"details,omitempty"`
	Cause       error          `json:"-"`
}

func (e *ToolError) Error() string {
	return e.Code + ": " + e.Message
}

func (e *ToolError) Unwrap() error {
	return e.Cause
}

const (
	ErrorCodeInvalidInput       = "invalid_input"
	ErrorCodePermissionDenied   = "permission_denied"
	ErrorCodeScopeDenied        = "scope_denied"
	ErrorCodeNotAvailable       = "not_available"
	ErrorCodeRuntimeUnavailable = "runtime_unavailable"
	ErrorCodeTimeout            = "timeout"
	ErrorCodeCancelled          = "cancelled"
	ErrorCodeConflict           = "conflict"
	ErrorCodeRateLimited        = "rate_limited"
	ErrorCodeDependencyMissing  = "dependency_missing"
	ErrorCodeConnectionLost     = "connection_lost"
	ErrorCodeExecutionFailed    = "execution_failed"
	ErrorCodeInvalidResult      = "invalid_result"
	ErrorCodeInternalError      = "internal_error"
)

type RecordedSideEffect struct {
	Type        string         `json:"type"`
	Target      string         `json:"target"`
	Description string         `json:"description,omitempty"`
	Reversible  bool           `json:"reversible"`
	Metadata    map[string]any `json:"metadata,omitempty"`
}

type UnifiedToolResult struct {
	InvocationID string               `json:"invocationId"`
	Status       ToolResultStatus     `json:"status"`
	Content      []ToolContent        `json:"content,omitempty"`
	Structured   json.RawMessage      `json:"structured,omitempty"`
	Error        *ToolError           `json:"error,omitempty"`
	SideEffects  []RecordedSideEffect `json:"sideEffects,omitempty"`
	Metadata     map[string]any       `json:"metadata,omitempty"`
}
