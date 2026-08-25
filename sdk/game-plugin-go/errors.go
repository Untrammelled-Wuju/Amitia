package sdk

import "fmt"

type SDKErrorCode string

const (
	ErrorProtocol   SDKErrorCode = "protocol_error"
	ErrorTransport  SDKErrorCode = "transport_error"
	ErrorEncode     SDKErrorCode = "encode_error"
	ErrorDecode     SDKErrorCode = "decode_error"
	ErrorValidation SDKErrorCode = "validation_error"
)

type SDKError struct {
	Code    SDKErrorCode
	Message string
	Cause   error
}

func (e SDKError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("sdk.%s: %s: %v", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("sdk.%s: %s", e.Code, e.Message)
}

func (e SDKError) Unwrap() error {
	return e.Cause
}

func NewProtocolError(msg string, args ...interface{}) SDKError {
	return SDKError{
		Code:    ErrorProtocol,
		Message: fmt.Sprint(fmt.Sprintf(msg, args...)),
	}
}

func NewTransportError(msg string, args ...interface{}) SDKError {
	return SDKError{
		Code:    ErrorTransport,
		Message: fmt.Sprint(fmt.Sprintf(msg, args...)),
	}
}

func NewEncodeError(msg string, args ...interface{}) SDKError {
	return SDKError{
		Code:    ErrorEncode,
		Message: fmt.Sprint(fmt.Sprintf(msg, args...)),
	}
}

func NewDecodeError(msg string, args ...interface{}) SDKError {
	return SDKError{
		Code:    ErrorDecode,
		Message: fmt.Sprint(fmt.Sprintf(msg, args...)),
	}
}

func NewValidationError(msg string, args ...interface{}) SDKError {
	return SDKError{
		Code:    ErrorValidation,
		Message: fmt.Sprint(fmt.Sprintf(msg, args...)),
	}
}

func NewProtocolErrorWithCause(code SDKErrorCode, msg string, cause error) SDKError {
	return SDKError{
		Code:    code,
		Message: msg,
		Cause:   cause,
	}
}
