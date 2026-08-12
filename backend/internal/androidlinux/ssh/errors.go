//go:build linux && !android

package ssh

import "fmt"

type SSHError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (e *SSHError) Error() string {
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

const (
	ErrCodeInvalidRequest     = "INVALID_REQUEST"
	ErrCodeHostDenied         = "HOST_DENIED"
	ErrCodePortDenied         = "PORT_DENIED"
	ErrCodeAuthFailed         = "AUTH_FAILED"
	ErrCodeConnectionFailed   = "CONNECTION_FAILED"
	ErrCodeHostKeyUnknown     = "HOST_KEY_UNKNOWN"
	ErrCodeHostKeyMismatch    = "HOST_KEY_MISMATCH"
	ErrCodeHostKeyDenied      = "HOST_KEY_DENIED"
	ErrCodeTimeout            = "TIMEOUT"
	ErrCodeCommandFailed      = "COMMAND_FAILED"
	ErrCodeOutputTooLarge     = "OUTPUT_TOO_LARGE"
	ErrCodeInvalidHostKey     = "INVALID_HOST_KEY"
	ErrCodeInvalidFingerprint = "INVALID_FINGERPRINT"
	ErrCodeUnsupportedAuth    = "UNSUPPORTED_AUTH"
	ErrCodeInternal           = "INTERNAL"
)

func ErrInvalidRequest(msg string) error {
	return &SSHError{Code: ErrCodeInvalidRequest, Message: msg}
}

func ErrHostDenied(host string) error {
	return &SSHError{Code: ErrCodeHostDenied, Message: fmt.Sprintf("host not allowed: %s", host)}
}

func ErrPortDenied(port int) error {
	return &SSHError{Code: ErrCodePortDenied, Message: fmt.Sprintf("port not allowed: %d", port)}
}

func ErrAuthFailed(msg string) error {
	return &SSHError{Code: ErrCodeAuthFailed, Message: msg}
}

func ErrConnectionFailed(msg string) error {
	return &SSHError{Code: ErrCodeConnectionFailed, Message: msg}
}

func ErrHostKeyUnknown(host string) error {
	return &SSHError{Code: ErrCodeHostKeyUnknown, Message: fmt.Sprintf("host key not known: %s", host)}
}

func ErrHostKeyMismatch(host string) error {
	return &SSHError{Code: ErrCodeHostKeyMismatch, Message: fmt.Sprintf("host key mismatch: %s", host)}
}

func ErrHostKeyDenied(host string) error {
	return &SSHError{Code: ErrCodeHostKeyDenied, Message: fmt.Sprintf("host key verification disabled: %s", host)}
}

func ErrTimeout(msg string) error {
	return &SSHError{Code: ErrCodeTimeout, Message: msg}
}

func ErrCommandFailed(msg string) error {
	return &SSHError{Code: ErrCodeCommandFailed, Message: msg}
}

func ErrOutputTooLarge(maxBytes int64) error {
	return &SSHError{Code: ErrCodeOutputTooLarge, Message: fmt.Sprintf("output exceeds %d bytes", maxBytes)}
}

func ErrInvalidHostKey(msg string) error {
	return &SSHError{Code: ErrCodeInvalidHostKey, Message: msg}
}

func ErrInvalidFingerprint(msg string) error {
	return &SSHError{Code: ErrCodeInvalidFingerprint, Message: msg}
}

func ErrUnsupportedAuth(msg string) error {
	return &SSHError{Code: ErrCodeUnsupportedAuth, Message: msg}
}

func ErrInternal(msg string) error {
	return &SSHError{Code: ErrCodeInternal, Message: msg}
}
