package recovery

import (
	"fmt"

	"github.com/u-ai/backend/internal/gamehost/domain"
)

type RecoveryError struct {
	Code    RecoveryErrorCode
	Message string
	Cause   error
}

type RecoveryErrorCode string

const (
	ErrCodeRuntimeRecovering    RecoveryErrorCode = "runtime_already_recovering"
	ErrCodeRecoveryExhausted    RecoveryErrorCode = "recovery_exhausted"
	ErrCodeQuarantined          RecoveryErrorCode = "quarantined"
	ErrCodeRollbackUnavailable  RecoveryErrorCode = "rollback_unavailable"
	ErrCodeRollbackFailed       RecoveryErrorCode = "rollback_failed"
	ErrCodeCheckpointCorrupt    RecoveryErrorCode = "checkpoint_corrupt"
	ErrCodeCheckPointIncompatible RecoveryErrorCode = "checkpoint_incompatible"
	ErrCodeMigrationIncompatible RecoveryErrorCode = "migration_incompatible"
	ErrCodeRuntimeRebuildFailed  RecoveryErrorCode = "runtime_rebuild_failed"
	ErrCodeRuntimeRestartFailed  RecoveryErrorCode = "runtime_restart_failed"
	ErrCodeInternal              RecoveryErrorCode = "internal"
)

func (e *RecoveryError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("recovery[%s]: %s: %v", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("recovery[%s]: %s", e.Code, e.Message)
}

func (e *RecoveryError) Unwrap() error {
	return e.Cause
}

func NewRuntimeAlreadyRecoveringError(runtimeID domain.RuntimeInstanceID) *RecoveryError {
	return &RecoveryError{
		Code:    ErrCodeRuntimeRecovering,
		Message: fmt.Sprintf("runtime %s already has an active recovery operation", runtimeID),
	}
}

func NewRecoveryExhaustedError(runtimeID domain.RuntimeInstanceID, attempts int, maxAttempts int) *RecoveryError {
	return &RecoveryError{
		Code:    ErrCodeRecoveryExhausted,
		Message: fmt.Sprintf("runtime %s recovery exhausted after %d/%d attempts", runtimeID, attempts, maxAttempts),
	}
}

func NewQuarantinedError(runtimeID domain.RuntimeInstanceID, reason string) *RecoveryError {
	return &RecoveryError{
		Code:    ErrCodeQuarantined,
		Message: fmt.Sprintf("runtime %s quarantined: %s", runtimeID, reason),
	}
}

func NewRollbackUnavailableError(extensionID string, cause error) *RecoveryError {
	return &RecoveryError{
		Code:    ErrCodeRollbackUnavailable,
		Message: fmt.Sprintf("rollback unavailable for extension %s", extensionID),
		Cause:   cause,
	}
}

func NewRollbackFailedError(extensionID string, cause error) *RecoveryError {
	return &RecoveryError{
		Code:    ErrCodeRollbackFailed,
		Message: fmt.Sprintf("rollback failed for extension %s", extensionID),
		Cause:   cause,
	}
}
