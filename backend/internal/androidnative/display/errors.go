package display

const (
	ErrDisplayUnsupported            = "DISPLAY_UNSUPPORTED"
	ErrDisplayNotFound               = "DISPLAY_NOT_FOUND"
	ErrDisplayRemoved                = "DISPLAY_REMOVED"
	ErrDisplayTargetStale            = "DISPLAY_TARGET_STALE"
	ErrDisplayInvalidID              = "DISPLAY_INVALID_ID"
	ErrDisplayPrivate                = "DISPLAY_PRIVATE"
	ErrDisplaySecure                 = "DISPLAY_SECURE"
	ErrDisplayInactive               = "DISPLAY_INACTIVE"
	ErrDisplayOff                    = "DISPLAY_OFF"
	ErrDisplayUITreeUnsupported      = "DISPLAY_UI_TREE_UNSUPPORTED"
	ErrDisplayGestureUnsupported     = "DISPLAY_GESTURE_UNSUPPORTED"
	ErrDisplayScreenshotUnsupported  = "DISPLAY_SCREENSHOT_UNSUPPORTED"
	ErrDisplayScreenFrameUnsupported = "DISPLAY_SCREEN_FRAME_UNSUPPORTED"
	ErrDisplayActivityLaunchUnsupported = "DISPLAY_ACTIVITY_LAUNCH_UNSUPPORTED"
	ErrDisplayActivityLaunchDenied   = "DISPLAY_ACTIVITY_LAUNCH_DENIED"
	ErrDisplayTopologyUnsupported    = "DISPLAY_TOPOLOGY_UNSUPPORTED"
	ErrDisplayAmbiguous              = "DISPLAY_AMBIGUOUS"
	ErrDisplayNativeBridgeUnavailable = "DISPLAY_NATIVE_BRIDGE_UNAVAILABLE"
	ErrDisplayInvalidResponse        = "DISPLAY_INVALID_RESPONSE"
	ErrDisplayTimeout                = "DISPLAY_TIMEOUT"
	ErrDisplayCancelled              = "DISPLAY_CANCELLED"
)

type Error struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (e *Error) Error() string {
	return e.Code + ": " + e.Message
}

func NewError(code, message string) *Error {
	return &Error{Code: code, Message: message}
}
