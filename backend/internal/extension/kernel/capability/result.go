package capability

import (
	"context"
	"encoding/json"
	"errors"
)

type ToolResultStatus string

const (
	ToolResultStatusSuccess   ToolResultStatus = "success"
	ToolResultStatusFailed    ToolResultStatus = "failed"
	ToolResultStatusCancelled ToolResultStatus = "cancelled"
	ToolResultStatusTimedOut  ToolResultStatus = "timed_out"
)

func (s ToolResultStatus) Valid() bool {
	switch s {
	case ToolResultStatusSuccess,
		ToolResultStatusFailed,
		ToolResultStatusCancelled,
		ToolResultStatusTimedOut:
		return true
	default:
		return false
	}
}

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

type ToolErrorCategory string

const (
	ToolErrorCategoryValidation  ToolErrorCategory = "validation"
	ToolErrorCategoryPermission  ToolErrorCategory = "permission"
	ToolErrorCategoryAvailability ToolErrorCategory = "availability"
	ToolErrorCategoryRuntime     ToolErrorCategory = "runtime"
	ToolErrorCategoryTimeout     ToolErrorCategory = "timeout"
	ToolErrorCategoryCancellation ToolErrorCategory = "cancellation"
	ToolErrorCategoryConflict    ToolErrorCategory = "conflict"
	ToolErrorCategoryRateLimit   ToolErrorCategory = "rate_limit"
	ToolErrorCategoryResource    ToolErrorCategory = "resource"
	ToolErrorCategoryDependency  ToolErrorCategory = "dependency"
	ToolErrorCategoryStream      ToolErrorCategory = "stream"
	ToolErrorCategoryInternal    ToolErrorCategory = "internal"
)

type ToolError struct {
	Code        string            `json:"code"`
	Message     string            `json:"message"`
	Category    ToolErrorCategory `json:"category,omitempty"`
	DomainCode  string            `json:"domainCode,omitempty"`
	Retryable   bool              `json:"retryable"`
	UserVisible bool              `json:"userVisible"`
	Details     map[string]any    `json:"details,omitempty"`
	Cause       error             `json:"-"`
}

func (e *ToolError) Error() string {
	if e == nil {
		return ""
	}
	return e.Code + ": " + e.Message
}

func (e *ToolError) Unwrap() error {
	if e == nil {
		return nil
	}
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
	ErrorCodeConnectionLost        = "connection_lost"
	ErrorCodeExecutionFailed       = "execution_failed"
	ErrorCodeInvalidResult         = "invalid_result"
	ErrorCodeStreamProtocol        = "stream_protocol_error"
	ErrorCodeStreamLimitExceeded   = "stream_limit_exceeded"
	ErrorCodeStreamDeliveryFailed  = "stream_delivery_failed"
	ErrorCodeInternalError         = "internal_error"
	ErrorCodeResourceLimitInvalid   = "resource_limit_invalid"
	ErrorCodeResourceLimitUnavailable = "resource_limit_unavailable"
	ErrorCodeResourceLimitExceeded  = "resource_limit_exceeded"
	ErrorCodeResourceUsageUnavailable = "resource_usage_unavailable"
	ErrorCodeCircuitOpen              = "circuit_open"
	ErrorCodeConcurrencyPolicyInvalid = "concurrency_policy_invalid"
	ErrorCodeRateLimitPolicyInvalid   = "rate_limit_policy_invalid"
	ErrorCodeBackpressureRejected     = "backpressure_rejected"
)

func ErrorCategoryForCode(code string) ToolErrorCategory {
	switch code {
	case ErrorCodeInvalidInput,
		ErrorCodeInvalidResult:
		return ToolErrorCategoryValidation
	case ErrorCodeResourceLimitInvalid,
		ErrorCodeResourceLimitUnavailable,
		ErrorCodeResourceLimitExceeded,
		ErrorCodeResourceUsageUnavailable:
		return ToolErrorCategoryResource
	case ErrorCodePermissionDenied,
		ErrorCodeScopeDenied:
		return ToolErrorCategoryPermission
	case ErrorCodeNotAvailable:
		return ToolErrorCategoryAvailability
	case ErrorCodeRuntimeUnavailable,
		ErrorCodeExecutionFailed,
		ErrorCodeConnectionLost:
		return ToolErrorCategoryRuntime
	case ErrorCodeTimeout:
		return ToolErrorCategoryTimeout
	case ErrorCodeCancelled:
		return ToolErrorCategoryCancellation
	case ErrorCodeConflict:
		return ToolErrorCategoryConflict
	case ErrorCodeRateLimited:
		return ToolErrorCategoryRateLimit
	case ErrorCodeDependencyMissing:
		return ToolErrorCategoryDependency
	case ErrorCodeCircuitOpen:
		return ToolErrorCategoryAvailability
	case ErrorCodeConcurrencyPolicyInvalid:
		return ToolErrorCategoryResource
	case ErrorCodeRateLimitPolicyInvalid:
		return ToolErrorCategoryResource
	case ErrorCodeBackpressureRejected:
		return ToolErrorCategoryRateLimit
	case ErrorCodeStreamProtocol,
		ErrorCodeStreamLimitExceeded,
		ErrorCodeStreamDeliveryFailed:
		return ToolErrorCategoryStream
	default:
		return ToolErrorCategoryInternal
	}
}

func NormalizeToolError(toolErr *ToolError) *ToolError {
	if toolErr == nil {
		return nil
	}
	if toolErr.Details != nil {
		toolErr.Details = cloneStringAnyMap(toolErr.Details)
	}
	if toolErr.Code == "" {
		toolErr.Code = ErrorCodeExecutionFailed
	}
	if toolErr.Category == "" {
		toolErr.Category = ErrorCategoryForCode(toolErr.Code)
	}
	return toolErr
}

func NormalizeToolErrorResult(result UnifiedToolResult) UnifiedToolResult {
	if result.Error != nil {
		result.Error = NormalizeToolError(result.Error)
	}
	return result
}

func ToolErrorFromCause(err error, fallbackCode, safeMessage string) *ToolError {
	if err == nil {
		return nil
	}
	var existing *ToolError
	if errors.As(err, &existing) {
		return NormalizeToolError(existing)
	}
	return &ToolError{
		Code:        fallbackCode,
		Category:    ErrorCategoryForCode(fallbackCode),
		Message:     safeMessage,
		Cause:       err,
		UserVisible: false,
	}
}

type RecordedSideEffect struct {
	Type        string         `json:"type"`
	Target      string         `json:"target"`
	Description string         `json:"description,omitempty"`
	Reversible  bool           `json:"reversible"`
	Metadata    map[string]any `json:"metadata,omitempty"`
}

type UnifiedToolResult struct {
	InvocationID string               `json:"invocationId"`
	ToolID       string               `json:"toolId,omitempty"`
	Status       ToolResultStatus     `json:"status"`
	Content      []ToolContent        `json:"content,omitempty"`
	Structured   json.RawMessage      `json:"structured,omitempty"`
	Error        *ToolError           `json:"error,omitempty"`
	SideEffects  []RecordedSideEffect `json:"sideEffects,omitempty"`
	DurationMS   int64                `json:"durationMs,omitempty"`
	ResourceUsage *ResourceUsage      `json:"resourceUsage,omitempty"`
	Metadata     map[string]any       `json:"metadata,omitempty"`
}

func NewToolSuccessResult(invocationID, toolID string) UnifiedToolResult {
	return UnifiedToolResult{
		InvocationID: invocationID,
		ToolID:       toolID,
		Status:       ToolResultStatusSuccess,
	}
}

func NewToolFailureResult(invocationID, toolID string, toolErr *ToolError) UnifiedToolResult {
	if toolErr == nil {
		toolErr = &ToolError{
			Code:     ErrorCodeExecutionFailed,
			Category: ToolErrorCategoryRuntime,
			Message:  "execution failed",
		}
	}
	return UnifiedToolResult{
		InvocationID: invocationID,
		ToolID:       toolID,
		Status:       ToolResultStatusFailed,
		Error:        NormalizeToolError(toolErr),
	}
}

func NewToolCancelledResult(invocationID, toolID string) UnifiedToolResult {
	return UnifiedToolResult{
		InvocationID: invocationID,
		ToolID:       toolID,
		Status:       ToolResultStatusCancelled,
		Error: &ToolError{
			Code:      ErrorCodeCancelled,
			Category:  ToolErrorCategoryCancellation,
			Message:   "execution was cancelled",
			Retryable: false,
		},
	}
}

func NewToolTimedOutResult(invocationID, toolID string) UnifiedToolResult {
	return UnifiedToolResult{
		InvocationID: invocationID,
		ToolID:       toolID,
		Status:       ToolResultStatusTimedOut,
		Error: &ToolError{
			Code:     ErrorCodeTimeout,
			Category: ToolErrorCategoryTimeout,
			Message:  "execution timed out",
		},
	}
}

func ResultFromContextError(invocationID string, err error) UnifiedToolResult {
	if errors.Is(err, context.DeadlineExceeded) {
		return NewToolTimedOutResult(invocationID, "")
	}
	if errors.Is(err, context.Canceled) {
		return NewToolCancelledResult(invocationID, "")
	}
	return NewToolFailureResult(invocationID, "", ToolErrorFromCause(err, ErrorCodeInternalError, "execution context error"))
}

func (r UnifiedToolResult) Clone() UnifiedToolResult {
	clone := UnifiedToolResult{
		InvocationID: r.InvocationID,
		ToolID:       r.ToolID,
		Status:       r.Status,
		DurationMS:   r.DurationMS,
	}

	if r.Content != nil {
		clone.Content = make([]ToolContent, len(r.Content))
		copy(clone.Content, r.Content)
	}

	if r.Structured != nil {
		clone.Structured = append(json.RawMessage(nil), r.Structured...)
	}

	if r.Error != nil {
		errCopy := *r.Error
		if r.Error.Details != nil {
			errCopy.Details = cloneStringAnyMap(r.Error.Details)
		}
		clone.Error = &errCopy
	}

	if r.SideEffects != nil {
		clone.SideEffects = make([]RecordedSideEffect, len(r.SideEffects))
		copy(clone.SideEffects, r.SideEffects)
	}

	if r.Metadata != nil {
		clone.Metadata = cloneStringAnyMap(r.Metadata)
	}

	if r.ResourceUsage != nil {
		usageCopy := *r.ResourceUsage
		if r.ResourceUsage.MeasuredDimensions != nil {
			usageCopy.MeasuredDimensions = append([]ResourceDimension(nil), r.ResourceUsage.MeasuredDimensions...)
		}
		clone.ResourceUsage = &usageCopy
	}

	return clone
}
