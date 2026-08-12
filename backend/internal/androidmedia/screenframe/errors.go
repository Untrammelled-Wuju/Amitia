package screenframe

type Error struct {
	Code    string
	Message string
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	return ": " + e.Code + ": " + e.Message
}

const (
	ErrUnsupported              = "SCREEN_FRAME_UNSUPPORTED"
	ErrPermissionRequired       = "SCREEN_FRAME_PERMISSION_REQUIRED"
	ErrSessionAlreadyActive     = "SCREEN_FRAME_SESSION_ALREADY_ACTIVE"
	ErrSessionNotFound          = "SCREEN_FRAME_SESSION_NOT_FOUND"
	ErrSessionStale             = "SCREEN_FRAME_SESSION_STALE"
	ErrSessionNotRunning        = "SCREEN_FRAME_SESSION_NOT_RUNNING"
	ErrInvalidDisplay           = "SCREEN_FRAME_DISPLAY_UNAVAILABLE"
	ErrInvalidFPS               = "SCREEN_FRAME_INVALID_FPS"
	ErrInvalidSize              = "SCREEN_FRAME_INVALID_SIZE"
	ErrProjectionStartFailed    = "SCREEN_FRAME_PROJECTION_START_FAILED"
	ErrProjectionRevoked        = "SCREEN_FRAME_PROJECTION_REVOKED"
	ErrImageReaderFailed        = "SCREEN_FRAME_IMAGE_READER_FAILED"
	ErrFrameNotAvailable        = "SCREEN_FRAME_FRAME_NOT_AVAILABLE"
	ErrResourceLimit            = "SCREEN_FRAME_RESOURCE_LIMIT"
	ErrEncodeFailed             = "SCREEN_FRAME_ENCODE_FAILED"
	ErrArtifactFailed           = "SCREEN_FRAME_ARTIFACT_FAILED"
	ErrCancelled                = "SCREEN_FRAME_CANCELLED"
	ErrTimeout                  = "SCREEN_FRAME_TIMEOUT"
	ErrBlockedNativeHost        = "BLOCKED_ANDROID_NATIVE_HOST_SOURCE"
	ErrFrozenContract           = "BLOCKED_BY_FROZEN_A_CONTRACT"
	ErrBlockedByB34             = "BLOCKED_BY_B34_SCREENSHOT_PIPELINE"
)

const (
	PermissionContinuousCapture = "android.media.screen_capture.continuous"
)

var kernelCodeMapping = map[string]string{
	ErrUnsupported:           "not_available",
	ErrPermissionRequired:    "user_action_required",
	ErrSessionAlreadyActive:   "resource_limit_exceeded",
	ErrSessionNotFound:       "not_available",
	ErrSessionStale:          "conflict",
	ErrSessionNotRunning:     "not_available",
	ErrInvalidDisplay:        "invalid_input",
	ErrInvalidFPS:            "invalid_input",
	ErrInvalidSize:           "invalid_input",
	ErrProjectionStartFailed: "execution_failed",
	ErrProjectionRevoked:     "user_action_required",
	ErrImageReaderFailed:     "execution_failed",
	ErrFrameNotAvailable:     "not_available",
	ErrResourceLimit:         "resource_limit_exceeded",
	ErrEncodeFailed:          "execution_failed",
	ErrArtifactFailed:        "execution_failed",
	ErrCancelled:             "cancelled",
	ErrTimeout:               "timed_out",
	ErrBlockedNativeHost:     "not_available",
	ErrFrozenContract:        "not_available",
	ErrBlockedByB34:          "not_available",
}

func MapToKernelCode(domainCode string) string {
	if code, ok := kernelCodeMapping[domainCode]; ok {
		return code
	}
	return "execution_failed"
}

func NewFrameError(code, message string) error {
	return &Error{Code: code, Message: message}
}

func FormatResourceLimit(reason string) error {
	return &Error{
		Code:    ErrResourceLimit,
		Message: "screen frame resource limit: " + reason,
	}
}

func IsSessionTerminal(state SessionState) bool {
	switch state {
	case SessionStateStopped, SessionStateProjectionRevoked, SessionStateFailed:
		return true
	}
	return false
}

func IsProjectionRevokedError(code string) bool {
	return code == ErrProjectionRevoked
}

func IsRetryable(code string) bool {
	return false
}
