package share

import "github.com/u-ai/backend/internal/nativebridge"

func MapErrorToNativeBridge(err error) (string, string) {
	if err == nil {
		return "", ""
	}
	msg := err.Error()
	for _, code := range []string{
		ErrShareTextTooLong,
		ErrShareTooManyResources,
		ErrShareSubjectTooLong,
		ErrShareShareTitleTooLong,
		ErrSharePreviewTitleTooLong,
		ErrSharePreviewSubtitleTooLong,
		ErrShareURLInvalid,
		ErrShareURLSchemeNotAllowed,
		ErrShareResourceTooLarge,
		ErrShareTotalTooLarge,
		ErrShareStagingInvalid,
		ErrShareStagingNotCommitted,
		ErrShareStagingPathEscape,
		ErrShareStagingTooManyResources,
		ErrShareStagingTooLarge,
		ErrShareStagingTextTooLong,
	} {
		if len(msg) >= len(code) && msg[:len(code)] == code {
			return code, msg
		}
	}
	return ErrOutcomeUnknown, msg
}

func MapCodeToMessage(code string) string {
	switch code {
	case ErrShareUnsupported:
		return "share is not supported on this device"
	case ErrShareUnavailable:
		return "share is currently unavailable"
	case ErrShareExtensionMissing:
		return "share extension is not installed"
	case ErrShareURLSchemeNotAllowed:
		return "URL scheme is not allowed for sharing"
	case ErrShareURLInvalid:
		return "invalid URL"
	case ErrShareTooManyResources:
		return "too many resources to share"
	case ErrShareResourceTooLarge:
		return "single resource exceeds maximum size"
	case ErrShareTotalTooLarge:
		return "total resources size exceeds maximum"
	case ErrShareTextTooLong:
		return "share text exceeds maximum length"
	case ErrShareSubjectTooLong:
		return "share subject exceeds maximum length"
	case ErrShareShareTitleTooLong:
		return "share title exceeds maximum length"
	case ErrSharePreviewTitleTooLong:
		return "preview title exceeds maximum length"
	case ErrShareSendFailed:
		return "share send operation failed"
	case ErrShareUserCancelled:
		return "share was cancelled by user"
	case ErrShareUIContextRequired:
		return "UI context required for presenting share sheet"
	case ErrShareUserIntentRequired:
		return "user intent required for share"
	case ErrShareReceivedNotFound:
		return "pending share not found"
	case ErrShareReceivedStale:
		return "pending share is stale"
	case ErrShareStagingInvalid:
		return "staging data is invalid"
	case ErrShareStagingNotCommitted:
		return "staging is not committed"
	case ErrShareStagingPathEscape:
		return "staging path contains escape sequence"
	case ErrShareIntakeFailed:
		return "share intake failed"
	case ErrNativeBridgeUnavailable:
		return "ios native bridge is not available"
	default:
		return code
	}
}

func NewShareError(request nativebridge.Request, code, message string) nativebridge.Response {
	return nativebridge.Response{
		ProtocolVersion: request.ProtocolVersion,
		RequestId:       request.RequestId,
		Status:          "error",
		Error: &nativebridge.Error{
			Code:    code,
			Message: message,
		},
	}
}
