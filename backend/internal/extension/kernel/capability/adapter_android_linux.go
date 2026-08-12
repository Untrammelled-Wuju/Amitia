//go:build linux

package capability

import (
	"context"
	"encoding/json"

	"github.com/u-ai/backend/internal/androidlinux/terminal"
)

type androidLinuxRuntimeAdapter struct {
	provider terminal.AndroidLinuxProvider
}

func NewAndroidLinuxRuntimeAdapter(provider terminal.AndroidLinuxProvider) *androidLinuxRuntimeAdapter {
	return &androidLinuxRuntimeAdapter{provider: provider}
}

func (a *androidLinuxRuntimeAdapter) Supports(binding RuntimeBinding) bool {
	return binding.RuntimeType == RuntimeTypeAndroidLinux
}

func (a *androidLinuxRuntimeAdapter) Execute(
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
				Message:     "android linux provider not available",
				UserVisible: true,
			},
		}
	}

	if err := ctx.Err(); err != nil {
		return handleCtxError(invocation.InvocationID, err)
	}

	payload := map[string]any{}
	if len(input) > 0 {
		if err := json.Unmarshal(input, &payload); err != nil {
			return UnifiedToolResult{
				InvocationID: invocation.InvocationID,
				Status:       ToolResultStatusFailed,
				Error: &ToolError{
					Code:        ErrorCodeInvalidInput,
					Message:     "invalid terminal input: " + err.Error(),
					UserVisible: true,
				},
			}
		}
	}

	payload["userId"] = invocation.UserID
	payload["characterId"] = invocation.CharacterID
	payload["conversationId"] = invocation.ConversationID

	request := terminal.AndroidLinuxRequest{
		RequestID: invocation.InvocationID,
		Operation: binding.HandlerName,
		SessionID: extractSessionID(payload),
		Payload:   payload,
	}

	done := make(chan terminal.AndroidLinuxResponse, 1)
	go func() {
		done <- a.provider.Execute(ctx, request)
	}()

	select {
	case <-ctx.Done():
		return handleCtxError(invocation.InvocationID, ctx.Err())
	case response := <-done:
		return a.normalizeResponse(invocation.InvocationID, response)
	}
}

func (a *androidLinuxRuntimeAdapter) Health(ctx context.Context, binding RuntimeBinding) HealthStatus {
	if a.provider == nil {
		return HealthUnhealthy
	}

	done := make(chan terminal.HealthStatus, 1)
	go func() {
		done <- a.provider.Health(ctx)
	}()

	select {
	case <-ctx.Done():
		return HealthUnknown
	case status := <-done:
		return mapHealthStatus(status)
	}
}

func (a *androidLinuxRuntimeAdapter) Cancel(ctx context.Context, binding RuntimeBinding, invocation ToolInvocationContext, reason ToolCancellationReason) error {
	cancellable, ok := a.provider.(terminal.AndroidLinuxCancellableProvider)
	if !ok {
		return ErrRuntimeCancellationUnsupported{}
	}
	return cancellable.CancelOp(ctx, invocation.InvocationID, string(reason.Code))
}

func (a *androidLinuxRuntimeAdapter) normalizeResponse(invocationID string, resp terminal.AndroidLinuxResponse) UnifiedToolResult {
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
		}
		return result
	case "cancelled":
		result.Status = ToolResultStatusCancelled
		result.Error = &ToolError{
			Code:        ErrorCodeCancelled,
			Message:     "terminal execution cancelled",
			UserVisible: true,
		}
		return result
	case "timeout":
		result.Status = ToolResultStatusTimedOut
		result.Error = &ToolError{
			Code:        ErrorCodeTimeout,
			Message:     "terminal execution timed out",
			UserVisible: true,
			Retryable:   true,
		}
		return result
	default:
		result.Status = ToolResultStatusFailed
		result.Error = mapTerminalError(resp.Error)
		return result
	}
}

func mapTerminalError(termErr *terminal.AndroidLinuxError) *ToolError {
	if termErr == nil {
		return &ToolError{
			Code:        ErrorCodeExecutionFailed,
			Message:     "terminal operation failed",
			UserVisible: true,
		}
	}

	toolErr := &ToolError{
		Code:        ErrorCodeExecutionFailed,
		Message:     termErr.Message,
		UserVisible: true,
	}

	switch termErr.Code {
	case terminal.ErrCodeNotAvailable:
		toolErr.Code = ErrorCodeNotAvailable
		toolErr.Retryable = false
	case terminal.ErrCodeSessionNotFound:
		toolErr.Code = ErrorCodeNotAvailable
		toolErr.Retryable = false
	case terminal.ErrCodeSessionLimit:
		toolErr.Code = ErrorCodeResourceLimitExceeded
		toolErr.Retryable = false
	case terminal.ErrCodeScopeDenied:
		toolErr.Code = ErrorCodeScopeDenied
		toolErr.Retryable = false
	case terminal.ErrCodeNotRunning:
		toolErr.Code = ErrorCodeExecutionFailed
		toolErr.Retryable = false
	case terminal.ErrCodeInputTooLarge:
		toolErr.Code = ErrorCodeInvalidInput
		toolErr.Retryable = false
	case terminal.ErrCodeOutputLimit:
		toolErr.Code = ErrorCodeExecutionFailed
		toolErr.Retryable = false
	case terminal.ErrCodeInvalidSize:
		toolErr.Code = ErrorCodeInvalidInput
		toolErr.Retryable = false
	case terminal.ErrCodeStartFailed:
		toolErr.Code = ErrorCodeExecutionFailed
		toolErr.Retryable = true
	case terminal.ErrCodeIOFailed:
		toolErr.Code = ErrorCodeExecutionFailed
		toolErr.Retryable = true
	case terminal.ErrCodeCancelled:
		toolErr.Code = ErrorCodeCancelled
		toolErr.Retryable = false
	case terminal.ErrCodeExited:
		toolErr.Code = ErrorCodeExecutionFailed
		toolErr.Retryable = false
	}

	return toolErr
}

func mapHealthStatus(status terminal.HealthStatus) HealthStatus {
	switch status {
	case terminal.HealthReady:
		return HealthReady
	case terminal.HealthUnhealthy:
		return HealthUnhealthy
	default:
		return HealthUnknown
	}
}

func handleCtxError(invocationID string, err error) UnifiedToolResult {
	if err == context.DeadlineExceeded {
		return UnifiedToolResult{
			InvocationID: invocationID,
			Status:       ToolResultStatusTimedOut,
			Error: &ToolError{
				Code:        ErrorCodeTimeout,
				Message:     "terminal execution timed out",
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
			Message:     "terminal execution cancelled",
			UserVisible: true,
		},
	}
}

func extractSessionID(payload map[string]any) terminal.SessionID {
	if sid, ok := payload["sessionId"].(string); ok {
		return terminal.SessionID(sid)
	}
	return ""
}

var _ RuntimeAdapter = (*androidLinuxRuntimeAdapter)(nil)
