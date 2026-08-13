// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package realtime

import "fmt"

type VoiceErrorCode string

const (
	VoiceErrUnavailable         VoiceErrorCode = "VOICE_UNAVAILABLE"
	VoiceErrSessionNotFound     VoiceErrorCode = "VOICE_SESSION_NOT_FOUND"
	VoiceErrSessionBusy         VoiceErrorCode = "VOICE_SESSION_BUSY"
	VoiceErrSessionInvalidState VoiceErrorCode = "VOICE_SESSION_INVALID_STATE"
	VoiceErrSessionStartFailed  VoiceErrorCode = "VOICE_SESSION_START_FAILED"
	VoiceErrSessionStopFailed   VoiceErrorCode = "VOICE_SESSION_STOP_FAILED"
	VoiceErrSessionSuspended    VoiceErrorCode = "VOICE_SESSION_SUSPENDED"

	VoiceErrMicPermissionRequired VoiceErrorCode = "VOICE_MICROPHONE_PERMISSION_REQUIRED"
	VoiceErrMicDenied             VoiceErrorCode = "VOICE_MICROPHONE_DENIED"
	VoiceErrMicUnavailable        VoiceErrorCode = "VOICE_MICROPHONE_UNAVAILABLE"

	VoiceErrAudioSessionBusy      VoiceErrorCode = "VOICE_AUDIO_SESSION_BUSY"
	VoiceErrAudioSessionFailed    VoiceErrorCode = "VOICE_AUDIO_SESSION_FAILED"
	VoiceErrAudioRouteUnavailable VoiceErrorCode = "VOICE_AUDIO_ROUTE_UNAVAILABLE"
	VoiceErrAudioInterrupted      VoiceErrorCode = "VOICE_AUDIO_INTERRUPTED"

	VoiceErrCaptureStartFailed VoiceErrorCode = "VOICE_CAPTURE_START_FAILED"
	VoiceErrCaptureFailed      VoiceErrorCode = "VOICE_CAPTURE_FAILED"
	VoiceErrCaptureOverrun     VoiceErrorCode = "VOICE_CAPTURE_OVERRUN"

	VoiceErrPlaybackStartFailed VoiceErrorCode = "VOICE_PLAYBACK_START_FAILED"
	VoiceErrPlaybackFailed      VoiceErrorCode = "VOICE_PLAYBACK_FAILED"
	VoiceErrPlaybackUnderrun    VoiceErrorCode = "VOICE_PLAYBACK_UNDERRUN"
	VoiceErrPlaybackCancelled   VoiceErrorCode = "VOICE_PLAYBACK_CANCELLED"

	VoiceErrVADUnavailable VoiceErrorCode = "VOICE_VAD_UNAVAILABLE"
	VoiceErrVADFailed      VoiceErrorCode = "VOICE_VAD_FAILED"

	VoiceErrEndpointTimeout  VoiceErrorCode = "VOICE_ENDPOINT_TIMEOUT"
	VoiceErrUtteranceTooLong VoiceErrorCode = "VOICE_UTTERANCE_TOO_LONG"

	VoiceErrASRUnavailable  VoiceErrorCode = "VOICE_ASR_UNAVAILABLE"
	VoiceErrASRStreamFailed VoiceErrorCode = "VOICE_ASR_STREAM_FAILED"
	VoiceErrASRFinalFailed  VoiceErrorCode = "VOICE_ASR_FINAL_FAILED"
	VoiceErrASRTimeout      VoiceErrorCode = "VOICE_ASR_TIMEOUT"

	VoiceErrTTSUnavailable  VoiceErrorCode = "VOICE_TTS_UNAVAILABLE"
	VoiceErrTTSStreamFailed VoiceErrorCode = "VOICE_TTS_STREAM_FAILED"
	VoiceErrTTSCancelled    VoiceErrorCode = "VOICE_TTS_CANCELLED"

	VoiceErrWakeUnavailable     VoiceErrorCode = "VOICE_WAKE_UNAVAILABLE"
	VoiceErrWakeModelInvalid    VoiceErrorCode = "VOICE_WAKE_MODEL_INVALID"
	VoiceErrWakeModelLoadFailed VoiceErrorCode = "VOICE_WAKE_MODEL_LOAD_FAILED"
	VoiceErrWakeNotArmed        VoiceErrorCode = "VOICE_WAKE_NOT_ARMED"
	VoiceErrWakeSystemHotwordNA VoiceErrorCode = "VOICE_WAKE_SYSTEM_HOTWORD_UNAVAILABLE"
	VoiceErrWakeBackgroundUnsup VoiceErrorCode = "VOICE_WAKE_BACKGROUND_UNSUPPORTED"

	VoiceErrRealtimeProviderNA   VoiceErrorCode = "VOICE_REALTIME_PROVIDER_UNAVAILABLE"
	VoiceErrRealtimeProviderFail VoiceErrorCode = "VOICE_REALTIME_PROVIDER_FAILED"
	VoiceErrRealtimeReconnectReq VoiceErrorCode = "VOICE_REALTIME_RECONNECT_REQUIRED"

	VoiceErrNetworkUnavailable VoiceErrorCode = "VOICE_NETWORK_UNAVAILABLE"

	VoiceErrBackgroundStartDenied VoiceErrorCode = "VOICE_BACKGROUND_START_NOT_ALLOWED"
	VoiceErrPlatformRestriction   VoiceErrorCode = "VOICE_PLATFORM_RESTRICTION"

	VoiceErrCancelled VoiceErrorCode = "VOICE_CANCELLED"
	VoiceErrTimeout   VoiceErrorCode = "VOICE_TIMEOUT"
)

type VoiceError struct {
	Code    VoiceErrorCode
	Message string
	Cause   error
}

func (e *VoiceError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("voice error [%s]: %s: %v", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("voice error [%s]: %s", e.Code, e.Message)
}

func NewVoiceError(code VoiceErrorCode, message string) *VoiceError {
	return &VoiceError{Code: code, Message: message}
}

func WrapVoiceError(code VoiceErrorCode, message string, cause error) *VoiceError {
	return &VoiceError{Code: code, Message: message, Cause: cause}
}

