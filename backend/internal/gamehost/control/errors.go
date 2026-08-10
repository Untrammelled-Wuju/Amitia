package control

import (
	"fmt"

	"github.com/u-ai/backend/internal/gamehost/domain"
)

type AuthorityError struct {
	Code    domain.ErrorCode
	Message string
	Cause   error
}

func (e *AuthorityError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("[%s] %s: %v", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

func (e *AuthorityError) Unwrap() error {
	return e.Cause
}

func errRuntimeNotFound(runtimeID domain.RuntimeInstanceID) *AuthorityError {
	return &AuthorityError{
		Code:    domain.ErrNotFound,
		Message: "runtime not found: " + string(runtimeID),
	}
}

func errAuthorityNotFound(runtimeID domain.RuntimeInstanceID) *AuthorityError {
	return &AuthorityError{
		Code:    domain.ErrNotFound,
		Message: "authority not found: " + string(runtimeID),
	}
}

func errInvalidTransition(from domain.ControlMode, to domain.ControlMode) *AuthorityError {
	return &AuthorityError{
		Code:    domain.ErrInvalidArgument,
		Message: "invalid control transition: " + string(from) + " -> " + string(to),
	}
}

func errStaleEpoch(runtimeID domain.RuntimeInstanceID, expected uint64, actual uint64) *AuthorityError {
	return &AuthorityError{
		Code:    domain.ErrInvalidState,
		Message: fmt.Sprintf("stale authority epoch for runtime %s: expected=%d actual=%d", runtimeID, expected, actual),
	}
}

func errInvalidControlMode(mode domain.ControlMode) *AuthorityError {
	return &AuthorityError{
		Code:    domain.ErrInvalidArgument,
		Message: "invalid control mode: " + string(mode),
	}
}

func errAuthorityUnavailable(runtimeID domain.RuntimeInstanceID, reason string) *AuthorityError {
	return &AuthorityError{
		Code:    domain.ErrRuntimeUnavailable,
		Message: "authority unavailable for runtime " + string(runtimeID) + ": " + reason,
	}
}
