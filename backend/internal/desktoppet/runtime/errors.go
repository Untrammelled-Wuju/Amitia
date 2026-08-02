// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package runtime

import "errors"

const (
	ErrCodeRuntimeDisabled             = "RUNTIME_DISABLED"
	ErrCodeRuntimeOffline              = "RUNTIME_OFFLINE"
	ErrCodeRuntimeNotReady             = "RUNTIME_NOT_READY"
	ErrCodeRuntimeAmbiguous            = "RUNTIME_AMBIGUOUS"
	ErrCodeRuntimeUnauthorized         = "RUNTIME_UNAUTHORIZED"
	ErrCodeRuntimeForbiddenOrigin      = "RUNTIME_FORBIDDEN_ORIGIN"
	ErrCodeRuntimeProtocolIncompatible = "RUNTIME_PROTOCOL_INCOMPATIBLE"
	ErrCodeRuntimeCapabilityMissing    = "RUNTIME_CAPABILITY_MISSING"
	ErrCodeRuntimeBusy                 = "RUNTIME_BUSY"
	ErrCodeRuntimeCommandTimeout       = "RUNTIME_COMMAND_TIMEOUT"
	ErrCodeRuntimeCommandRejected      = "RUNTIME_COMMAND_REJECTED"
	ErrCodeRuntimeCommandFailed        = "RUNTIME_COMMAND_FAILED"
	ErrCodeRuntimeDisconnected         = "RUNTIME_DISCONNECTED"
	ErrCodeRuntimeSessionReplaced      = "RUNTIME_SESSION_REPLACED"
	ErrCodeRuntimeHeartbeatTimeout     = "RUNTIME_HEARTBEAT_TIMEOUT"
	ErrCodeRuntimeBackpressure         = "RUNTIME_BACKPRESSURE"
	ErrCodeRuntimeMessageTooLarge      = "RUNTIME_MESSAGE_TOO_LARGE"
	ErrCodeRuntimeProtocolError        = "RUNTIME_PROTOCOL_ERROR"
	ErrCodeRuntimeStateDiverged        = "RUNTIME_STATE_DIVERGED"
	ErrCodeRuntimeSnapshotInvalid      = "RUNTIME_SNAPSHOT_INVALID"
	ErrCodeRuntimeCommandStoreFailed   = "RUNTIME_COMMAND_STORE_FAILED"
	ErrCodeBackendShuttingDown         = "BACKEND_SHUTTING_DOWN"
)

var (
	ErrRuntimeDisabled             = errors.New("runtime bridge is disabled")
	ErrRuntimeOffline              = errors.New("no target runtime online")
	ErrRuntimeNotReady             = errors.New("runtime connected but not synced")
	ErrRuntimeAmbiguous            = errors.New("multiple candidate runtimes")
	ErrRuntimeUnauthorized         = errors.New("runtime handshake unauthorized")
	ErrRuntimeForbiddenOrigin      = errors.New("runtime origin forbidden")
	ErrRuntimeProtocolIncompatible = errors.New("protocol version incompatible")
	ErrRuntimeCapabilityMissing    = errors.New("runtime capability missing")
	ErrRuntimeBusy                 = errors.New("runtime send queue full")
	ErrRuntimeCommandTimeout       = errors.New("command timeout waiting for applied")
	ErrRuntimeCommandRejected      = errors.New("runtime rejected command")
	ErrRuntimeCommandFailed        = errors.New("runtime command execution failed")
	ErrRuntimeDisconnected         = errors.New("runtime disconnected")
	ErrRuntimeSessionReplaced      = errors.New("runtime session replaced")
	ErrRuntimeHeartbeatTimeout     = errors.New("runtime heartbeat timeout")
	ErrRuntimeBackpressure         = errors.New("runtime backpressure")
	ErrRuntimeMessageTooLarge      = errors.New("runtime message too large")
	ErrRuntimeProtocolError        = errors.New("runtime protocol error")
	ErrRuntimeStateDiverged        = errors.New("runtime state diverged")
	ErrRuntimeSnapshotInvalid      = errors.New("desired snapshot invalid")
	ErrRuntimeCommandStoreFailed   = errors.New("command store failed")
	ErrBackendShuttingDown         = errors.New("backend shutting down")
)

type RuntimeError struct {
	Code    string
	Message string
	Err     error
}

func (e *RuntimeError) Error() string {
	if e.Err != nil {
		return e.Message + ": " + e.Err.Error()
	}
	return e.Message
}

func (e *RuntimeError) Unwrap() error { return e.Err }

func (e *RuntimeError) GetCode() string { return e.Code }

func NewRuntimeError(code, message string, err error) *RuntimeError {
	return &RuntimeError{Code: code, Message: message, Err: err}
}

func MapRuntimeErrorCodeToHTTP(code string) int {
	switch code {
	case ErrCodeRuntimeDisabled, ErrCodeRuntimeOffline, ErrCodeRuntimeDisconnected,
		ErrCodeRuntimeHeartbeatTimeout, ErrCodeRuntimeSessionReplaced, ErrCodeBackendShuttingDown:
		return 503
	case ErrCodeRuntimeNotReady, ErrCodeRuntimeAmbiguous, ErrCodeRuntimeCapabilityMissing,
		ErrCodeRuntimeCommandRejected, ErrCodeRuntimeStateDiverged:
		return 409
	case ErrCodeRuntimeUnauthorized:
		return 401
	case ErrCodeRuntimeForbiddenOrigin:
		return 403
	case ErrCodeRuntimeProtocolIncompatible:
		return 426
	case ErrCodeRuntimeBusy, ErrCodeRuntimeBackpressure:
		return 429
	case ErrCodeRuntimeCommandTimeout:
		return 504
	case ErrCodeRuntimeCommandFailed, ErrCodeRuntimeSnapshotInvalid, ErrCodeRuntimeCommandStoreFailed:
		return 500
	case ErrCodeRuntimeMessageTooLarge, ErrCodeRuntimeProtocolError:
		return 400
	default:
		return 500
	}
}
