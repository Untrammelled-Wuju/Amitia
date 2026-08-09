package runtime

import "fmt"

type LifecyclePlanErrorCode string

const (
	ErrLifecycleInvalidRuntime    LifecyclePlanErrorCode = "invalid_runtime"
	ErrTopologyGraphMismatch     LifecyclePlanErrorCode = "topology_graph_mismatch"
	ErrLifecycleServiceNotFound   LifecyclePlanErrorCode = "service_not_found"
	ErrLifecycleInvalidTarget     LifecyclePlanErrorCode = "invalid_target"
	ErrLifecycleDependencyError   LifecyclePlanErrorCode = "dependency_error"
	ErrLifecycleInvalidProgress   LifecyclePlanErrorCode = "invalid_progress"
)

type LifecyclePlanError struct {
	Code    LifecyclePlanErrorCode
	Message string
	Cause   error
}

func (e *LifecyclePlanError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("[%s] %s: %v", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

func (e *LifecyclePlanError) Unwrap() error {
	return e.Cause
}

func NewLifecyclePlanError(code LifecyclePlanErrorCode, message string) *LifecyclePlanError {
	return &LifecyclePlanError{
		Code:    code,
		Message: message,
	}
}

func NewLifecyclePlanErrorWithCause(code LifecyclePlanErrorCode, message string, cause error) *LifecyclePlanError {
	return &LifecyclePlanError{
		Code:    code,
		Message: message,
		Cause:   cause,
	}
}

func IsLifecyclePlanError(err error, code LifecyclePlanErrorCode) bool {
	le, ok := err.(*LifecyclePlanError)
	if !ok {
		return false
	}
	return le.Code == code
}
