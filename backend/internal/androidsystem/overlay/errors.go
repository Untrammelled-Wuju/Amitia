package overlay

import (
	"github.com/u-ai/backend/internal/extension/kernel/capability"
)

const (
	OVERLAY_UNSUPPORTED                  = "OVERLAY_UNSUPPORTED"
	OVERLAY_PERMISSION_REQUIRED          = "OVERLAY_PERMISSION_REQUIRED"
	OVERLAY_PERMISSION_DENIED            = "OVERLAY_PERMISSION_DENIED"
	OVERLAY_NATIVE_HOST_UNAVAILABLE      = "OVERLAY_NATIVE_HOST_UNAVAILABLE"

	OVERLAY_INVALID_INPUT                = "OVERLAY_INVALID_INPUT"
	OVERLAY_INVALID_KIND                 = "OVERLAY_INVALID_KIND"
	OVERLAY_INVALID_RESOURCE             = "OVERLAY_INVALID_RESOURCE"

	OVERLAY_LIMIT_REACHED                = "OVERLAY_LIMIT_REACHED"
	OVERLAY_NOT_FOUND                    = "OVERLAY_NOT_FOUND"
	OVERLAY_ALREADY_CLOSED               = "OVERLAY_ALREADY_CLOSED"

	OVERLAY_CREATE_FAILED                = "OVERLAY_CREATE_FAILED"
	OVERLAY_UPDATE_FAILED                = "OVERLAY_UPDATE_FAILED"
	OVERLAY_SHOW_FAILED                  = "OVERLAY_SHOW_FAILED"
	OVERLAY_HIDE_FAILED                  = "OVERLAY_HIDE_FAILED"
	OVERLAY_CLOSE_FAILED                 = "OVERLAY_CLOSE_FAILED"

	OVERLAY_TIMEOUT                      = "OVERLAY_TIMEOUT"
	OVERLAY_CANCELLED                    = "OVERLAY_CANCELLED"
	OVERLAY_INVALID_RESPONSE             = "OVERLAY_INVALID_RESPONSE"
)

func MapError(code string, message string) *capability.ToolError {
	toolErr := &capability.ToolError{
		Code:        capability.ErrorCodeExecutionFailed,
		Message:     message,
		UserVisible: true,
	}

	switch code {
	case OVERLAY_PERMISSION_REQUIRED, OVERLAY_PERMISSION_DENIED:
		toolErr.Code = capability.ErrorCodePermissionDenied
		toolErr.Retryable = false
	case OVERLAY_UNSUPPORTED, OVERLAY_NATIVE_HOST_UNAVAILABLE:
		toolErr.Code = capability.ErrorCodeNotAvailable
		toolErr.Retryable = false
	case OVERLAY_INVALID_INPUT, OVERLAY_INVALID_KIND, OVERLAY_INVALID_RESOURCE:
		toolErr.Code = capability.ErrorCodeInvalidInput
		toolErr.Retryable = false
	case OVERLAY_LIMIT_REACHED:
		toolErr.Code = capability.ErrorCodeConflict
		toolErr.Retryable = false
	case OVERLAY_NOT_FOUND:
		toolErr.Code = capability.ErrorCodeNotAvailable
		toolErr.Retryable = false
	case OVERLAY_ALREADY_CLOSED:
		toolErr.Code = capability.ErrorCodeConflict
		toolErr.Retryable = false
	case OVERLAY_TIMEOUT:
		toolErr.Code = capability.ErrorCodeTimeout
		toolErr.Retryable = true
	case OVERLAY_CANCELLED:
		toolErr.Code = capability.ErrorCodeCancelled
		toolErr.Retryable = false
	case OVERLAY_INVALID_RESPONSE:
		toolErr.Code = capability.ErrorCodeInternalError
		toolErr.Retryable = false
	case OVERLAY_CREATE_FAILED, OVERLAY_UPDATE_FAILED, OVERLAY_SHOW_FAILED, OVERLAY_HIDE_FAILED, OVERLAY_CLOSE_FAILED:
		toolErr.Code = capability.ErrorCodeExecutionFailed
		toolErr.Retryable = false
	}

	return toolErr
}

type overlayError struct {
	code    string
	message string
}

func (e *overlayError) Error() string  { return e.message }
func (e *overlayError) Code() string   { return e.code }

func newOverlayError(code string, message string) *overlayError {
	return &overlayError{code: code, message: message}
}
