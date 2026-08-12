package notification

import (
	"github.com/u-ai/backend/internal/androidsystem"
	"github.com/u-ai/backend/internal/extension/kernel/capability"
)

func MapToToolError(code string, message string) *capability.ToolError {
	notifErr := &androidsystem.NotificationError{
		Code:    code,
		Message: message,
	}
	return androidsystem.MapNotificationError(notifErr)
}

func ListenerPermissionError() *capability.ToolError {
	return &capability.ToolError{
		Code:        capability.ErrorCodePermissionDenied,
		Message:    "notification listener access not granted by user",
		UserVisible: true,
		Retryable:   false,
	}
}

func PostPermissionError() *capability.ToolError {
	return &capability.ToolError{
		Code:        capability.ErrorCodePermissionDenied,
		Message:    "notification post permission not granted",
		UserVisible: true,
		Retryable:   false,
	}
}

func NotConnectedError() *capability.ToolError {
	return &capability.ToolError{
		Code:        capability.ErrorCodeNotAvailable,
		Message:    "notification listener not connected",
		UserVisible: true,
		Retryable:   false,
	}
}

func NotFoundError() *capability.ToolError {
	return &capability.ToolError{
		Code:        capability.ErrorCodeNotAvailable,
		Message:    "notification not found",
		UserVisible: true,
		Retryable:   false,
	}
}

func StaleReferenceError() *capability.ToolError {
	return &capability.ToolError{
		Code:        capability.ErrorCodeConflict,
		Message:    "notification reference is stale after listener reconnect",
		UserVisible: true,
		Retryable:   false,
	}
}

func NotDismissibleError() *capability.ToolError {
	return &capability.ToolError{
		Code:        capability.ErrorCodePermissionDenied,
		Message:    "notification is not dismissible",
		UserVisible: true,
		Retryable:   false,
	}
}

func ContentActionUnavailableError() *capability.ToolError {
	return &capability.ToolError{
		Code:        capability.ErrorCodeNotAvailable,
		Message:    "notification has no content action",
		UserVisible: true,
		Retryable:   false,
	}
}

func RemoteInputUnsupportedError() *capability.ToolError {
	return &capability.ToolError{
		Code:        capability.ErrorCodeNotAvailable,
		Message:    "notification remote input actions are not supported",
		UserVisible: true,
		Retryable:   false,
	}
}

func PostDisabledError() *capability.ToolError {
	return &capability.ToolError{
		Code:        capability.ErrorCodePermissionDenied,
		Message:    "notifications are disabled for this app",
		UserVisible: true,
		Retryable:   false,
	}
}

func SettingsUnavailableError() *capability.ToolError {
	return &capability.ToolError{
		Code:        capability.ErrorCodeNotAvailable,
		Message:    "notification settings activity is not available",
		UserVisible: true,
		Retryable:   false,
	}
}

func OperationNotSupportedError() *capability.ToolError {
	return &capability.ToolError{
		Code:        capability.ErrorCodeNotAvailable,
		Message:    "notification operation not supported",
		UserVisible: true,
		Retryable:   false,
	}
}

func BlockedError(reason string) *capability.ToolError {
	return &capability.ToolError{
		Code:        capability.ErrorCodeNotAvailable,
		Message:    reason,
		UserVisible: true,
		Retryable:   false,
	}
}
