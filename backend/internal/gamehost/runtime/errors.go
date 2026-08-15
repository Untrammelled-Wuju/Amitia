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

	ErrRuntimeUnavailable     ErrorCode = "runtime_unavailable"
	ErrServiceUnavailable     ErrorCode = "service_unavailable"
	ErrRuntimeOperationLocked ErrorCode = "runtime_operation_locked"
	ErrDefinitionNotResolved  ErrorCode = "definition_not_resolved"
	ErrServiceLaunchFailed    ErrorCode = "service_launch_failed"
	ErrRollbackFailed         ErrorCode = "rollback_failed"
	ErrShutdownFailed         ErrorCode = "shutdown_failed"
	ErrExecutionTimeout       ErrorCode = "execution_timeout"
	ErrDependencyNotSatisfied ErrorCode = "dependency_not_satisfied"
	ErrRestartFailed          ErrorCode = "restart_failed"
	ErrLifecycleConflict      ErrorCode = "lifecycle_conflict"
	ErrSupersededByEmergency  ErrorCode = "superseded_by_emergency"
	ErrSupersededByDisable    ErrorCode = "superseded_by_disable"
	ErrSupersededByUninstall  ErrorCode = "superseded_by_uninstall"
	ErrRevisionChanged        ErrorCode = "revision_changed"
	ErrEmergencyActive        ErrorCode = "emergency_active"
)

type TopologyError struct {
	Code       ErrorCode
	Message    string
	Retryable  bool
	Cause      error
	RuntimeID  string
	ServiceID  string
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

type ExecutionError struct {
	Code       ErrorCode
	RuntimeID  string
	PluginID   string
	ServiceID  string
	DefinitionID string
	Message    string
	Cause      error
}

func (e *ExecutionError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("[%s] runtime=%s plugin=%s service=%s def=%s: %s: %v",
			e.Code, e.RuntimeID, e.PluginID, e.ServiceID, e.DefinitionID, e.Message, e.Cause)
	}
	return fmt.Sprintf("[%s] runtime=%s plugin=%s service=%s def=%s: %s",
		e.Code, e.RuntimeID, e.PluginID, e.ServiceID, e.DefinitionID, e.Message)
}

func (e *ExecutionError) Unwrap() error {
	return e.Cause
}

type RuntimeStartError struct {
	Cause          error
	RuntimeID      string
	RollbackErrors []error
}

func (e *RuntimeStartError) Error() string {
	if len(e.RollbackErrors) > 0 {
		return fmt.Sprintf("runtime_start_failed: runtime=%s cause=%v rollback_errors=%d",
			e.RuntimeID, e.Cause, len(e.RollbackErrors))
	}
	return fmt.Sprintf("runtime_start_failed: runtime=%s cause=%v", e.RuntimeID, e.Cause)
}

func (e *RuntimeStartError) Unwrap() error {
	return e.Cause
}

type RuntimeStopError struct {
	RuntimeID   string
	StopErrors  []error
	CleanupErrors []error
}

func (e *RuntimeStopError) Error() string {
	return fmt.Sprintf("runtime_stop_failed: runtime=%s stop_errors=%d cleanup_errors=%d",
		e.RuntimeID, len(e.StopErrors), len(e.CleanupErrors))
}

type RuntimeRestartError struct {
	Code           ErrorCode
	RuntimeID      string
	OldGeneration  int64
	NewGeneration  int64
	Cause          error
	StopErrors     []error
	StartErrors    []error
	RollbackErrors []error
}

func (e *RuntimeRestartError) Error() string {
	return fmt.Sprintf("runtime_restart_failed: runtime=%s code=%s old_gen=%d new_gen=%d cause=%v",
		e.RuntimeID, e.Code, e.OldGeneration, e.NewGeneration, e.Cause)
}

func (e *RuntimeRestartError) Unwrap() error {
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

func NewExecutionError(code ErrorCode, runtimeID, pluginID, serviceID, definitionID, message string) *ExecutionError {
	return &ExecutionError{
		Code:         code,
		RuntimeID:    runtimeID,
		PluginID:     pluginID,
		ServiceID:    serviceID,
		DefinitionID: definitionID,
		Message:      message,
	}
}

func NewExecutionErrorWithCause(code ErrorCode, runtimeID, pluginID, serviceID, definitionID, message string, cause error) *ExecutionError {
	return &ExecutionError{
		Code:         code,
		RuntimeID:    runtimeID,
		PluginID:     pluginID,
		ServiceID:    serviceID,
		DefinitionID: definitionID,
		Message:      message,
		Cause:        cause,
	}
}

func IsTopologyError(err error, code ErrorCode) bool {
	te, ok := err.(*TopologyError)
	if !ok {
		return false
	}
	return te.Code == code
}

func IsExecutionError(err error, code ErrorCode) bool {
	te, ok := err.(*ExecutionError)
	if !ok {
		return false
	}
	return te.Code == code
}

func IsRuntimeRestartError(err error, code ErrorCode) bool {
	re, ok := err.(*RuntimeRestartError)
	if !ok {
		return false
	}
	return re.Code == code
}
