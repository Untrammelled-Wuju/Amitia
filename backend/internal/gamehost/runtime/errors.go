package runtime

import "fmt"

const (
	maxMetadataKeyLength   = 128
	maxMetadataValueLength = 1024
)

type ErrorCode string

const (
	ErrInvalidArgument     ErrorCode = "invalid_argument"
	ErrNotFound            ErrorCode = "not_found"
	ErrAlreadyExists       ErrorCode = "already_exists"
	ErrInvalidState        ErrorCode = "invalid_state"
	ErrUnsupported         ErrorCode = "unsupported"
	ErrDependencyNotFound  ErrorCode = "dependency_not_found"
	ErrDuplicateService    ErrorCode = "duplicate_service"
	ErrPluginMismatch      ErrorCode = "plugin_mismatch"
	ErrSelfDependency      ErrorCode = "self_dependency"
	ErrDuplicateDependency ErrorCode = "duplicate_dependency"
)

type TopologyError struct {
	Code      ErrorCode
	Message   string
	Retryable bool
	Cause     error
}

func (e *TopologyError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("[%s] %s: %v", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

func (e *TopologyError) Unwrap() error {
	return e.Cause
}

func NewTopologyError(code ErrorCode, message string) *TopologyError {
	return &TopologyError{
		Code:    code,
		Message: message,
	}
}

func NewTopologyErrorWithCause(code ErrorCode, message string, cause error) *TopologyError {
	return &TopologyError{
		Code:    code,
		Message: message,
		Cause:   cause,
	}
}

func IsTopologyError(err error, code ErrorCode) bool {
	te, ok := err.(*TopologyError)
	if !ok {
		return false
	}
	return te.Code == code
}
