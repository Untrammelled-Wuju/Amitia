package share

import (
	"github.com/u-ai/backend/internal/extension/kernel/capability"
)

const (
	SHARE_UNAVAILABLE                = "SHARE_UNAVAILABLE"
	SHARE_BUSY                       = "SHARE_BUSY"
	SHARE_NO_TARGET                  = "SHARE_NO_TARGET"
	SHARE_INVALID_INPUT              = "SHARE_INVALID_INPUT"
	SHARE_TEXT_TOO_LARGE             = "SHARE_TEXT_TOO_LARGE"
	SHARE_SUBJECT_TOO_LARGE          = "SHARE_SUBJECT_TOO_LARGE"
	SHARE_CHOOSER_TITLE_TOO_LARGE    = "SHARE_CHOOSER_TITLE_TOO_LARGE"
	SHARE_TOO_MANY_RESOURCES         = "SHARE_TOO_MANY_RESOURCES"
	SHARE_RESOURCE_NOT_FOUND         = "SHARE_RESOURCE_NOT_FOUND"
	SHARE_RESOURCE_DENIED            = "SHARE_RESOURCE_DENIED"
	SHARE_RESOURCE_TOO_LARGE         = "SHARE_RESOURCE_TOO_LARGE"
	SHARE_TOTAL_TOO_LARGE            = "SHARE_TOTAL_TOO_LARGE"
	SHARE_MIME_UNSUPPORTED           = "SHARE_MIME_UNSUPPORTED"
	SHARE_MIXED_MIME_UNSUPPORTED     = "SHARE_MIXED_MIME_UNSUPPORTED"
	SHARE_EXPORT_FAILED              = "SHARE_EXPORT_FAILED"
	SHARE_CONTENT_URI_FAILED         = "SHARE_CONTENT_URI_FAILED"
	SHARE_UI_CONTEXT_REQUIRED        = "SHARE_UI_CONTEXT_REQUIRED"
	SHARE_INBOUND_INVALID            = "SHARE_INBOUND_INVALID"
	SHARE_INBOUND_TOO_LARGE          = "SHARE_INBOUND_TOO_LARGE"
	SHARE_INBOUND_IMPORT_FAILED      = "SHARE_INBOUND_IMPORT_FAILED"
	SHARE_CANCELLED                  = "SHARE_CANCELLED"
	SHARE_PERMISSION_DENIED          = "SHARE_PERMISSION_DENIED"
	SHARE_RESOURCE_SCOPE_DENIED      = "SHARE_RESOURCE_SCOPE_DENIED"
)

func MapError(code string, message string) *capability.ToolError {
	toolErr := &capability.ToolError{
		Code:        capability.ErrorCodeExecutionFailed,
		Message:     message,
		UserVisible: true,
		Retryable:   false,
	}

	switch code {
	case SHARE_UNAVAILABLE, SHARE_BUSY, SHARE_UI_CONTEXT_REQUIRED:
		toolErr.Code = capability.ErrorCodeNotAvailable
	case SHARE_PERMISSION_DENIED:
		toolErr.Code = capability.ErrorCodePermissionDenied
	case SHARE_RESOURCE_DENIED, SHARE_RESOURCE_SCOPE_DENIED:
		toolErr.Code = capability.ErrorCodePermissionDenied
	case SHARE_TEXT_TOO_LARGE, SHARE_SUBJECT_TOO_LARGE, SHARE_CHOOSER_TITLE_TOO_LARGE,
		SHARE_TOO_MANY_RESOURCES, SHARE_RESOURCE_TOO_LARGE, SHARE_TOTAL_TOO_LARGE,
		SHARE_INBOUND_TOO_LARGE, SHARE_INVALID_INPUT:
		toolErr.Code = capability.ErrorCodeInvalidInput
	case SHARE_RESOURCE_NOT_FOUND:
		toolErr.Code = capability.ErrorCodeNotAvailable
	case SHARE_MIME_UNSUPPORTED, SHARE_MIXED_MIME_UNSUPPORTED:
		toolErr.Code = capability.ErrorCodeInvalidInput
	case SHARE_NO_TARGET:
		toolErr.Code = capability.ErrorCodeNotAvailable
	case SHARE_CANCELLED:
		toolErr.Code = capability.ErrorCodeCancelled
	case SHARE_INBOUND_INVALID, SHARE_INBOUND_IMPORT_FAILED:
		toolErr.Code = capability.ErrorCodeNotAvailable
	}

	return toolErr
}
