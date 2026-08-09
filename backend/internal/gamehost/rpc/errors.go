package rpc

import (
	"fmt"

	"github.com/u-ai/backend/internal/gamehost/domain"
)

type RPCErrorCode string

const (
	RPCErrorReservedNamespace RPCErrorCode = "namespace_reserved"
	RPCErrorNamespaceConflict RPCErrorCode = "namespace_conflict"
	RPCErrorMethodNotFound    RPCErrorCode = "method_not_found"
	RPCErrorServiceUnavailable RPCErrorCode = "service_unavailable"
)

type RPCError struct {
	Code    RPCErrorCode
	Inner   domain.ErrorCode
	Message string
	Cause   error
}

func (e *RPCError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("[%s/%s] %s: %v", e.Code, e.Inner, e.Message, e.Cause)
	}
	return fmt.Sprintf("[%s/%s] %s", e.Code, e.Inner, e.Message)
}

func (e *RPCError) Unwrap() error {
	return e.Cause
}

func NewRPCError(code RPCErrorCode, inner domain.ErrorCode, message string) *RPCError {
	return &RPCError{
		Code:    code,
		Inner:   inner,
		Message: message,
	}
}

func NewRPCErrorWithCause(code RPCErrorCode, inner domain.ErrorCode, message string, cause error) *RPCError {
	return &RPCError{
		Code:    code,
		Inner:   inner,
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
	if rpcErr, ok := err.(*RPCError); ok {
		return &domain.HostError{
			Code:    rpcErr.Inner,
			Message: rpcErr.Message,
			Cause:   rpcErr.Cause,
		}
	}
	return &domain.HostError{
		Code:    domain.ErrInternal,
		Message: err.Error(),
	}
}
