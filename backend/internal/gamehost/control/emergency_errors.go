package control

import (
	"fmt"

	"github.com/u-ai/backend/internal/gamehost/domain"
)

func errEmergencyRuntimeNotFound(runtimeID domain.RuntimeInstanceID) *AuthorityError {
	return &AuthorityError{
		Code:    domain.ErrNotFound,
		Message: "emergency stop: runtime not found: " + string(runtimeID),
	}
}

func errEmergencyStateConflict(runtimeID domain.RuntimeInstanceID, current EmergencyStopState) *AuthorityError {
	return &AuthorityError{
		Code:    domain.ErrInvalidState,
		Message: fmt.Sprintf("emergency stop: state conflict for %s: already %s", runtimeID, current),
	}
}

func errEmergencyCriticalFailure(stage EmergencyStopState, cause string) *AuthorityError {
	return &AuthorityError{
		Code:    domain.ErrInvalidState,
		Message: fmt.Sprintf("emergency stop critical failure at stage %s: %s", stage, cause),
	}
}

func errEmergencyDeadlineExceeded(operationID string) *AuthorityError {
	return &AuthorityError{
		Code:    domain.ErrResourceExhausted,
		Message: "emergency stop: deadline exceeded for operation " + operationID,
	}
}

func errEmergencyVerificationFailed(operationID string) *AuthorityError {
	return &AuthorityError{
		Code:    domain.ErrInvalidState,
		Message: "emergency stop: verification failed for operation " + operationID,
	}
}
