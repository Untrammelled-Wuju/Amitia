package capability

import (
	"context"
	"encoding/json"
)

const desktopBridgeProtocolVersion = 1

type DesktopBridgeRequest struct {
	ProtocolVersion int            `json:"protocolVersion"`
	RequestID       string         `json:"requestId"`
	Operation       string         `json:"operation"`
	Payload         map[string]any `json:"payload,omitempty"`
}

type DesktopBridgeResponse struct {
	ProtocolVersion int            `json:"protocolVersion"`
	RequestID       string         `json:"requestId"`
	Status          string         `json:"status"`
	Result          map[string]any `json:"result,omitempty"`
	Error           *DesktopError  `json:"error,omitempty"`
}

type DesktopError struct {
	Code       string `json:"code"`
	Message    string `json:"message"`
	DomainCode string `json:"domainCode,omitempty"`
}

type DesktopProvider interface {
	Execute(ctx context.Context, request DesktopBridgeRequest) DesktopBridgeResponse
	Health(ctx context.Context) HealthStatus
}

type desktopRuntimeAdapter struct {
	provider DesktopProvider
}

func NewDesktopRuntimeAdapter(provider DesktopProvider) *desktopRuntimeAdapter {
	return &desktopRuntimeAdapter{provider: provider}
}

func (a *desktopRuntimeAdapter) Supports(binding RuntimeBinding) bool {
	return binding.RuntimeType == RuntimeTypeDesktop_Extension
}

func (a *desktopRuntimeAdapter) Execute(
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
				Message:     "desktop extension provider not available",
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
					Message:     "invalid desktop provider input: " + err.Error(),
					UserVisible: true,
				},
			}
		}
	}

	request := DesktopBridgeRequest{
		ProtocolVersion: desktopBridgeProtocolVersion,
		RequestID:       invocation.InvocationID,
		Operation:       binding.HandlerName,
		Payload:         payload,
	}

	done := make(chan DesktopBridgeResponse, 1)
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

func (a *desktopRuntimeAdapter) Health(ctx context.Context, binding RuntimeBinding) HealthStatus {
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

func (a *desktopRuntimeAdapter) handleCtxError(invocationID string, err error) UnifiedToolResult {
	if err == context.DeadlineExceeded {
		return UnifiedToolResult{
			InvocationID: invocationID,
			Status:       ToolResultStatusTimedOut,
			Error: &ToolError{
				Code:        ErrorCodeTimeout,
				Message:     "desktop provider execution timed out",
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
			Message:     "desktop provider execution cancelled",
			UserVisible: true,
		},
	}
}

func (a *desktopRuntimeAdapter) normalizeResponse(invocationID string, resp DesktopBridgeResponse) UnifiedToolResult {
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
			Message:     "desktop provider execution cancelled",
			UserVisible: true,
		}
		return result
	case "timeout":
		result.Status = ToolResultStatusTimedOut
		result.Error = &ToolError{
			Code:        ErrorCodeTimeout,
			Message:     "desktop provider execution timed out",
			UserVisible: true,
			Retryable:   true,
		}
		return result
	default:
		result.Status = ToolResultStatusFailed
		result.Error = a.mapDesktopError(resp.Error)
		return result
	}
}

func (a *desktopRuntimeAdapter) mapDesktopError(desktopErr *DesktopError) *ToolError {
	if desktopErr == nil {
		return &ToolError{
			Code:        ErrorCodeExecutionFailed,
			Message:     "desktop provider returned unknown error",
			UserVisible: true,
		}
	}

	toolErr := &ToolError{
		Code:        ErrorCodeExecutionFailed,
		Message:     desktopErr.Message,
		UserVisible: true,
	}

	if desktopErr.DomainCode != "" {
		toolErr.Details = map[string]any{
			"domainCode": desktopErr.DomainCode,
		}
	}

	switch desktopErr.Code {
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
	case "CONFLICT":
		toolErr.Code = ErrorCodeConflict
		toolErr.Retryable = false
	case "NOT_FOUND":
		toolErr.Code = ErrorCodeNotAvailable
		toolErr.Retryable = false
	}

	return toolErr
}
