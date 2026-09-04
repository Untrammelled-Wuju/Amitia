package protocol

import (
	"encoding/json"
	"fmt"
	"strings"
	"unicode"
)

type ErrorCode string

const (
	ErrorInvalidRequest        ErrorCode = "invalid_request"
	ErrorInvalidArgument       ErrorCode = "invalid_argument"
	ErrorNotFound              ErrorCode = "not_found"
	ErrorAlreadyExists         ErrorCode = "already_exists"
	ErrorUnsupported           ErrorCode = "unsupported"

	ErrorProtocolMismatch      ErrorCode = "protocol_mismatch"
	ErrorCapabilityUnsupported ErrorCode = "capability_unsupported"

	ErrorRuntimeUnavailable    ErrorCode = "runtime_unavailable"
	ErrorServiceUnavailable    ErrorCode = "service_unavailable"
	ErrorInvalidRuntimeState   ErrorCode = "invalid_runtime_state"

	ErrorPermissionDenied      ErrorCode = "permission_denied"
	ErrorResourceExhausted     ErrorCode = "resource_exhausted"

	ErrorTimeout               ErrorCode = "timeout"
	ErrorCancelled             ErrorCode = "cancelled"

	ErrorInternal              ErrorCode = "internal"
)

type ProtocolError struct {
	Code      ErrorCode       `json:"code"`
	Message   string          `json:"message"`
	Retryable bool            `json:"retryable,omitempty"`
	Data      json.RawMessage `json:"data,omitempty"`
}

func DefaultRetryable(code ErrorCode) bool {
	switch code {
	case ErrorRuntimeUnavailable, ErrorServiceUnavailable, ErrorResourceExhausted, ErrorTimeout:
		return true
	default:
		return false
	}
}

func IsKnownErrorCode(code ErrorCode) bool {
	switch code {
	case ErrorInvalidRequest, ErrorInvalidArgument, ErrorNotFound,
		ErrorAlreadyExists, ErrorUnsupported, ErrorProtocolMismatch,
		ErrorCapabilityUnsupported, ErrorRuntimeUnavailable, ErrorServiceUnavailable,
		ErrorInvalidRuntimeState, ErrorPermissionDenied, ErrorResourceExhausted,
		ErrorTimeout, ErrorCancelled, ErrorInternal:
		return true
	default:
		return false
	}
}

func ValidateErrorCode(code ErrorCode) error {
	if code == "" {
		return fmt.Errorf("error code must not be empty")
	}
	const maxLength = 256
	if len(code) > maxLength {
		return fmt.Errorf("error code exceeds maximum length of %d", maxLength)
	}
	for _, r := range code {
		if unicode.IsControl(r) {
			return fmt.Errorf("error code contains control character")
		}
	}
	if IsKnownErrorCode(code) {
		return nil
	}
	if !strings.Contains(string(code), ".") {
		return fmt.Errorf("unknown bare error code: %s", code)
	}
	parts := strings.Split(string(code), ".")
	for _, part := range parts {
		if part == "" {
			return fmt.Errorf("error code parts must not be empty")
		}
	}
	return nil
}

func (e ProtocolError) Validate() error {
	if err := ValidateErrorCode(e.Code); err != nil {
		return err
	}
	if e.Message == "" {
		return fmt.Errorf("error message must not be empty")
	}
	return nil
}

func NewErrorEnvelope(id string, requestID string, code ErrorCode, message string, retryable bool) Envelope {
	return Envelope{
		Protocol:  ProtocolVersion,
		Type:      MessageTypeError,
		ID:        id,
		RequestID: requestID,
		Error: &ProtocolError{
			Code:      code,
			Message:   message,
			Retryable: retryable,
		},
	}
}

func NewErrorEnvelopeWithData(id string, requestID string, code ErrorCode, message string, retryable bool, data json.RawMessage) Envelope {
	return Envelope{
		Protocol:  ProtocolVersion,
		Type:      MessageTypeError,
		ID:        id,
		RequestID: requestID,
		Error: &ProtocolError{
			Code:      code,
			Message:   message,
			Retryable: retryable,
			Data:      data,
		},
	}
}

func NewErrorEnvelopeAutoRetry(id string, requestID string, code ErrorCode, message string) Envelope {
	return NewErrorEnvelope(id, requestID, code, message, DefaultRetryable(code))
}

func NewErrorEnvelopeWithDataAutoRetry(id string, requestID string, code ErrorCode, message string, data json.RawMessage) Envelope {
	return NewErrorEnvelopeWithData(id, requestID, code, message, DefaultRetryable(code), data)
}
