package externalautomation

import (
	"github.com/u-ai/backend/internal/extension/kernel/capability"
)

const (
	AUTOMATION_UNSUPPORTED              = "AUTOMATION_UNSUPPORTED"
	AUTOMATION_INVALID_REQUEST          = "AUTOMATION_INVALID_REQUEST"

	AUTOMATION_APP_NOT_FOUND            = "AUTOMATION_APP_NOT_FOUND"
	AUTOMATION_APP_NOT_LAUNCHABLE       = "AUTOMATION_APP_NOT_LAUNCHABLE"

	AUTOMATION_URI_INVALID              = "AUTOMATION_URI_INVALID"
	AUTOMATION_URI_SCHEME_BLOCKED       = "AUTOMATION_URI_SCHEME_BLOCKED"
	AUTOMATION_URI_NOT_RESOLVABLE       = "AUTOMATION_URI_NOT_RESOLVABLE"
	AUTOMATION_URI_TOO_LARGE            = "AUTOMATION_URI_TOO_LARGE"

	AUTOMATION_SETTINGS_UNSUPPORTED     = "AUTOMATION_SETTINGS_UNSUPPORTED"

	AUTOMATION_INTENT_ACTION_BLOCKED    = "AUTOMATION_INTENT_ACTION_BLOCKED"
	AUTOMATION_COMPONENT_NOT_FOUND      = "AUTOMATION_COMPONENT_NOT_FOUND"
	AUTOMATION_COMPONENT_NOT_EXPORTED   = "AUTOMATION_COMPONENT_NOT_EXPORTED"
	AUTOMATION_TARGET_PERMISSION_DENIED = "AUTOMATION_TARGET_PERMISSION_DENIED"
	AUTOMATION_PERMISSION_DENIED        = "AUTOMATION_PERMISSION_DENIED"

	AUTOMATION_BACKGROUND_START_RESTRICTED = "AUTOMATION_BACKGROUND_START_RESTRICTED"

	AUTOMATION_FOREGROUND_UNAVAILABLE   = "AUTOMATION_FOREGROUND_UNAVAILABLE"
	AUTOMATION_FOREGROUND_TIMEOUT       = "AUTOMATION_FOREGROUND_TIMEOUT"

	AUTOMATION_NATIVE_HOST_UNAVAILABLE  = "AUTOMATION_NATIVE_HOST_UNAVAILABLE"
	AUTOMATION_TIMEOUT                  = "AUTOMATION_TIMEOUT"
	AUTOMATION_CANCELLED                = "AUTOMATION_CANCELLED"
	AUTOMATION_INVALID_RESPONSE         = "AUTOMATION_INVALID_RESPONSE"
)

func MapError(code string, message string) *capability.ToolError {
	toolErr := &capability.ToolError{
		Code:        capability.ErrorCodeExecutionFailed,
		Message:     message,
		UserVisible: true,
	}

	switch code {
	case AUTOMATION_PERMISSION_DENIED, AUTOMATION_TARGET_PERMISSION_DENIED, AUTOMATION_COMPONENT_NOT_EXPORTED:
		toolErr.Code = capability.ErrorCodePermissionDenied
		toolErr.Retryable = false
	case AUTOMATION_UNSUPPORTED, AUTOMATION_NATIVE_HOST_UNAVAILABLE, AUTOMATION_FOREGROUND_UNAVAILABLE:
		toolErr.Code = capability.ErrorCodeNotAvailable
		toolErr.Retryable = false
	case AUTOMATION_INVALID_REQUEST, AUTOMATION_URI_INVALID, AUTOMATION_URI_TOO_LARGE:
		toolErr.Code = capability.ErrorCodeInvalidInput
		toolErr.Retryable = false
	case AUTOMATION_URI_SCHEME_BLOCKED, AUTOMATION_INTENT_ACTION_BLOCKED:
		toolErr.Code = capability.ErrorCodeInvalidInput
		toolErr.Retryable = false
	case AUTOMATION_APP_NOT_FOUND, AUTOMATION_APP_NOT_LAUNCHABLE,
		AUTOMATION_COMPONENT_NOT_FOUND, AUTOMATION_URI_NOT_RESOLVABLE:
		toolErr.Code = capability.ErrorCodeNotAvailable
		toolErr.Retryable = false
	case AUTOMATION_SETTINGS_UNSUPPORTED:
		toolErr.Code = capability.ErrorCodeNotAvailable
		toolErr.Retryable = false
	case AUTOMATION_BACKGROUND_START_RESTRICTED:
		toolErr.Code = capability.ErrorCodePermissionDenied
		toolErr.Retryable = false
	case AUTOMATION_TIMEOUT, AUTOMATION_FOREGROUND_TIMEOUT:
		toolErr.Code = capability.ErrorCodeTimeout
		toolErr.Retryable = true
	case AUTOMATION_CANCELLED:
		toolErr.Code = capability.ErrorCodeCancelled
		toolErr.Retryable = false
	case AUTOMATION_INVALID_RESPONSE:
		toolErr.Code = capability.ErrorCodeInternalError
		toolErr.Retryable = false
	}

	return toolErr
}

type automationError struct {
	code    string
	message string
}

func (e *automationError) Error() string { return e.message }
func (e *automationError) Code() string  { return e.code }

func newAutomationError(code string, message string) *automationError {
	return &automationError{code: code, message: message}
}
