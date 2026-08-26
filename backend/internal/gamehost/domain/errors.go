package domain

import "fmt"

type ErrorCode string

const (
	ErrInvalidArgument    ErrorCode = "invalid_argument"
	ErrNotFound           ErrorCode = "not_found"
	ErrAlreadyExists      ErrorCode = "already_exists"
	ErrInvalidState       ErrorCode = "invalid_state"
	ErrUnsupported        ErrorCode = "unsupported"
	ErrProtocolMismatch   ErrorCode = "protocol_mismatch"
	ErrRuntimeUnavailable ErrorCode = "runtime_unavailable"
	ErrPermissionDenied   ErrorCode = "permission_denied"
	ErrResourceExhausted  ErrorCode = "resource_exhausted"
	ErrInternal           ErrorCode = "internal"
	ErrTimeout            ErrorCode = "timeout"
	ErrCancelled          ErrorCode = "cancelled"
	ErrConflict           ErrorCode = "conflict"
)

type HostError struct {
	Code      ErrorCode
	Message   string
	Retryable bool
	Cause     error
}

func (e *HostError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("[%s] %s: %v", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

func (e *HostError) Unwrap() error {
	return e.Cause
}

func NewHostError(code ErrorCode, message string) *HostError {
	return &HostError{
		Code:    code,
		Message: message,
	}
}

func NewHostErrorWithCause(code ErrorCode, message string, cause error) *HostError {
	return &HostError{
		Code:    code,
		Message: message,
		Cause:   cause,
	}
}

func NewRetryableHostError(code ErrorCode, message string) *HostError {
	return &HostError{
		Code:      code,
		Message:   message,
		Retryable: true,
	}
}

func IsHostError(err error, code ErrorCode) bool {
	he, ok := err.(*HostError)
	if !ok {
		return false
	}
	return he.Code == code
}
