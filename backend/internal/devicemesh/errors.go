package devicemesh

import "errors"

var (
	ErrBootstrapRequired      = errors.New("mesh: bootstrap required")
	ErrBootstrapInvalid       = errors.New("mesh: bootstrap invalid")
	ErrBootstrapExpired       = errors.New("mesh: bootstrap expired")
	ErrBootstrapConsumed      = errors.New("mesh: bootstrap consumed")
	ErrCredentialInvalid      = errors.New("mesh: credential invalid")
	ErrCredentialExpired      = errors.New("mesh: credential expired")
	ErrCredentialRevoked      = errors.New("mesh: credential revoked")
	ErrIdentityMismatch       = errors.New("mesh: identity mismatch")
	ErrProtocolIncompatible   = errors.New("mesh: protocol incompatible")
	ErrHelloTimeout           = errors.New("mesh: hello timeout")
	ErrSessionSuperseded      = errors.New("mesh: session superseded")
	ErrCursorResetRequired    = errors.New("mesh: cursor reset required")
	ErrConnectionUnavailable  = errors.New("mesh: connection unavailable")
	ErrUnsupportedCommand     = errors.New("mesh: unsupported command")
	ErrDeviceNotFound         = errors.New("mesh: device not found")
	ErrDeviceOwnedByOther     = errors.New("mesh: device owned by another user")
	ErrDeviceRevoked          = errors.New("mesh: device revoked")
	ErrCredentialNotFound     = errors.New("mesh: credential not found")
	ErrAlreadyProvisioned     = errors.New("mesh: already provisioned")
	ErrNotProvisioned         = errors.New("mesh: not provisioned")
	ErrInvalidCloudURL        = errors.New("mesh: invalid cloud url")
	ErrSequenceGap            = errors.New("mesh: sequence gap")
)

type MeshError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (e MeshError) Error() string {
	return e.Code + ": " + e.Message
}

func NewMeshError(code, message string) MeshError {
	return MeshError{Code: code, Message: message}
}
