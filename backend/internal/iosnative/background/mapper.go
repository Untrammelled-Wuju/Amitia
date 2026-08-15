package background

import (
	"github.com/u-ai/backend/internal/nativebridge"
)

func MapErrorToNativeBridge(err error) (string, string) {
	if err == nil {
		return "", ""
	}
	msg := err.Error()
	for _, code := range []string{
		ErrBackgroundIdentifierInvalid,
		ErrBackgroundIdentifierNotPermitted,
		ErrBackgroundRegistrationFailed,
		ErrBackgroundSubmissionFailed,
		ErrBackgroundSubmissionNotImmediate,
		ErrBackgroundTooManyPending,
		ErrBackgroundNetworkRequirementUnavailable,
		ErrBackgroundPowerRequirementUnavailable,
		ErrBackgroundResourceUnsupported,
		ErrBackgroundGPUEntitlementRequired,
		ErrBackgroundTaskNotFound,
		ErrBackgroundTaskBindingInvalid,
		ErrBackgroundRuntimeUnavailable,
		ErrBackgroundRuntimeStartFailed,
		ErrBackgroundExpired,
		ErrBackgroundCancelled,
		ErrBackgroundInterrupted,
		ErrBackgroundProgressInvalid,
		ErrBackgroundCompletionFailed,
		ErrBackgroundNotUserInitiated,
		ErrBackgroundUIRequired,
		ErrBackgroundUnavailable,
		ErrBackgroundRefreshUnavailable,
		ErrBackgroundProcessingUnavailable,
		ErrBackgroundContinuedUnavailable,
		ErrFileUnavailable,
		ErrFilePickerUIRequired,
		ErrFileSelectionCancelled,
		ErrFileGrantInvalid,
		ErrFileGrantNotFound,
		ErrFileGrantStale,
		ErrFilePermissionRevoked,
		ErrFileSecurityScopeFailed,
		ErrFileCoordinationFailed,
		ErrFileCoordinationCancelled,
		ErrFileNotFound,
		ErrFileProviderUnavailable,
		ErrFileProviderOffline,
		ErrFileReadFailed,
		ErrFileWriteFailed,
		ErrFileMoveFailed,
		ErrFileCopyFailed,
		ErrFileDeleteFailed,
		ErrFileImportFailed,
		ErrFileExportFailed,
		ErrFileMaterializeFailed,
		ErrFilePathInvalid,
		ErrFileSizeLimitExceeded,
		ErrFileOutcomeUnknown,
	} {
		if len(msg) >= len(code) && msg[:len(code)] == code {
			return code, msg
		}
	}
	return ErrOutcomeUnknown, msg
}

func MapCodeToMessage(code string) string {
	switch code {
	case ErrBackgroundUnavailable:
		return "background tasks are not available on this device"
	case ErrBackgroundRefreshUnavailable:
		return "background app refresh is not available"
	case ErrBackgroundProcessingUnavailable:
		return "background processing is not available"
	case ErrBackgroundContinuedUnavailable:
		return "continued processing is not available on this device"
	case ErrBackgroundIdentifierInvalid:
		return "invalid background task identifier"
	case ErrBackgroundIdentifierNotPermitted:
		return "background task identifier is not permitted"
	case ErrBackgroundRegistrationFailed:
		return "failed to register background task handler"
	case ErrBackgroundSubmissionFailed:
		return "failed to submit background task request"
	case ErrBackgroundTaskNotFound:
		return "background task not found"
	case ErrBackgroundTaskBindingInvalid:
		return "invalid background task binding"
	case ErrBackgroundRuntimeUnavailable:
		return "amitia runtime is not available for background execution"
	case ErrBackgroundExpired:
		return "background task expired"
	case ErrBackgroundCancelled:
		return "background task was cancelled"
	case ErrBackgroundInterrupted:
		return "background task was interrupted"
	case ErrBackgroundNotUserInitiated:
		return "continued processing requires user initiation"
	case ErrBackgroundUIRequired:
		return "this operation requires user interaction"
	case ErrFileUnavailable:
		return "file access is not available"
	case ErrFilePickerUIRequired:
		return "document picker must be shown in the foreground"
	case ErrFileSelectionCancelled:
		return "file selection was cancelled"
	case ErrFileGrantInvalid:
		return "file grant is invalid"
	case ErrFileGrantNotFound:
		return "file grant not found"
	case ErrFileGrantStale:
		return "file grant is stale and needs reauthorization"
	case ErrFilePermissionRevoked:
		return "file permission has been revoked"
	case ErrFileSecurityScopeFailed:
		return "failed to access security-scoped resource"
	case ErrFileCoordinationFailed:
		return "file coordination failed"
	case ErrFileNotFound:
		return "file not found"
	case ErrFileProviderUnavailable:
		return "file provider is unavailable"
	case ErrFileReadFailed:
		return "failed to read file"
	case ErrFileWriteFailed:
		return "failed to write file"
	case ErrFilePathInvalid:
		return "invalid file path"
	case ErrFileSizeLimitExceeded:
		return "file size exceeds limit"
	default:
		return code
	}
}

func NewBackgroundError(request nativebridge.Request, code, message string) nativebridge.Response {
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

func getStringSlice(m map[string]any, key string) []string {
	var result []string
	if arr, ok := m[key].([]any); ok {
		for _, item := range arr {
			if s, ok := item.(string); ok && s != "" {
				result = append(result, s)
			}
		}
	}
	return result
}
