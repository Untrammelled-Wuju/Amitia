package screenshot

import (
	"fmt"
)

type Error struct {
	Code    string
	Message string
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	return e.Code + ": " + e.Message
}

const (
	ErrUnsupported              = "SCREENSHOT_UNSUPPORTED"
	ErrAccessibilityDisabled    = "SCREENSHOT_ACCESSIBILITY_DISABLED"
	ErrAccessibilityNotConnected = "SCREENSHOT_ACCESSIBILITY_NOT_CONNECTED"
	ErrCapabilityNotDeclared    = "SCREENSHOT_CAPABILITY_NOT_DECLARED"
	ErrInvalidDisplay          = "SCREENSHOT_INVALID_DISPLAY"
	ErrIntervalTooShort        = "SCREENSHOT_INTERVAL_TOO_SHORT"
	ErrSecureContent           = "SCREENSHOT_SECURE_CONTENT"
	ErrCaptureFailed           = "SCREENSHOT_CAPTURE_FAILED"
	ErrEncodeFailed            = "SCREENSHOT_ENCODE_FAILED"
	ErrTooLarge                = "SCREENSHOT_TOO_LARGE"
	ErrArtifactWriteFailed     = "SCREENSHOT_ARTIFACT_WRITE_FAILED"
	ErrResourceInvalid         = "SCREENSHOT_RESOURCE_INVALID"
	ErrCancelled               = "SCREENSHOT_CANCELLED"
	ErrResourceExhausted       = "SCREENSHOT_RESOURCE_EXHAUSTED"
	ErrBlockedNativeHost       = "BLOCKED_ANDROID_NATIVE_HOST_SOURCE"
	ErrFrozenContract          = "BLOCKED_BY_FROZEN_A_CONTRACT"
)

var errorMapping = map[string]string{
	ErrUnsupported:               "not_available",
	ErrAccessibilityDisabled:     "permission_denied",
	ErrAccessibilityNotConnected: "not_available",
	ErrCapabilityNotDeclared:     "not_available",
	ErrInvalidDisplay:            "invalid_input",
	ErrIntervalTooShort:          "rate_limited",
	ErrSecureContent:             "restricted_content",
	ErrCaptureFailed:             "execution_failed",
	ErrEncodeFailed:              "execution_failed",
	ErrTooLarge:                  "resource_limit_exceeded",
	ErrArtifactWriteFailed:       "execution_failed",
	ErrResourceInvalid:           "invalid_result",
	ErrCancelled:                 "cancelled",
	ErrResourceExhausted:        "resource_limit_exceeded",
	ErrBlockedNativeHost:         "not_available",
	ErrFrozenContract:            "not_available",
}

func MapToKernelCode(domainCode string) string {
	if code, ok := errorMapping[domainCode]; ok {
		return code
	}
	return "execution_failed"
}

func NativeHostUnavailableError() error {
	return &Error{
		Code:    ErrBlockedNativeHost,
		Message: "android native host source not available; screenshot capture cannot execute without native accessibility backend",
	}
}

func FormatResourceInvalid(reason string) error {
	return &Error{
		Code:    ErrResourceInvalid,
		Message: fmt.Sprintf("screenshot artifact resource is invalid: %s", reason),
	}
}
