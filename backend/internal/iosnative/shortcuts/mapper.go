package shortcuts

import (
	"strings"

	"github.com/u-ai/backend/internal/nativebridge"
)

func MapErrorToNativeBridge(err error) (string, string) {
	if err == nil {
		return "", ""
	}
	msg := err.Error()
	for _, code := range []string{
		ErrShortcutsParameterRequired,
		ErrShortcutsParameterInvalid,
		ErrShortcutsEntityNotFound,
		ErrShortcutsEntityQueryFailed,
		ErrShortcutsEntityAccessDenied,
		ErrShortcutsRuntimeUnavailable,
		ErrShortcutsRuntimeStartFailed,
		ErrShortcutsBackgroundNotAllowed,
		ErrShortcutsForegroundRequired,
		ErrShortcutsPermissionDenied,
		ErrShortcutsConfirmationRequired,
		ErrShortcutsConfirmationCancelled,
		ErrShortcutsActionFailed,
		ErrShortcutsActionTimeout,
		ErrShortcutsActionCancelled,
		ErrShortcutsResultTooLarge,
		ErrShortcutsResultUnsupported,
		ErrShortcutsActionNotExposed,
		ErrShortcutsActionUnsupported,
		ErrShortcutsIntentUnsupported,
		ErrShortcutsIntentDonationFailed,
		ErrShortcutsContributionRejected,
		ErrShortcutsExposureInvalid,
		ErrShortcutsExecutionModeInvalid,
		ErrShortcutsRiskLevelInvalid,
		ErrShortcutsCanonicalTargetInvalid,
		ErrShortcutsIdempotencyInvalid,
		ErrShortcutsSnapshotUnavailable,
		ErrShortcutsAppShortcutInvalid,
		ErrShortcutsPhraseInvalid,
		ErrShortcutsLocaleInvalid,
		ErrShortcutsMetadataInvalid,
		ErrShortcutsResultRedactionFailed,
		ErrShortcutsSecretExposureRisk,
		ErrShortcutsUnavailable,
	} {
		if len(msg) >= len(code) && msg[:len(code)] == code {
			return code, msg
		}
	}
	return ErrOutcomeUnknown, msg
}

func MapCodeToMessage(code string) string {
	switch code {
	case ErrShortcutsUnavailable:
		return "shortcuts are not available on this device"
	case ErrShortcutsIntentUnsupported:
		return "this intent is not supported"
	case ErrShortcutsActionUnsupported:
		return "this action is not supported"
	case ErrShortcutsActionNotExposed:
		return "this action is not exposed in the shortcuts catalog"
	case ErrShortcutsEntityNotFound:
		return "the specified entity was not found"
	case ErrShortcutsEntityQueryFailed:
		return "entity query failed"
	case ErrShortcutsEntityAccessDenied:
		return "access to this entity is denied"
	case ErrShortcutsParameterInvalid:
		return "one or more parameters are invalid"
	case ErrShortcutsParameterRequired:
		return "a required parameter is missing"
	case ErrShortcutsRuntimeUnavailable:
		return "amitia runtime is not available"
	case ErrShortcutsRuntimeStartFailed:
		return "failed to start amitia runtime"
	case ErrShortcutsBackgroundNotAllowed:
		return "this action cannot run in the background"
	case ErrShortcutsForegroundRequired:
		return "this action requires the app to be in the foreground"
	case ErrShortcutsPermissionDenied:
		return "permission denied for this action"
	case ErrShortcutsConfirmationRequired:
		return "user confirmation is required before executing this action"
	case ErrShortcutsConfirmationCancelled:
		return "the action was cancelled by the user"
	case ErrShortcutsActionFailed:
		return "the action failed to complete"
	case ErrShortcutsActionTimeout:
		return "the action timed out"
	case ErrShortcutsActionCancelled:
		return "the action was cancelled"
	case ErrShortcutsResultTooLarge:
		return "the result is too large to return via shortcuts"
	case ErrShortcutsResultUnsupported:
		return "the result type is not supported by shortcuts"
	case ErrShortcutsNativeBridgeUnavailable:
		return "ios native bridge is not available"
	case ErrShortcutsInvalidResponse:
		return "received an invalid response from native layer"
	case ErrShortcutsIntentDonationFailed:
		return "failed to donate intent to the system"
	case ErrShortcutsContributionRejected:
		return "the contribution was rejected"
	case ErrShortcutsExposureInvalid:
		return "invalid exposure level specified"
	case ErrShortcutsExecutionModeInvalid:
		return "invalid execution mode specified"
	case ErrShortcutsRiskLevelInvalid:
		return "invalid risk level specified"
	case ErrShortcutsCanonicalTargetInvalid:
		return "invalid canonical target specified"
	case ErrShortcutsIdempotencyInvalid:
		return "invalid idempotency key"
	case ErrShortcutsSnapshotUnavailable:
		return "entity snapshot is unavailable"
	case ErrShortcutsAppShortcutInvalid:
		return "invalid app shortcut configuration"
	case ErrShortcutsPhraseInvalid:
		return "invalid localized phrase"
	case ErrShortcutsLocaleInvalid:
		return "invalid locale specified"
	case ErrShortcutsMetadataInvalid:
		return "invalid intent metadata"
	case ErrShortcutsResultRedactionFailed:
		return "failed to redact sensitive result content"
	case ErrShortcutsSecretExposureRisk:
		return "this action risks exposing sensitive data"
	default:
		return code
	}
}

func NewShortcutsError(request nativebridge.Request, code, message string) nativebridge.Response {
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

func DisplaySafeResult(result string) string {
	if len(result) > MaxResultBytes {
		return result[:MaxResultBytes]
	}
	return result
}

func SanitizeForDisplay(text string) string {
	text = strings.ReplaceAll(text, "\x00", "")
	return text
}
