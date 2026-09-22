package webresearch

import "fmt"

const (
	ErrInvalidInput       = "WEB_INVALID_INPUT"
	ErrNotConfigured      = "WEB_NOT_CONFIGURED"
	ErrReferenceNotFound  = "WEB_REFERENCE_NOT_FOUND"
	ErrReferenceScope     = "WEB_REFERENCE_SCOPE_MISMATCH"
	ErrFetchBlocked       = "WEB_FETCH_BLOCKED"
	ErrFetchFailed        = "WEB_FETCH_FAILED"
	ErrUnsupportedContent = "WEB_UNSUPPORTED_CONTENT"
	ErrBrowserUnavailable = "WEB_BROWSER_UNAVAILABLE"
	ErrBudgetExhausted    = "WEB_BUDGET_EXHAUSTED"
	ErrCancelled          = "WEB_CANCELLED"
)

type Error struct {
	Code      string
	Message   string
	Retryable bool
	Cause     error
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	message := e.Code
	if e.Message != "" {
		message += ": " + e.Message
	}
	if e.Cause != nil {
		message += ": " + e.Cause.Error()
	}
	return message
}

func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

func newError(code, message string, retryable bool, cause error) *Error {
	return &Error{Code: code, Message: message, Retryable: retryable, Cause: cause}
}

func wrapf(code string, retryable bool, cause error, format string, args ...any) *Error {
	return newError(code, fmt.Sprintf(format, args...), retryable, cause)
}
