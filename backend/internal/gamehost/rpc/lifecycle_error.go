package rpc

import (
	"context"
	"errors"
	"fmt"

	"github.com/u-ai/backend/internal/gamehost/domain"
	"github.com/u-ai/backend/pkg/gameplugin/protocol"
)

type LifecycleErrorCode string

const (
	LifecycleErrorTimeout     LifecycleErrorCode = "timeout"
	LifecycleErrorCancelled   LifecycleErrorCode = "cancelled"
	LifecycleErrorInternal    LifecycleErrorCode = "internal"
)

type LifecycleError struct {
	Code    LifecycleErrorCode
	Inner   domain.ErrorCode
	Message string
	Cause   error
}

func (e *LifecycleError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("[%s/%s] %s: %v", e.Code, e.Inner, e.Message, e.Cause)
	}
	return fmt.Sprintf("[%s/%s] %s", e.Code, e.Inner, e.Message)
}

func (e *LifecycleError) Unwrap() error {
	return e.Cause
}

func NewLifecycleError(code LifecycleErrorCode, inner domain.ErrorCode, message string, cause error) *LifecycleError {
	return &LifecycleError{
		Code:    code,
		Inner:   inner,
		Message: message,
		Cause:   cause,
	}
}

func TimeoutError(deadline string) *protocol.ProtocolError {
	return &protocol.ProtocolError{
		Code:    protocol.ErrorCode("timeout"),
		Message: fmt.Sprintf("request timed out at %s", deadline),
	}
}

func CancelledError() *protocol.ProtocolError {
	return &protocol.ProtocolError{
		Code:    protocol.ErrorCode("cancelled"),
		Message: "request was cancelled",
	}
}

func InternalError(msg string) *protocol.ProtocolError {
	return &protocol.ProtocolError{
		Code:    protocol.ErrorCode("internal"),
		Message: msg,
	}
}

func ServiceUnavailableError(msg string) *protocol.ProtocolError {
	return &protocol.ProtocolError{
		Code:    protocol.ErrorCode("runtime_unavailable"),
		Message: msg,
	}
}

func ResourceExhaustedError(msg string) *protocol.ProtocolError {
	return &protocol.ProtocolError{
		Code:    protocol.ErrorCode("resource_exhausted"),
		Message: msg,
	}
}

func MapLifecycleError(err error) *protocol.ProtocolError {
	if err == nil {
		return nil
	}

	if errors.Is(err, context.DeadlineExceeded) {
		return TimeoutError("")
	}

	if errors.Is(err, context.Canceled) {
		return CancelledError()
	}

	var lifecycleErr *LifecycleError
	if errors.As(err, &lifecycleErr) {
		switch lifecycleErr.Code {
		case LifecycleErrorTimeout:
			return TimeoutError("")
		case LifecycleErrorCancelled:
			return CancelledError()
		case LifecycleErrorInternal:
			return InternalError(lifecycleErr.Message)
		}
	}

	return InternalError(err.Error())
}

