package workspace

import (
	"errors"
)

var (
	ErrRemoteUnavailable          = errors.New("remote unavailable")
	ErrRemoteAuthFailed           = errors.New("remote authentication failed")
	ErrRemoteHostKeyChanged       = errors.New("remote host key changed")
	ErrRemoteTLSFailed            = errors.New("remote TLS verification failed")
	ErrRemotePermissionDenied     = errors.New("remote permission denied")
	ErrRemoteLocked               = errors.New("remote file locked")
	ErrRemoteInsufficientStorage  = errors.New("remote insufficient storage")
	ErrRemoteSymlinkUnsupported   = errors.New("remote symlink unsupported")
	ErrRemoteProtocolUnsupported  = errors.New("remote protocol unsupported")
	ErrRemoteOutcomeUnknown       = errors.New("remote operation outcome unknown")
	ErrRemoteEndpointUnreachable  = errors.New("remote endpoint unreachable")
	ErrRemoteCredentialNotFound   = errors.New("remote credential not found")
	ErrRemoteConfigInvalid        = errors.New("remote config invalid")
	ErrRemoteBoundaryEscaped      = errors.New("remote path escaped root boundary")
	ErrRemoteClientCacheExhausted = errors.New("remote client cache exhausted")
)

type RemoteError struct {
	Code       string
	Message    string
	Underlying error
}

func (e *RemoteError) Error() string {
	if e.Underlying != nil {
		return "remote error [" + e.Code + "]: " + e.Message + ": " + e.Underlying.Error()
	}
	return "remote error [" + e.Code + "]: " + e.Message
}

func (e *RemoteError) Unwrap() error {
	return e.Underlying
}

func NewRemoteError(code string, message string, underlying error) *RemoteError {
	return &RemoteError{
		Code:       code,
		Message:    message,
		Underlying: underlying,
	}
}
