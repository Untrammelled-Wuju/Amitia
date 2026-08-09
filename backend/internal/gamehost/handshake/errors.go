package handshake

import (
	"fmt"

	"github.com/u-ai/backend/internal/gamehost/domain"
)

type HandshakeErrorType string

const (
	HandshakeErrorRequired          HandshakeErrorType = "handshake_required"
	HandshakeErrorTimeout           HandshakeErrorType = "handshake_timeout"
	HandshakeErrorAlreadyCompleted  HandshakeErrorType = "handshake_already_completed"
	HandshakeErrorIdentityMismatch  HandshakeErrorType = "peer_identity_mismatch"
	HandshakeErrorRuntimeNotFound   HandshakeErrorType = "runtime_not_found"
	HandshakeErrorServiceNotFound   HandshakeErrorType = "service_not_found"
	HandshakeErrorProtocolMismatch  HandshakeErrorType = "protocol_mismatch"
	HandshakeErrorCapabilityMismatch HandshakeErrorType = "capability_mismatch"
	HandshakeErrorNamespaceInvalid  HandshakeErrorType = "namespace_invalid"
	HandshakeErrorNamespaceConflict HandshakeErrorType = "namespace_conflict"
	HandshakeErrorChannelInvalid    HandshakeErrorType = "channel_advertisement_invalid"
	HandshakeErrorConnectionClosed  HandshakeErrorType = "connection_closed"
)

type HandshakeError struct {
	Type    HandshakeErrorType
	Inner   domain.ErrorCode
	Message string
}

func (e *HandshakeError) Error() string {
	return fmt.Sprintf("[%s/%s] %s", e.Type, e.Inner, e.Message)
}

func NewHandshakeError(t HandshakeErrorType, inner domain.ErrorCode, message string) *HandshakeError {
	return &HandshakeError{
		Type:    t,
		Inner:   inner,
		Message: message,
	}
}

func MapToProtocolError(err *HandshakeError) *ProtocolErrorWrapper {
	if err == nil {
		return nil
	}

	switch err.Type {
	case HandshakeErrorRequired:
		return &ProtocolErrorWrapper{
			Code:    "invalid_request",
			Message: "handshake required: " + err.Message,
		}
	case HandshakeErrorTimeout:
		return &ProtocolErrorWrapper{
			Code:    "timeout",
			Message: "handshake timed out: " + err.Message,
		}
	case HandshakeErrorProtocolMismatch:
		return &ProtocolErrorWrapper{
			Code:    "protocol_mismatch",
			Message: err.Message,
		}
	case HandshakeErrorCapabilityMismatch:
		return &ProtocolErrorWrapper{
			Code:    "capability_unsupported",
			Message: err.Message,
		}
	case HandshakeErrorNamespaceInvalid, HandshakeErrorNamespaceConflict:
		return &ProtocolErrorWrapper{
			Code:    "invalid_request",
			Message: err.Message,
		}
	case HandshakeErrorAlreadyCompleted:
		return &ProtocolErrorWrapper{
			Code:    "already_exists",
			Message: err.Message,
		}
	case HandshakeErrorRuntimeNotFound, HandshakeErrorServiceNotFound:
		return &ProtocolErrorWrapper{
			Code:    "not_found",
			Message: err.Message,
		}
	case HandshakeErrorIdentityMismatch:
		return &ProtocolErrorWrapper{
			Code:    "invalid_request",
			Message: err.Message,
		}
	default:
		return &ProtocolErrorWrapper{
			Code:    "internal",
			Message: err.Message,
		}
	}
}

type ProtocolErrorWrapper struct {
	Code    string
	Message string
}
