//go:build linux && !android

package terminal

import "fmt"

type Error struct {
	code    string
	message string
}

func (e *Error) Error() string {
	return e.code + ": " + e.message
}

func (e *Error) Code() string {
	return e.code
}

const (
	ErrCodeNotAvailable     = "terminal.not_available"
	ErrCodeSessionNotFound  = "terminal.session_not_found"
	ErrCodeSessionLimit     = "terminal.session_limit"
	ErrCodeScopeDenied      = "terminal.scope_denied"
	ErrCodeNotRunning       = "terminal.not_running"
	ErrCodeInputTooLarge    = "terminal.input_too_large"
	ErrCodeOutputLimit      = "terminal.output_limit"
	ErrCodeInvalidSize      = "terminal.invalid_size"
	ErrCodeStartFailed      = "terminal.start_failed"
	ErrCodeIOFailed         = "terminal.io_failed"
	ErrCodeCancelled        = "terminal.cancelled"
	ErrCodeExited           = "terminal.exited"
)

func ErrNotAvailable(reason string) *Error {
	return &Error{code: ErrCodeNotAvailable, message: "terminal not available: " + reason}
}

func ErrSessionNotFound(id SessionID) *Error {
	return &Error{code: ErrCodeSessionNotFound, message: fmt.Sprintf("session not found: %s", id)}
}

func ErrSessionLimit(limit int) *Error {
	return &Error{code: ErrCodeSessionLimit, message: fmt.Sprintf("session limit reached: %d", limit)}
}

func ErrScopeDenied() *Error {
	return &Error{code: ErrCodeScopeDenied, message: "session scope denied"}
}

func ErrNotRunning() *Error {
	return &Error{code: ErrCodeNotRunning, message: "session not running"}
}

func ErrInputTooLarge(maxSize int) *Error {
	return &Error{code: ErrCodeInputTooLarge, message: fmt.Sprintf("input exceeds %d bytes", maxSize)}
}

func ErrOutputLimit(maxSize int) *Error {
	return &Error{code: ErrCodeOutputLimit, message: fmt.Sprintf("output exceeds %d bytes", maxSize)}
}

func ErrInvalidSize(reason string) *Error {
	return &Error{code: ErrCodeInvalidSize, message: "invalid size: " + reason}
}

func ErrStartFailed(reason string) *Error {
	return &Error{code: ErrCodeStartFailed, message: "start failed: " + reason}
}

func ErrIOFailed(reason string) *Error {
	return &Error{code: ErrCodeIOFailed, message: "io failed: " + reason}
}

func ErrCancelled() *Error {
	return &Error{code: ErrCodeCancelled, message: "session cancelled"}
}

func ErrExited(exitCode int) *Error {
	return &Error{code: ErrCodeExited, message: fmt.Sprintf("session exited with code %d", exitCode)}
}
