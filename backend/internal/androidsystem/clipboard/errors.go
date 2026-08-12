package clipboard

import (
	"github.com/u-ai/backend/internal/extension/kernel/capability"
)

const (
	CLIPBOARD_UNSUPPORTED              = "CLIPBOARD_UNSUPPORTED"
	CLIPBOARD_HOST_UNAVAILABLE         = "CLIPBOARD_HOST_UNAVAILABLE"
	CLIPBOARD_PERMISSION_DENIED        = "CLIPBOARD_PERMISSION_DENIED"
	CLIPBOARD_READ_FOREGROUND_REQUIRED = "CLIPBOARD_READ_FOREGROUND_REQUIRED"
	CLIPBOARD_READ_FOCUS_REQUIRED      = "CLIPBOARD_READ_FOCUS_REQUIRED"
	CLIPBOARD_FOREGROUND_STATE_UNKNOWN = "CLIPBOARD_FOREGROUND_STATE_UNKNOWN"
	CLIPBOARD_EMPTY                    = "CLIPBOARD_EMPTY"
	CLIPBOARD_CONTENT_TYPE_UNSUPPORTED = "CLIPBOARD_CONTENT_TYPE_UNSUPPORTED"
	CLIPBOARD_CONTENT_TOO_LARGE        = "CLIPBOARD_CONTENT_TOO_LARGE"
	CLIPBOARD_INPUT_TOO_LARGE          = "CLIPBOARD_INPUT_TOO_LARGE"
	CLIPBOARD_READ_FAILED              = "CLIPBOARD_READ_FAILED"
	CLIPBOARD_WRITE_FAILED             = "CLIPBOARD_WRITE_FAILED"
	CLIPBOARD_CLEAR_FAILED             = "CLIPBOARD_CLEAR_FAILED"
)

const (
	PermissionRead  = "clipboard.read"
	PermissionWrite = "clipboard.write"
)

func MapError(code string, message string) *capability.ToolError {
	toolErr := &capability.ToolError{
		Code:        capability.ErrorCodeExecutionFailed,
		Message:     message,
		UserVisible: true,
	}

	switch code {
	case CLIPBOARD_PERMISSION_DENIED:
		toolErr.Code = capability.ErrorCodePermissionDenied
		toolErr.Retryable = false
	case CLIPBOARD_HOST_UNAVAILABLE, CLIPBOARD_UNSUPPORTED:
		toolErr.Code = capability.ErrorCodeNotAvailable
		toolErr.Retryable = false
	case CLIPBOARD_READ_FOREGROUND_REQUIRED, CLIPBOARD_READ_FOCUS_REQUIRED:
		toolErr.Code = capability.ErrorCodePermissionDenied
		toolErr.Retryable = false
	case CLIPBOARD_FOREGROUND_STATE_UNKNOWN:
		toolErr.Code = capability.ErrorCodeNotAvailable
		toolErr.Retryable = false
	case CLIPBOARD_CONTENT_TYPE_UNSUPPORTED:
		toolErr.Code = capability.ErrorCodeInvalidInput
		toolErr.Retryable = false
	case CLIPBOARD_CONTENT_TOO_LARGE:
		toolErr.Code = capability.ErrorCodeInvalidInput
		toolErr.Retryable = false
	case CLIPBOARD_INPUT_TOO_LARGE:
		toolErr.Code = capability.ErrorCodeInvalidInput
		toolErr.Retryable = false
	case CLIPBOARD_READ_FAILED, CLIPBOARD_WRITE_FAILED, CLIPBOARD_CLEAR_FAILED:
		toolErr.Code = capability.ErrorCodeExecutionFailed
		toolErr.Retryable = false
	}

	return toolErr
}
