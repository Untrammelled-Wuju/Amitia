package capability

import (
	"context"
	"encoding/json"
)

const iosBridgeProtocolVersion = 1

type IOSBridgeRequest struct {
	ProtocolVersion int            `json:"protocolVersion"`
	RequestID       string         `json:"requestId"`
	Operation       string         `json:"operation"`
	Payload         map[string]any `json:"payload,omitempty"`
}

type IOSBridgeResponse struct {
	ProtocolVersion int            `json:"protocolVersion"`
	RequestID       string         `json:"requestId"`
	Status          string         `json:"status"`
	Result          map[string]any `json:"result,omitempty"`
	Error           *IOSError      `json:"error,omitempty"`
}

type IOSError struct {
	Code       string `json:"code"`
	Message    string `json:"message"`
	DomainCode string `json:"domainCode,omitempty"`
}

type IOSProvider interface {
	Execute(ctx context.Context, request IOSBridgeRequest) IOSBridgeResponse
	Health(ctx context.Context) HealthStatus
}

type IOSCancellableProvider interface {
	Cancel(ctx context.Context, requestID string, reason string) error
}

type iosRuntimeAdapter struct {
	provider IOSProvider
}

func NewIOSRuntimeAdapter(provider IOSProvider) *iosRuntimeAdapter {
	return &iosRuntimeAdapter{provider: provider}
}

func (a *iosRuntimeAdapter) Supports(binding RuntimeBinding) bool {
	return binding.RuntimeType == RuntimeTypeIOS_Native
}

func (a *iosRuntimeAdapter) Execute(
	ctx context.Context,
	binding RuntimeBinding,
	invocation ToolInvocationContext,
	input json.RawMessage,
) UnifiedToolResult {
	if a.provider == nil {
		return UnifiedToolResult{
			InvocationID: invocation.InvocationID,
			Status:       ToolResultStatusFailed,
			Error: &ToolError{
				Code:        ErrorCodeNotAvailable,
				Message:     "ios native provider not available",
				UserVisible: true,
			},
		}
	}

	if err := ctx.Err(); err != nil {
		return a.handleCtxError(invocation.InvocationID, err)
	}

	payload := map[string]any{}
	if len(input) > 0 {
		if err := json.Unmarshal(input, &payload); err != nil {
			return UnifiedToolResult{
				InvocationID: invocation.InvocationID,
				Status:       ToolResultStatusFailed,
				Error: &ToolError{
					Code:        ErrorCodeInvalidInput,
					Message:     "invalid ios provider input: " + err.Error(),
					UserVisible: true,
				},
			}
		}
	}

	request := IOSBridgeRequest{
		ProtocolVersion: iosBridgeProtocolVersion,
		RequestID:       invocation.InvocationID,
		Operation:       binding.HandlerName,
		Payload:         payload,
	}

	done := make(chan IOSBridgeResponse, 1)
	go func() {
		done <- a.provider.Execute(ctx, request)
	}()

	select {
	case <-ctx.Done():
		return a.handleCtxError(invocation.InvocationID, ctx.Err())
	case response := <-done:
		return a.normalizeResponse(invocation.InvocationID, response)
	}
}

func (a *iosRuntimeAdapter) Health(ctx context.Context, binding RuntimeBinding) HealthStatus {
	if a.provider == nil {
		return HealthUnhealthy
	}
	done := make(chan HealthStatus, 1)
	go func() {
		done <- a.provider.Health(ctx)
	}()
	select {
	case <-ctx.Done():
		return HealthUnknown
	case status := <-done:
		return status
	}
}

func (a *iosRuntimeAdapter) handleCtxError(invocationID string, err error) UnifiedToolResult {
	if err == context.DeadlineExceeded {
		return UnifiedToolResult{
			InvocationID: invocationID,
			Status:       ToolResultStatusTimedOut,
			Error: &ToolError{
				Code:        ErrorCodeTimeout,
				Message:     "ios provider execution timed out",
				UserVisible: true,
				Retryable:   true,
			},
		}
	}
	return UnifiedToolResult{
		InvocationID: invocationID,
		Status:       ToolResultStatusCancelled,
		Error: &ToolError{
			Code:        ErrorCodeCancelled,
			Message:     "ios provider execution cancelled",
			UserVisible: true,
		},
	}
}

func (a *iosRuntimeAdapter) normalizeResponse(invocationID string, resp IOSBridgeResponse) UnifiedToolResult {
	result := UnifiedToolResult{
		InvocationID: invocationID,
	}

	switch resp.Status {
	case "success":
		result.Status = ToolResultStatusSuccess
		if resp.Result != nil {
			structured, err := json.Marshal(resp.Result)
			if err == nil {
				result.Structured = structured
			}
			if text, ok := resp.Result["text"].(string); ok {
				result.Content = []ToolContent{{Type: ToolContentText, Text: text}}
			}
			if uri, ok := resp.Result["resourceUri"].(string); ok {
				result.Content = append(result.Content, ToolContent{
					Type: ToolContentResourceReference,
					URI:  uri,
				})
			}
		}
		return result
	case "cancelled":
		result.Status = ToolResultStatusCancelled
		result.Error = &ToolError{
			Code:        ErrorCodeCancelled,
			Message:     "ios provider execution cancelled",
			UserVisible: true,
		}
		return result
	case "timeout":
		result.Status = ToolResultStatusTimedOut
		result.Error = &ToolError{
			Code:        ErrorCodeTimeout,
			Message:     "ios provider execution timed out",
			UserVisible: true,
			Retryable:   true,
		}
		return result
	default:
		result.Status = ToolResultStatusFailed
		result.Error = a.mapIOSError(resp.Error)
		return result
	}
}

func (a *iosRuntimeAdapter) Cancel(ctx context.Context, binding RuntimeBinding, invocation ToolInvocationContext, reason ToolCancellationReason) error {
	cancellable, ok := a.provider.(IOSCancellableProvider)
	if !ok {
		return ErrRuntimeCancellationUnsupported{}
	}
	return cancellable.Cancel(ctx, invocation.InvocationID, string(reason.Code))
}

func (a *iosRuntimeAdapter) mapIOSError(iosErr *IOSError) *ToolError {
	if iosErr == nil {
		return &ToolError{
			Code:        ErrorCodeExecutionFailed,
			Message:     "ios provider returned unknown error",
			UserVisible: true,
		}
	}

	toolErr := &ToolError{
		Code:        ErrorCodeExecutionFailed,
		Message:     iosErr.Message,
		UserVisible: true,
	}

	if iosErr.DomainCode != "" {
		toolErr.Details = map[string]any{
			"domainCode": iosErr.DomainCode,
		}
	}

	switch iosErr.Code {
	case "PROVIDER_UNAVAILABLE":
		toolErr.Code = ErrorCodeNotAvailable
		toolErr.Retryable = false
	case "AUTHORIZATION_DENIED":
		toolErr.Code = ErrorCodePermissionDenied
		toolErr.Retryable = false
	case "USER_ACTION_REQUIRED":
		toolErr.Code = ErrorCodePermissionDenied
		toolErr.Retryable = false
	case "PLATFORM_NOT_SUPPORTED":
		toolErr.Code = ErrorCodeNotAvailable
		toolErr.Retryable = false
	case "BRIDGE_DISCONNECTED":
		toolErr.Code = ErrorCodeConnectionLost
		toolErr.Retryable = true
	case "BRIDGE_TIMEOUT":
		toolErr.Code = ErrorCodeTimeout
		toolErr.Retryable = true
	case "BRIDGE_INVALID_RESPONSE":
		toolErr.Code = ErrorCodeInternalError
		toolErr.Retryable = false
	case "FOREGROUND_REQUIRED":
		toolErr.Code = ErrorCodePermissionDenied
		toolErr.Retryable = false
	case "PERMISSION_REQUIRED":
		toolErr.Code = ErrorCodePermissionDenied
		toolErr.Retryable = false
	case "UNSUPPORTED":
		toolErr.Code = ErrorCodeNotAvailable
		toolErr.Retryable = false
	}

	return toolErr
}
