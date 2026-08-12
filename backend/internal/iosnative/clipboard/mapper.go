package clipboard

import "github.com/u-ai/backend/internal/nativebridge"

func MapErrorToNativeBridge(err error) (string, string) {
	if err == nil {
		return "", ""
	}
	return ErrOutcomeUnknown, err.Error()
}

func MapCodeToMessage(code string) string {
	switch code {
	case ErrClipboardUnsupported:
		return "clipboard is not supported on this device"
	case ErrClipboardUnavailable:
		return "clipboard is currently unavailable"
	case ErrClipboardEmpty:
		return "clipboard is empty"
	case ErrDetectionUnsupported:
		return "pattern detection is not supported"
	case ErrDetectionFailed:
		return "pattern detection failed"
	case ErrReadNotAllowed:
		return "clipboard read is not allowed"
	case ErrReadUserIntentRequired:
		return "user intent is required to read clipboard"
	case ErrReadPermissionDenied:
		return "clipboard read permission denied"
	case ErrReadFailed:
		return "clipboard read failed"
	case ErrItemNotFound:
		return "clipboard item not found"
	case ErrItemStale:
		return "clipboard item is stale"
	case ErrTypeUnsupported:
		return "content type is not supported"
	case ErrRepresentationUnavailable:
		return "representation is not available"
	case ErrContentTooLarge:
		return "content exceeds maximum size"
	case ErrWriteFailed:
		return "clipboard write failed"
	case ErrWritePermissionDenied:
		return "clipboard write permission denied"
	case ErrWriteTypeNotAllowed:
		return "write type is not allowed"
	case ErrWriteValueRequired:
		return "write value is required"
	case ErrWriteValueInvalid:
		return "write value is invalid"
	case ErrURLTooLong:
		return "URL exceeds maximum length"
	case ErrClearFailed:
		return "clipboard clear failed"
	case ErrClearNotAllowed:
		return "clipboard clear is not allowed"
	case ErrSensitiveBlocked:
		return "sensitive write blocked"
	case ErrSecretWriteNotConfirmed:
		return "secret write requires manual confirmation"
	case ErrNativeBridgeUnavailable:
		return "ios native bridge is not available"
	default:
		return code
	}
}

func NewClipboardError(request nativebridge.Request, code, message string) nativebridge.Response {
	return nativebridge.Response{
		ProtocolVersion: request.ProtocolVersion,
		RequestID:       request.RequestID,
		Status:          "error",
		Error: &nativebridge.Error{
			Code:    code,
			Message: message,
		},
	}
}

func MapSensitivityToLocalOnly(s Sensitivity) *bool {
	switch s {
	case SensitivitySensitive:
		v := true
		return &v
	case SensitivitySecret:
		v := true
		return &v
	default:
		return nil
	}
}

func MapSensitivityToExpiration(s Sensitivity) *int {
	switch s {
	case SensitivitySensitive:
		v := DefaultExpirationSensitive
		return &v
	case SensitivitySecret:
		v := DefaultExpirationSecret
		return &v
	default:
		return nil
	}
}
