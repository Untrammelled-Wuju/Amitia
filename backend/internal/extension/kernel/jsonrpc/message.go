package jsonrpc

import (
	"encoding/json"
	"errors"
	"fmt"
)

const ProtocolVersion = "2.0"
const RPCVersion = "amitia-runtime-rpc/1"

type MessageKind string

const (
	KindRequest      MessageKind = "request"
	KindResponse     MessageKind = "response"
	KindNotification MessageKind = "notification"
	KindError        MessageKind = "error"
)

type RequestID struct {
	value any
	isSet bool
}

func NewStringID(id string) RequestID {
	return RequestID{value: id, isSet: true}
}

func NewNumberID(id int64) RequestID {
	return RequestID{value: id, isSet: true}
}

func NullID() RequestID {
	return RequestID{value: nil, isSet: true}
}

func EmptyID() RequestID {
	return RequestID{isSet: false}
}

func (r RequestID) IsSet() bool   { return r.isSet }
func (r RequestID) IsNull() bool  { return r.isSet && r.value == nil }
func (r RequestID) String() string {
	if !r.isSet {
		return ""
	}
	if r.value == nil {
		return "null"
	}
	return fmt.Sprintf("%v", r.value)
}

func (r RequestID) MarshalJSON() ([]byte, error) {
	if !r.isSet {
		return []byte("null"), nil
	}
	return json.Marshal(r.value)
}

func (r *RequestID) UnmarshalJSON(data []byte) error {
	r.isSet = true
	if string(data) == "null" {
		r.value = nil
		return nil
	}
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		r.value = s
		return nil
	}
	var n json.Number
	if err := json.Unmarshal(data, &n); err == nil {
		r.value = n
		return nil
	}
	return errors.New("jsonrpc: invalid request id")
}

type Request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      RequestID       `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type Response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      RequestID       `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *Error          `json:"error,omitempty"`
}

type Notification struct {
	JSONRPC string          `json:"jsonrpc"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type Error struct {
	Code    ErrorCode       `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

func (e *Error) Error() string {
	return fmt.Sprintf("jsonrpc: %s: %s", e.Code, e.Message)
}

type ErrorCode string

const (
	ErrCodeParseError          ErrorCode = "parse_error"
	ErrCodeInvalidRequest      ErrorCode = "invalid_request"
	ErrCodeMethodNotFound      ErrorCode = "method_not_found"
	ErrCodeInvalidParams       ErrorCode = "invalid_params"
	ErrCodeInternal            ErrorCode = "internal"
	ErrCodePermissionDenied    ErrorCode = "permission_denied"
	ErrCodeResourceExhausted   ErrorCode = "resource_exhausted"
	ErrCodeTimeout             ErrorCode = "timeout"
	ErrCodeCancelled           ErrorCode = "cancelled"
	ErrCodeProtocol            ErrorCode = "protocol"
	ErrCodeHandshakeFailed     ErrorCode = "handshake_failed"
	ErrCodeVersionMismatch     ErrorCode = "version_mismatch"
	ErrCodeStreamClosed        ErrorCode = "stream_closed"
	ErrCodeStreamBackpressure  ErrorCode = "stream_backpressure"
	ErrCodeFrameTooLarge       ErrorCode = "frame_too_large"
	ErrCodeSessionExpired      ErrorCode = "session_expired"
	ErrCodeUnauthorized        ErrorCode = "unauthorized"
	ErrCodeRequestNotFound     ErrorCode = "request_not_found"
)

type ErrorCategory string

const (
	CategoryPermission ErrorCategory = "permission"
	CategoryProtocol   ErrorCategory = "protocol"
	CategoryResource   ErrorCategory = "resource"
	CategoryRuntime    ErrorCategory = "runtime"
	CategoryTransient  ErrorCategory = "transient"
	CategoryStream     ErrorCategory = "stream"
)

type ErrorData struct {
	Retryable bool           `json:"retryable"`
	Category  ErrorCategory  `json:"category"`
	Detail    map[string]any `json:"detail,omitempty"`
}

func NewError(code ErrorCode, message string, retryable bool, category ErrorCategory) *Error {
	data := ErrorData{Retryable: retryable, Category: category}
	raw, _ := json.Marshal(data)
	return &Error{Code: code, Message: message, Data: raw}
}

func NewErrorWithData(code ErrorCode, message string, data ErrorData) *Error {
	raw, _ := json.Marshal(data)
	return &Error{Code: code, Message: message, Data: raw}
}

func ParseError(message string) *Error {
	return NewError(ErrCodeParseError, message, false, CategoryProtocol)
}

func MethodNotFoundError(method string) *Error {
	return NewError(ErrCodeMethodNotFound, fmt.Sprintf("method not found: %s", method), false, CategoryProtocol)
}

func InvalidParamsError(message string) *Error {
	return NewError(ErrCodeInvalidParams, message, false, CategoryProtocol)
}

func InternalError(message string) *Error {
	return NewError(ErrCodeInternal, message, true, CategoryRuntime)
}

func PermissionDeniedError(message string) *Error {
	return NewError(ErrCodePermissionDenied, message, false, CategoryPermission)
}

func TimeoutError(message string) *Error {
	return NewError(ErrCodeTimeout, message, false, CategoryTransient)
}

func CancelledError(message string) *Error {
	return NewError(ErrCodeCancelled, message, false, CategoryRuntime)
}

func HandshakeFailedError(message string) *Error {
	return NewError(ErrCodeHandshakeFailed, message, false, CategoryProtocol)
}

func FrameTooLargeError(actual, limit int) *Error {
	return NewError(
		ErrCodeFrameTooLarge,
		fmt.Sprintf("frame size %d exceeds limit %d", actual, limit),
		false,
		CategoryProtocol,
	)
}

type Envelope struct {
	Kind    MessageKind
	Request *Request
	Response *Response
	Notification *Notification
}

func EncodeRequest(id RequestID, method string, params any) (*Request, error) {
	var raw json.RawMessage
	if params != nil {
		b, err := json.Marshal(params)
		if err != nil {
			return nil, fmt.Errorf("jsonrpc: marshal params: %w", err)
		}
		raw = b
	}
	return &Request{JSONRPC: ProtocolVersion, ID: id, Method: method, Params: raw}, nil
}

func EncodeResponse(id RequestID, result any) (*Response, error) {
	var raw json.RawMessage
	if result != nil {
		b, err := json.Marshal(result)
		if err != nil {
			return nil, fmt.Errorf("jsonrpc: marshal result: %w", err)
		}
		raw = b
	}
	return &Response{JSONRPC: ProtocolVersion, ID: id, Result: raw}, nil
}

func EncodeErrorResponse(id RequestID, err *Error) *Response {
	return &Response{JSONRPC: ProtocolVersion, ID: id, Error: err}
}

func EncodeNotification(method string, params any) (*Notification, error) {
	var raw json.RawMessage
	if params != nil {
		b, err := json.Marshal(params)
		if err != nil {
			return nil, fmt.Errorf("jsonrpc: marshal params: %w", err)
		}
		raw = b
	}
	return &Notification{JSONRPC: ProtocolVersion, Method: method, Params: raw}, nil
}

func DecodeEnvelope(data []byte) (*Envelope, error) {
	if len(data) == 0 {
		return nil, errors.New("jsonrpc: empty payload")
	}
	var probe struct {
		JSONRPC string          `json:"jsonrpc"`
		ID      *json.RawMessage `json:"id"`
		Method  string          `json:"method"`
		Result  json.RawMessage `json:"result,omitempty"`
		Error   json.RawMessage `json:"error,omitempty"`
		Params  json.RawMessage `json:"params,omitempty"`
	}
	if err := json.Unmarshal(data, &probe); err != nil {
		return nil, fmt.Errorf("jsonrpc: decode: %w", err)
	}
	if probe.JSONRPC != ProtocolVersion {
		return nil, fmt.Errorf("jsonrpc: unsupported version %q", probe.JSONRPC)
	}
	if probe.Method != "" && probe.ID != nil {
		var req Request
		if err := json.Unmarshal(data, &req); err != nil {
			return nil, fmt.Errorf("jsonrpc: decode request: %w", err)
		}
		return &Envelope{Kind: KindRequest, Request: &req}, nil
	}
	if probe.Method != "" && probe.ID == nil {
		var n Notification
		if err := json.Unmarshal(data, &n); err != nil {
			return nil, fmt.Errorf("jsonrpc: decode notification: %w", err)
		}
		return &Envelope{Kind: KindNotification, Notification: &n}, nil
	}
	if probe.ID != nil {
		var resp Response
		if err := json.Unmarshal(data, &resp); err != nil {
			return nil, fmt.Errorf("jsonrpc: decode response: %w", err)
		}
		kind := KindResponse
		if resp.Error != nil {
			kind = KindError
		}
		return &Envelope{Kind: kind, Response: &resp}, nil
	}
	return nil, errors.New("jsonrpc: cannot determine message kind")
}

func MarshalMessage(msg any) ([]byte, error) {
	switch v := msg.(type) {
	case *Request:
		return json.Marshal(v)
	case *Response:
		return json.Marshal(v)
	case *Notification:
		return json.Marshal(v)
	default:
		return nil, fmt.Errorf("jsonrpc: unsupported message type %T", msg)
	}
}
