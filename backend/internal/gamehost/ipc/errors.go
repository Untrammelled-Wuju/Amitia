package ipc

import (
	"fmt"

	"github.com/u-ai/backend/internal/gamehost/domain"
)

type IPCErrorType string

const (
	IPCErrorTransport   IPCErrorType = "transport"
	IPCErrorProtocol    IPCErrorType = "protocol"
	IPCErrorPeerRoute   IPCErrorType = "peer_route"
	IPCErrorLimit       IPCErrorType = "limit"
	IPCErrorDuplicate   IPCErrorType = "duplicate"
)

type IPCError struct {
	Type    IPCErrorType
	Code    domain.ErrorCode
	Message string
	Cause   error
}

func (e *IPCError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("[%s/%s] %s: %v", e.Type, e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("[%s/%s] %s", e.Type, e.Code, e.Message)
}

func (e *IPCError) Unwrap() error {
	return e.Cause
}

func NewIPCError(errType IPCErrorType, code domain.ErrorCode, message string) *IPCError {
	return &IPCError{
		Type:    errType,
		Code:    code,
		Message: message,
	}
}

func NewIPCErrorWithCause(errType IPCErrorType, code domain.ErrorCode, message string, cause error) *IPCError {
	return &IPCError{
		Type:    errType,
		Code:    code,
		Message: message,
		Cause:   cause,
	}
}

func ToHostError(err error) *domain.HostError {
	if err == nil {
		return nil
	}
	if he, ok := err.(*domain.HostError); ok {
		return he
	}
	if ipcErr, ok := err.(*IPCError); ok {
		return &domain.HostError{
			Code:    ipcErr.Code,
			Message: ipcErr.Message,
			Cause:   ipcErr.Cause,
		}
	}
	return &domain.HostError{
		Code:    domain.ErrInternal,
		Message: err.Error(),
	}
}
