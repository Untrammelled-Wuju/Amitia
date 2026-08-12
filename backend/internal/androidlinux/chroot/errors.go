//go:build linux && !android

package chroot

import "fmt"

type ChrootError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (e *ChrootError) Error() string {
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

const (
	ErrCodeInvalidRequest    = "INVALID_REQUEST"
	ErrCodeRootFSDenied      = "ROOTFS_DENIED"
	ErrCodeRootFSNotFound    = "ROOTFS_NOT_FOUND"
	ErrCodeRootFSInvalid     = "ROOTFS_INVALID"
	ErrCodeExecDenied        = "EXEC_DENIED"
	ErrCodeBinaryBlocked     = "BINARY_BLOCKED"
	ErrCodeEnvBlocked        = "ENV_BLOCKED"
	ErrCodeTimeout           = "TIMEOUT"
	ErrCodeCommandFailed     = "COMMAND_FAILED"
	ErrCodeInternal          = "INTERNAL"
)

func ErrInvalidRequest(msg string) error {
	return &ChrootError{Code: ErrCodeInvalidRequest, Message: msg}
}

func ErrRootFSDenied(path string) error {
	return &ChrootError{Code: ErrCodeRootFSDenied, Message: fmt.Sprintf("rootfs not allowed: %s", path)}
}

func ErrRootFSNotFound(path string) error {
	return &ChrootError{Code: ErrCodeRootFSNotFound, Message: fmt.Sprintf("rootfs not found: %s", path)}
}

func ErrRootFSInvalid(msg string) error {
	return &ChrootError{Code: ErrCodeRootFSInvalid, Message: msg}
}

func ErrExecDenied(msg string) error {
	return &ChrootError{Code: ErrCodeExecDenied, Message: msg}
}

func ErrBinaryBlocked(binary string) error {
	return &ChrootError{Code: ErrCodeBinaryBlocked, Message: fmt.Sprintf("binary not allowed: %s", binary)}
}

func ErrEnvBlocked(key string) error {
	return &ChrootError{Code: ErrCodeEnvBlocked, Message: fmt.Sprintf("env variable not allowed: %s", key)}
}

func ErrTimeout(msg string) error {
	return &ChrootError{Code: ErrCodeTimeout, Message: msg}
}

func ErrCommandFailed(msg string) error {
	return &ChrootError{Code: ErrCodeCommandFailed, Message: msg}
}

func ErrInternal(msg string) error {
	return &ChrootError{Code: ErrCodeInternal, Message: msg}
}
