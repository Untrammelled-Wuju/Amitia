package runtime

import "fmt"

const (
	ErrHealthAdapterUnavailable ErrorCode = "health_adapter_unavailable"
	ErrRestartExhausted         ErrorCode = "restart_exhausted"
	ErrServiceQuarantined       ErrorCode = "service_quarantined"
	ErrCrashUnrecoverable       ErrorCode = "crash_unrecoverable"
	ErrCleanupFailed            ErrorCode = "cleanup_failed"
)

var healthErrorCodes = map[ErrorCode]bool{
	ErrHealthAdapterUnavailable: true,
	ErrRestartExhausted:         true,
	ErrServiceQuarantined:       true,
	ErrCrashUnrecoverable:       true,
	ErrCleanupFailed:            true,
}

func IsHealthError(err error) bool {
	he, ok := err.(*ExecutionError)
	if !ok {
		return false
	}
	return healthErrorCodes[he.Code]
}

type ServiceHealthError struct {
	Code      ErrorCode
	RuntimeID string
	ServiceID string
	Message   string
	Cause     error
}

func (e *ServiceHealthError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("[%s] runtime=%s service=%s: %s: %v", e.Code, e.RuntimeID, e.ServiceID, e.Message, e.Cause)
	}
	return fmt.Sprintf("[%s] runtime=%s service=%s: %s", e.Code, e.RuntimeID, e.ServiceID, e.Message)
}

func (e *ServiceHealthError) Unwrap() error {
	return e.Cause
}

type QuarantineError struct {
	RuntimeID  string
	ServiceID  string
	Reason     string
	Quarantine bool
	Cause      error
}

func (e *QuarantineError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("[service_quarantined] runtime=%s service=%s reason=%s: %v", e.RuntimeID, e.ServiceID, e.Reason, e.Cause)
	}
	return fmt.Sprintf("[service_quarantined] runtime=%s service=%s reason=%s", e.RuntimeID, e.ServiceID, e.Reason)
}

func (e *QuarantineError) Unwrap() error {
	return e.Cause
}

type CrashHandlerError struct {
	RuntimeID    string
	ServiceID   string
	ExitExpected bool
	Cause        error
}

func (e *CrashHandlerError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("[crash_handler] runtime=%s service=%s expected=%v: %v", e.RuntimeID, e.ServiceID, e.ExitExpected, e.Cause)
	}
	return fmt.Sprintf("[crash_handler] runtime=%s service=%s expected=%v", e.RuntimeID, e.ServiceID, e.ExitExpected)
}

func (e *CrashHandlerError) Unwrap() error {
	return e.Cause
}
