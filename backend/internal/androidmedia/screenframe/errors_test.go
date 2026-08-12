package screenframe

import "testing"

func TestError_Error(t *testing.T) {
	e := &Error{Code: "TEST_CODE", Message: "test message"}
	expected := ": TEST_CODE: test message"
	if got := e.Error(); got != expected {
		t.Errorf("expected %q, got %q", expected, got)
	}
}

func TestMapToKernelCode(t *testing.T) {
	tests := []struct {
		domain string
		kernel string
	}{
		{ErrUnsupported, "not_available"},
		{ErrPermissionRequired, "user_action_required"},
		{ErrSessionAlreadyActive, "resource_limit_exceeded"},
		{ErrSessionNotFound, "not_available"},
		{ErrSessionStale, "conflict"},
		{ErrSessionNotRunning, "not_available"},
		{ErrInvalidDisplay, "invalid_input"},
		{ErrInvalidFPS, "invalid_input"},
		{ErrInvalidSize, "invalid_input"},
		{ErrProjectionStartFailed, "execution_failed"},
		{ErrProjectionRevoked, "user_action_required"},
		{ErrImageReaderFailed, "execution_failed"},
		{ErrFrameNotAvailable, "not_available"},
		{ErrResourceLimit, "resource_limit_exceeded"},
		{ErrEncodeFailed, "execution_failed"},
		{ErrArtifactFailed, "execution_failed"},
		{ErrCancelled, "cancelled"},
		{ErrTimeout, "timed_out"},
		{ErrBlockedNativeHost, "not_available"},
		{ErrFrozenContract, "not_available"},
		{ErrBlockedByB34, "not_available"},
		{"UNKNOWN_CODE", "execution_failed"},
	}

	for _, tt := range tests {
		if got := MapToKernelCode(tt.domain); got != tt.kernel {
			t.Errorf("domain %s: expected %s, got %s", tt.domain, tt.kernel, got)
		}
	}
}

func TestNewFrameError(t *testing.T) {
	err := NewFrameError("CODE", "message")
	if err == nil {
		t.Fatal("expected non-nil error")
	}
	serr, ok := err.(*Error)
	if !ok {
		t.Fatalf("expected *Error, got %T", err)
	}
	if serr.Code != "CODE" {
		t.Errorf("expected code CODE, got %v", serr.Code)
	}
	if serr.Message != "message" {
		t.Errorf("expected message, got %v", serr.Message)
	}
}

func TestFormatResourceLimit(t *testing.T) {
	err := FormatResourceLimit("reason text")
	if err == nil {
		t.Fatal("expected non-nil error")
	}
	serr, ok := err.(*Error)
	if !ok {
		t.Fatalf("expected *Error, got %T", err)
	}
	if serr.Code != ErrResourceLimit {
		t.Errorf("expected ErrResourceLimit, got %v", serr.Code)
	}
}

func TestIsSessionTerminal(t *testing.T) {
	tests := []struct {
		state    SessionState
		terminal bool
	}{
		{SessionStateStarting, false},
		{SessionStateAwaitingPermission, false},
		{SessionStateRunning, false},
		{SessionStateStopping, false},
		{SessionStateStopped, true},
		{SessionStateProjectionRevoked, true},
		{SessionStateFailed, true},
	}

	for _, tt := range tests {
		if got := IsSessionTerminal(tt.state); got != tt.terminal {
			t.Errorf("state %s: expected terminal=%v, got %v", tt.state, tt.terminal, got)
		}
	}
}

func TestIsProjectionRevokedError(t *testing.T) {
	if !IsProjectionRevokedError(ErrProjectionRevoked) {
		t.Error("expected ErrProjectionRevoked to be projection revoked error")
	}
	if IsProjectionRevokedError(ErrPermissionRequired) {
		t.Error("ErrPermissionRequired should not be projection revoked error")
	}
}

func TestIsRetryable(t *testing.T) {
	codes := []string{
		ErrUnsupported, ErrPermissionRequired, ErrSessionAlreadyActive,
		ErrSessionNotFound, ErrSessionStale, ErrSessionNotRunning,
		ErrInvalidFPS, ErrProjectionStartFailed, ErrProjectionRevoked,
		ErrFrameNotAvailable, ErrResourceLimit, ErrCancelled, ErrTimeout,
	}
	for _, code := range codes {
		if IsRetryable(code) {
			t.Errorf("code %s should not be retryable", code)
		}
	}
}
