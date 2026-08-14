package deviceruntime

import "errors"

var (
	ErrRuntimeSessionInvalid    = errors.New("device runtime session is invalid")
	ErrRuntimeSessionNotFound   = errors.New("device runtime session not found")
	ErrRuntimeSessionExpired    = errors.New("device runtime session expired")
	ErrRuntimeSessionStale      = errors.New("device runtime session is stale")
	ErrConnectionSuperseded     = errors.New("device runtime connection superseded")
	ErrRuntimeCursorStale       = errors.New("device runtime cursor is stale")
	ErrPresenceProjectionFailed = errors.New("device runtime presence projection failed")
)
