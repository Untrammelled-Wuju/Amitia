package domain

import (
	"errors"
	"fmt"
	"testing"
)

func TestHostErrorError(t *testing.T) {
	err := NewHostError(ErrInvalidArgument, "bad input")
	expected := "[invalid_argument] bad input"
	if err.Error() != expected {
		t.Errorf("expected %q, got %q", expected, err.Error())
	}
}

func TestHostErrorWithCause(t *testing.T) {
	cause := fmt.Errorf("underlying error")
	err := NewHostErrorWithCause(ErrInternal, "something failed", cause)

	expected := "[internal] something failed: underlying error"
	if err.Error() != expected {
		t.Errorf("expected %q, got %q", expected, err.Error())
	}
}

func TestHostErrorUnwrap(t *testing.T) {
	cause := fmt.Errorf("root cause")
	err := NewHostErrorWithCause(ErrNotFound, "not found", cause)

	if !errors.Is(err, cause) {
		// errors.Is walks the chain, but we need to unwrap manually
		// Actually errors.Is will work because of Unwrap
	}

	unwrapped := errors.Unwrap(err)
	if unwrapped != cause {
		t.Errorf("expected unwrapped error to be cause, got %v", unwrapped)
	}
}

func TestHostErrorUnwrapMultiple(t *testing.T) {
	root := fmt.Errorf("root")
	middle := NewHostErrorWithCause(ErrInternal, "middle", root)
	outer := NewHostErrorWithCause(ErrRuntimeUnavailable, "outer", middle)

	unwrapped := errors.Unwrap(outer)
	if unwrapped != middle {
		t.Error("expected middle error")
	}

	unwrapped = errors.Unwrap(unwrapped)
	if unwrapped != root {
		t.Error("expected root error")
	}
}

func TestIsHostError(t *testing.T) {
	err := NewHostError(ErrAlreadyExists, "already exists")
	if !IsHostError(err, ErrAlreadyExists) {
		t.Error("expected IsHostError to return true for matching code")
	}
	if IsHostError(err, ErrNotFound) {
		t.Error("expected IsHostError to return false for non-matching code")
	}

	regularErr := fmt.Errorf("regular error")
	if IsHostError(regularErr, ErrInternal) {
		t.Error("expected IsHostError to return false for non-HostError")
	}
}

func TestHostErrorAs(t *testing.T) {
	var he *HostError
	err := NewHostError(ErrInvalidState, "invalid state")

	if !errors.As(err, &he) {
		t.Error("expected errors.As to succeed")
	}
	if he.Code != ErrInvalidState {
		t.Errorf("expected code %s, got %s", ErrInvalidState, he.Code)
	}
}

func TestRetryableHostError(t *testing.T) {
	err := NewRetryableHostError(ErrResourceExhausted, "rate limited")
	if !err.Retryable {
		t.Error("expected error to be retryable")
	}
	if err.Code != ErrResourceExhausted {
		t.Errorf("unexpected error code: %s", err.Code)
	}
}

func TestErrorCodeValues(t *testing.T) {
	cases := map[ErrorCode]string{
		ErrInvalidArgument:    "invalid_argument",
		ErrNotFound:           "not_found",
		ErrAlreadyExists:      "already_exists",
		ErrInvalidState:       "invalid_state",
		ErrUnsupported:        "unsupported",
		ErrProtocolMismatch:   "protocol_mismatch",
		ErrRuntimeUnavailable: "runtime_unavailable",
		ErrPermissionDenied:   "permission_denied",
		ErrResourceExhausted:  "resource_exhausted",
		ErrInternal:           "internal",
	}

	for code, expected := range cases {
		if string(code) != expected {
			t.Errorf("expected %s, got %s", expected, code)
		}
	}
}
