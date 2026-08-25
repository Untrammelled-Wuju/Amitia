package rpc

import (
	"encoding/json"

	"github.com/u-ai/backend/internal/gamehost/domain"
	"github.com/u-ai/backend/pkg/gameplugin/protocol"
)

type RPCRequest struct {
	ConnectionID   string
	ID             string
	PluginID       domain.PluginID
	RuntimeID      domain.RuntimeInstanceID
	ServiceID      domain.ServiceID
	Generation     int64
	Namespace      Namespace
	Method         Method
	Payload        json.RawMessage
	Metadata       map[string]json.RawMessage
	BinaryObjectID string
	BinaryOffset   int64
	BinaryPayload  []byte
}

type RPCResponse struct {
	RequestID string
	Payload   json.RawMessage
	Error     *RPCRoutedError
}

type RPCRoutedError struct {
	Code      string          `json:"code"`
	Message   string          `json:"message"`
	Retryable bool            `json:"retryable,omitempty"`
	Data      json.RawMessage `json:"data,omitempty"`
}

func (r RPCResponse) ToProtocolEnvelope() protocol.Envelope {
	env := protocol.Envelope{
		Protocol:  protocol.ProtocolVersion,
		Type:      protocol.MessageTypeResponse,
		ID:        r.RequestID,
		RequestID: r.RequestID,
	}
	if r.Payload != nil {
		env.Payload = r.Payload
	}
	if r.Error != nil {
		env.Error = &protocol.ProtocolError{
			Code:      protocol.ErrorCode(r.Error.Code),
			Message:   r.Error.Message,
			Retryable: r.Error.Retryable,
			Data:      r.Error.Data,
		}
		env.Type = protocol.MessageTypeError
	}
	return env
}
