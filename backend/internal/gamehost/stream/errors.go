package stream

import (
	"github.com/u-ai/backend/internal/gamehost/domain"
)

const (
	ErrEventFailure      = domain.ErrInternal
	ErrStateFailure      = domain.ErrInternal
	ErrSequenceExhausted = domain.ErrInternal
)

var (
	ErrQueueFull = domain.NewHostError(domain.ErrResourceExhausted, "stream: queue full")

	ErrRateLimited = domain.NewHostError(domain.ErrResourceExhausted, "stream: rate limited")

	ErrCursorStale = domain.NewHostError(domain.ErrInvalidArgument, "stream: cursor too old")

	ErrCursorAhead = domain.NewHostError(domain.ErrInvalidArgument, "stream: cursor ahead of latest")

	ErrGenerationMismatch = domain.NewHostError(domain.ErrInvalidArgument, "stream: stream generation mismatch")

	ErrWrongRuntime = domain.NewHostError(domain.ErrInvalidArgument, "stream: cursor runtime mismatch")

	ErrWrongService = domain.NewHostError(domain.ErrInvalidArgument, "stream: cursor service mismatch")

	ErrWrongChannel = domain.NewHostError(domain.ErrInvalidArgument, "stream: cursor channel mismatch")

	ErrStreamClosed = domain.NewHostError(domain.ErrInvalidState, "stream: stream closed")

	ErrSubscriptionClosed = domain.NewHostError(domain.ErrInvalidState, "stream: subscription closed")

	ErrBlockTimeout = domain.NewHostError(domain.ErrResourceExhausted, "stream: block timeout")

	ErrObjectReleased = domain.NewHostError(domain.ErrInvalidState, "stream: binary object already released")

	ErrReplayFull = domain.NewHostError(domain.ErrResourceExhausted, "stream: replay buffer full")
)
