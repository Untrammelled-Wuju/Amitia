package alarms

import "github.com/u-ai/backend/internal/nativebridge"

func AuthorizationStatusFromNative(native string) AuthorizationStatus {
	switch native {
	case "authorized":
		return AuthAuthorized
	case "denied":
		return AuthDenied
	default:
		return AuthNotDetermined
	}
}

func MapErrorToNativeBridge(err error) (string, string) {
	if err == nil {
		return "", ""
	}
	return ErrOutcomeUnknown, err.Error()
}

func MapCodeToMessage(code string) string {
	switch code {
	case ErrAlarmsUnsupported:
		return "alarms are not supported on this device"
	case ErrAlarmsUnsupportedOSVersion:
		return "alarms require iOS 26.0 or later"
	case ErrAlarmsAuthDenied:
		return "alarm access denied"
	case ErrAlarmsNotFound:
		return "alarm not found"
	case ErrAlarmsScheduleInPast:
		return "alarm schedule is in the past"
	case ErrAlarmsScheduleInvalid:
		return "invalid alarm schedule"
	case ErrAlarmsCountdownInvalid:
		return "invalid countdown duration"
	case ErrAlarmsPresentationInvalid:
		return "invalid alarm presentation"
	case ErrAlarmsSoundInvalid:
		return "invalid alarm sound"
	case ErrAlarmsSoundNotRegistered:
		return "alarm sound is not registered"
	case ErrAlarmsActionInvalid:
		return "invalid alarm action"
	case ErrAlarmsMetadataInvalid:
		return "invalid alarm metadata"
	case ErrAlarmsMaximumLimitReached:
		return "maximum alarm limit reached"
	case ErrAlarmsPlatformFailed:
		return "alarm platform operation failed"
	case ErrAlarmsInvalidState:
		return "invalid alarm state for this operation"
	case ErrAlarmsStopFailed:
		return "failed to stop alarm"
	case ErrAlarmsCancelFailed:
		return "failed to cancel alarm"
	case ErrAlarmsPauseFailed:
		return "failed to pause alarm"
	case ErrAlarmsResumeFailed:
		return "failed to resume alarm"
	case ErrAlarmsCountdownFailed:
		return "failed to start countdown"
	case ErrNativeBridgeUnavailable:
		return "ios native bridge is not available"
	default:
		return code
	}
}

func NewAlarmError(request nativebridge.Request, code, message string) nativebridge.Response {
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
