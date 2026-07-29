package runtime_supervisor

import (
	"errors"
	"fmt"
)

const (
	CodeRuntimeReconcileFailed = "RUNTIME_RECONCILE_FAILED"
	CodeRuntimeStartFailed     = "RUNTIME_START_FAILED"
	CodeRuntimeStopFailed      = "RUNTIME_STOP_FAILED"
	CodeRuntimeDrainTimeout    = "RUNTIME_DRAIN_TIMEOUT"
	CodeRuntimeHealthFailed    = "RUNTIME_HEALTH_FAILED"
)

var (
	ErrRuntimeReconcileFailed = errors.New("runtime_supervisor: reconcile failed")
	ErrRuntimeStartFailed     = errors.New("runtime_supervisor: start failed")
	ErrRuntimeStopFailed      = errors.New("runtime_supervisor: stop failed")
	ErrRuntimeDrainTimeout    = errors.New("runtime_supervisor: drain timeout")
	ErrRuntimeHealthFailed    = errors.New("runtime_supervisor: health check failed")
)

type RuntimeError struct {
	Code   string
	Cause  error
	Detail string
}

func (e *RuntimeError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("runtime_supervisor[%s]: %s: %v", e.Code, e.Detail, e.Cause)
	}
	return fmt.Sprintf("runtime_supervisor[%s]: %s", e.Code, e.Detail)
}

func (e *RuntimeError) Unwrap() error { return e.Cause }

func NewRuntimeError(code, detail string, cause error) *RuntimeError {
	return &RuntimeError{Code: code, Detail: detail, Cause: cause}
}

func ClassifyReconcileError(result ReconcileResult) *RuntimeError {
	if result.Error == nil {
		return nil
	}
	switch result.Actual {
	case ActualFailed:
		if result.Desired == DesiredRunning || result.Desired == DesiredConnected {
			return NewRuntimeError(CodeRuntimeStartFailed, fmt.Sprintf("extension=%s module=%s", result.DefinitionID, result.InstanceID), result.Error)
		}
		return NewRuntimeError(CodeRuntimeReconcileFailed, fmt.Sprintf("extension=%s", result.DefinitionID), result.Error)
	case ActualCrashed:
		return NewRuntimeError(CodeRuntimeStartFailed, fmt.Sprintf("extension=%s instance=%s", result.DefinitionID, result.InstanceID), result.Error)
	case ActualQuarantined:
		return NewRuntimeError(CodeRuntimeReconcileFailed, fmt.Sprintf("extension=%s instance=%s quarantined", result.DefinitionID, result.InstanceID), result.Error)
	default:
		return NewRuntimeError(CodeRuntimeReconcileFailed, fmt.Sprintf("extension=%s actual=%s", result.DefinitionID, result.Actual), result.Error)
	}
}
