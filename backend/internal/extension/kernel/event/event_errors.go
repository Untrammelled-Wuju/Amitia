package event

import (
	"errors"
	"fmt"
)

var (
	ErrSchemaNotFound                 = errors.New("event: schema not found")
	ErrSchemaConflict                 = errors.New("event: schema conflict")
	ErrSchemaInvalid                  = errors.New("event: schema invalid")
	ErrSubscriptionNotFound           = errors.New("event: subscription not found")
	ErrSubscriptionInvalid            = errors.New("event: subscription invalid")
	ErrSubscriptionConflict           = errors.New("event: subscription conflict")
	ErrDeliveryNotFound               = errors.New("event: delivery not found")
	ErrDeadLetterNotFound             = errors.New("event: dead letter not found")
	ErrInvalidEvent                   = errors.New("event: invalid event")
	ErrInvalidPayload                 = errors.New("event: invalid payload")
	ErrInvalidFilter                  = errors.New("event: invalid filter")
	ErrInvalidProjection              = errors.New("event: invalid projection")
	ErrNoSubscribers                  = errors.New("event: no subscribers")
	ErrDeliveryFailed                 = errors.New("event: delivery failed")
	ErrDeadLetter                     = errors.New("event: dead letter")
	ErrEventLoopDetected              = errors.New("event: loop detected")
	ErrEventDepthExceeded             = errors.New("event: depth exceeded")
	ErrPermissionDenied               = errors.New("event: permission denied")
	ErrScopeDenied                    = errors.New("event: scope denied")
	ErrCircuitOpen                    = errors.New("event: circuit open")
	ErrTimeout                        = errors.New("event: timeout")
	ErrCancelled                      = errors.New("event: cancelled")
	ErrOutboxConflict                 = errors.New("event: outbox conflict")
	ErrOutboxEmpty                    = errors.New("event: outbox empty")
	ErrLeaseExpired                   = errors.New("event: lease expired")
	ErrLeaseHeld                      = errors.New("event: lease held")
	ErrReplayDenied                   = errors.New("event: replay denied")
	ErrProducerDenied                 = errors.New("event: producer denied")
	ErrNamespaceDenied                = errors.New("event: namespace denied")
	ErrBackpressure                   = errors.New("event: backpressure")
	ErrDependencyMissing              = errors.New("event: dependency missing")
	ErrRuntimeUnavailable             = errors.New("event: runtime unavailable")
	ErrRuntimeCrashed                 = errors.New("event: runtime crashed")
	ErrHandlerNotFound                = errors.New("event: handler not found")
	ErrInvalidResult                  = errors.New("event: invalid result")
	ErrProtocolError                  = errors.New("event: protocol error")
	ErrHostAPIAbuse                   = errors.New("event: host api abuse")
	ErrRateLimited                    = errors.New("event: rate limited")
	ErrInvalidSubscription            = errors.New("event: invalid subscription")
	ErrUnsupportedVersion             = errors.New("event: unsupported version")
	ErrExtensionDisabled              = errors.New("event: extension disabled")
	ErrPermanentDependencyMissing     = errors.New("event: permanent dependency missing")
	ErrTemporaryDependencyUnavailable = errors.New("event: temporary dependency unavailable")
	ErrTemporaryHostError             = errors.New("event: temporary host error")
	ErrStaleProducer                  = errors.New("event: stale producer generation")
	ErrStaleSubscription              = errors.New("event: stale subscription generation")
)

func IsRetryable(err error) bool {
	switch {
	case errors.Is(err, ErrRuntimeUnavailable),
		errors.Is(err, ErrRuntimeCrashed),
		errors.Is(err, ErrTimeout),
		errors.Is(err, ErrTemporaryDependencyUnavailable),
		errors.Is(err, ErrTemporaryHostError),
		errors.Is(err, ErrRateLimited):
		return true
	default:
		return false
	}
}

func IsPermanent(err error) bool {
	switch {
	case errors.Is(err, ErrPermissionDenied),
		errors.Is(err, ErrScopeDenied),
		errors.Is(err, ErrInvalidPayload),
		errors.Is(err, ErrInvalidSubscription),
		errors.Is(err, ErrHandlerNotFound),
		errors.Is(err, ErrUnsupportedVersion),
		errors.Is(err, ErrExtensionDisabled),
		errors.Is(err, ErrPermanentDependencyMissing),
		errors.Is(err, ErrEventLoopDetected),
		errors.Is(err, ErrEventDepthExceeded),
		errors.Is(err, ErrInvalidResult),
		errors.Is(err, ErrProtocolError),
		errors.Is(err, ErrHostAPIAbuse),
		errors.Is(err, ErrCircuitOpen):
		return true
	default:
		return false
	}
}

func ErrorCode(err error) string {
	switch {
	case errors.Is(err, ErrRuntimeUnavailable):
		return "runtime_unavailable"
	case errors.Is(err, ErrRuntimeCrashed):
		return "runtime_crashed"
	case errors.Is(err, ErrTimeout):
		return "timeout"
	case errors.Is(err, ErrTemporaryDependencyUnavailable):
		return "temporary_dependency_unavailable"
	case errors.Is(err, ErrTemporaryHostError):
		return "temporary_host_error"
	case errors.Is(err, ErrRateLimited):
		return "rate_limited"
	case errors.Is(err, ErrPermissionDenied):
		return "permission_denied"
	case errors.Is(err, ErrScopeDenied):
		return "scope_denied"
	case errors.Is(err, ErrInvalidPayload):
		return "invalid_payload"
	case errors.Is(err, ErrInvalidSubscription):
		return "invalid_subscription"
	case errors.Is(err, ErrHandlerNotFound):
		return "handler_not_found"
	case errors.Is(err, ErrUnsupportedVersion):
		return "unsupported_event_version"
	case errors.Is(err, ErrExtensionDisabled):
		return "extension_disabled"
	case errors.Is(err, ErrPermanentDependencyMissing):
		return "permanent_dependency_missing"
	case errors.Is(err, ErrEventLoopDetected):
		return "event_loop_detected"
	case errors.Is(err, ErrEventDepthExceeded):
		return "event_depth_exceeded"
	case errors.Is(err, ErrInvalidResult):
		return "invalid_result"
	case errors.Is(err, ErrProtocolError):
		return "protocol_error"
	case errors.Is(err, ErrHostAPIAbuse):
		return "host_api_abuse"
	case errors.Is(err, ErrCircuitOpen):
		return "circuit_open"
	case errors.Is(err, ErrCancelled):
		return "cancelled"
	case errors.Is(err, ErrStaleProducer):
		return "reject_stale_producer"
	case errors.Is(err, ErrStaleSubscription):
		return "cancel_stale_subscription"
	default:
		return fmt.Sprintf("unknown:%v", err)
	}
}
