package control

import (
	"fmt"
	"time"

	"github.com/u-ai/backend/internal/gamehost/domain"
)

func errPermitRuntimeMismatch(expected domain.RuntimeInstanceID, actual domain.RuntimeInstanceID) *AuthorityError {
	return &AuthorityError{
		Code:    domain.ErrInvalidArgument,
		Message: fmt.Sprintf("permit runtime mismatch: expected=%s actual=%s", expected, actual),
	}
}

func errPermitServiceMismatch(expected domain.ServiceID, actual domain.ServiceID) *AuthorityError {
	return &AuthorityError{
		Code:    domain.ErrInvalidArgument,
		Message: fmt.Sprintf("permit service mismatch: expected=%s actual=%s", expected, actual),
	}
}

func errPermitStale(permitEpoch uint64, currentEpoch uint64) *AuthorityError {
	return &AuthorityError{
		Code:    domain.ErrInvalidState,
		Message: fmt.Sprintf("permit stale epoch: permit=%d current=%d", permitEpoch, currentEpoch),
	}
}

func errPermitStaleGeneration(permitGeneration uint64, currentGeneration uint64) *AuthorityError {
	return &AuthorityError{
		Code:    domain.ErrInvalidState,
		Message: fmt.Sprintf("permit stale generation: permit=%d current=%d", permitGeneration, currentGeneration),
	}
}

func errPermitExpired(permitID string, expiresAt time.Time, now time.Time) *AuthorityError {
	return &AuthorityError{
		Code:    domain.ErrInvalidState,
		Message: fmt.Sprintf("permit %s expired at %s (now %s)", permitID, expiresAt.Format(time.RFC3339), now.Format(time.RFC3339)),
	}
}
