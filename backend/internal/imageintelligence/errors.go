package imageintelligence

import (
	"encoding/json"
	"fmt"
	"net/http"
)

type ErrorCode string

const (
	ErrUnAvailable              ErrorCode = "IMAGE_INTELLIGENCE_UNAVAILABLE"
	ErrResourceNotFound         ErrorCode = "IMAGE_RESOURCE_NOT_FOUND"
	ErrResourceDenied           ErrorCode = "IMAGE_RESOURCE_DENIED"
	ErrFormatUnsupported        ErrorCode = "IMAGE_FORMAT_UNSUPPORTED"
	ErrInvalid                  ErrorCode = "IMAGE_INVALID"
	ErrTooLarge                 ErrorCode = "IMAGE_TOO_LARGE"
	ErrDimensionsTooLarge       ErrorCode = "IMAGE_DIMENSIONS_TOO_LARGE"
	ErrDecodeFailed             ErrorCode = "IMAGE_DECODE_FAILED"
	ErrProviderUnavailable      ErrorCode = "IMAGE_PROVIDER_UNAVAILABLE"
	ErrProviderAuth             ErrorCode = "IMAGE_PROVIDER_AUTH_FAILED"
	ErrProviderRateLimited      ErrorCode = "IMAGE_PROVIDER_RATE_LIMITED"
	ErrProviderInvalidResponse  ErrorCode = "IMAGE_PROVIDER_INVALID_RESPONSE"
	ErrUnderstandFailed         ErrorCode = "IMAGE_UNDERSTAND_FAILED"
	ErrOCRUnavailable           ErrorCode = "OCR_UNAVAILABLE"
	ErrOCRFailed                ErrorCode = "OCR_FAILED"
	ErrOCRInvalidResponse       ErrorCode = "OCR_INVALID_RESPONSE"
	ErrGenUnavailable           ErrorCode = "IMAGE_GENERATION_UNAVAILABLE"
	ErrGenFailed                ErrorCode = "IMAGE_GENERATION_FAILED"
	ErrGenInvalidResponse       ErrorCode = "IMAGE_GENERATION_INVALID_RESPONSE"
	ErrGenOutputInvalid         ErrorCode = "IMAGE_GENERATION_OUTPUT_INVALID"
	ErrOperationCancelled       ErrorCode = "IMAGE_OPERATION_CANCELLED"
	ErrOperationTimeout         ErrorCode = "IMAGE_OPERATION_TIMEOUT"
)

type Error struct {
	Code       ErrorCode `json:"code"`
	Message    string    `json:"message"`
	Provider   string    `json:"provider,omitempty"`
	Retryable  bool      `json:"retryable"`
	HTTPStatus int       `json:"-"`
}

func (e *Error) Error() string {
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

func mapRetryClass(class string) bool {
	switch class {
	case "transport", "infrastructure":
		return true
	}
	return false
}

func mapProviderErrorToDomain(provider string, retryClass string, msg string) *Error {
	switch retryClass {
	case "auth":
		return &Error{Code: ErrProviderAuth, Message: msg, Provider: provider, Retryable: false, HTTPStatus: http.StatusUnauthorized}
	case "rate_limit":
		return &Error{Code: ErrProviderRateLimited, Message: msg, Provider: provider, Retryable: false, HTTPStatus: http.StatusTooManyRequests}
	case "invalid_response":
		return &Error{Code: ErrProviderInvalidResponse, Message: msg, Provider: provider, Retryable: false, HTTPStatus: http.StatusBadGateway}
	}
	return &Error{Code: ErrProviderUnavailable, Message: msg, Provider: provider, Retryable: mapRetryClass(retryClass), HTTPStatus: http.StatusServiceUnavailable}
}

func mapImageErrorToDomain(msg string, retryable bool) *Error {
	return &Error{Code: ErrUnderstandFailed, Message: msg, Retryable: retryable, HTTPStatus: http.StatusInternalServerError}
}

func marshalErrorResponse(err *Error) json.RawMessage {
	data, _ := json.Marshal(map[string]interface{}{
		"error": map[string]interface{}{
			"code":      err.Code,
			"message":   err.Message,
			"provider":  err.Provider,
			"retryable": err.Retryable,
		},
	})
	return data
}
