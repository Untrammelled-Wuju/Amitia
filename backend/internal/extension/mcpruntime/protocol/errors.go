package protocol

import (
	"encoding/json"
	"fmt"
)

const (
	ErrorParse          = -32700
	ErrorInvalidRequest = -32600
	ErrorMethodNotFound = -32601
	ErrorInvalidParams  = -32602
	ErrorInternal       = -32603
	ErrorRequestTimeout = -32001
	ErrorRequestCancel  = -32800
)

type RPCError struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

func (e *RPCError) Error() string {
	if e == nil {
		return ""
	}
	return fmt.Sprintf("MCP JSON-RPC error %d: %s", e.Code, e.Message)
}

func NewError(code int, message string, data any) *RPCError {
	result := &RPCError{Code: code, Message: message}
	if data != nil {
		result.Data, _ = json.Marshal(data)
	}
	return result
}

var (
	ErrInvalidMessage     = fmt.Errorf("MCP_PROTOCOL_INVALID_MESSAGE")
	ErrMessageTooLarge    = fmt.Errorf("MCP_PROTOCOL_MESSAGE_TOO_LARGE")
	ErrUnsupportedVersion = fmt.Errorf("MCP_PROTOCOL_VERSION_UNSUPPORTED")
	ErrUnknownResponse    = fmt.Errorf("MCP_PROTOCOL_UNKNOWN_RESPONSE")
	ErrDuplicateRequestID = fmt.Errorf("MCP_PROTOCOL_DUPLICATE_ID")
	ErrInitialization     = fmt.Errorf("MCP_PROTOCOL_INITIALIZATION_FAILED")
	ErrTransportClosed    = fmt.Errorf("MCP_TRANSPORT_CLOSED")
	ErrRequestTimeout     = fmt.Errorf("MCP_PROTOCOL_REQUEST_TIMEOUT")
	ErrRequestCancelled   = fmt.Errorf("MCP_PROTOCOL_REQUEST_CANCELLED")
)
