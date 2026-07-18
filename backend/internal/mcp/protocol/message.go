package protocol

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

const JSONRPCVersion = "2.0"

type MessageKind string

const (
	MessageRequest      MessageKind = "request"
	MessageResponse     MessageKind = "response"
	MessageError        MessageKind = "error"
	MessageNotification MessageKind = "notification"
)

type Message struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *RPCError       `json:"error,omitempty"`
}

func Request(id any, method string, params any) (Message, error) {
	rawID, err := MarshalID(id)
	if err != nil {
		return Message{}, err
	}
	rawParams, err := marshalOptional(params)
	if err != nil {
		return Message{}, err
	}
	message := Message{JSONRPC: JSONRPCVersion, ID: rawID, Method: method, Params: rawParams}
	return message, message.Validate()
}

func Notification(method string, params any) (Message, error) {
	rawParams, err := marshalOptional(params)
	if err != nil {
		return Message{}, err
	}
	message := Message{JSONRPC: JSONRPCVersion, Method: method, Params: rawParams}
	return message, message.Validate()
}

func Response(id json.RawMessage, result any) (Message, error) {
	rawResult, err := json.Marshal(result)
	if err != nil {
		return Message{}, err
	}
	message := Message{JSONRPC: JSONRPCVersion, ID: cloneRaw(id), Result: rawResult}
	return message, message.Validate()
}

func ErrorResponse(id json.RawMessage, rpcError *RPCError) (Message, error) {
	message := Message{JSONRPC: JSONRPCVersion, ID: cloneRaw(id), Error: rpcError}
	return message, message.Validate()
}

func (m Message) Kind() (MessageKind, error) {
	if err := m.Validate(); err != nil {
		return "", err
	}
	if m.Method != "" {
		if len(m.ID) == 0 {
			return MessageNotification, nil
		}
		return MessageRequest, nil
	}
	if m.Error != nil {
		return MessageError, nil
	}
	return MessageResponse, nil
}

func (m Message) Validate() error {
	if m.JSONRPC != JSONRPCVersion {
		return fmt.Errorf("%w: jsonrpc must be 2.0", ErrInvalidMessage)
	}
	if m.Method != "" {
		if len(m.Result) != 0 || m.Error != nil {
			return fmt.Errorf("%w: request cannot contain result or error", ErrInvalidMessage)
		}
		if len(m.ID) != 0 {
			if _, err := CanonicalID(m.ID, false); err != nil {
				return err
			}
		}
		return nil
	}
	if len(m.ID) == 0 {
		return fmt.Errorf("%w: response id is required", ErrInvalidMessage)
	}
	if _, err := CanonicalID(m.ID, true); err != nil {
		return err
	}
	if (len(m.Result) == 0) == (m.Error == nil) {
		return fmt.Errorf("%w: response must contain exactly one of result or error", ErrInvalidMessage)
	}
	return nil
}

func Decode(data []byte, maxBytes int64) (Message, error) {
	if maxBytes > 0 && int64(len(data)) > maxBytes {
		return Message{}, ErrMessageTooLarge
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	var message Message
	if err := decoder.Decode(&message); err != nil {
		return Message{}, fmt.Errorf("%w: %v", ErrInvalidMessage, err)
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return Message{}, fmt.Errorf("%w: trailing JSON data", ErrInvalidMessage)
	}
	if err := message.Validate(); err != nil {
		return Message{}, err
	}
	return message, nil
}

func Encode(message Message, maxBytes int64) ([]byte, error) {
	if err := message.Validate(); err != nil {
		return nil, err
	}
	data, err := json.Marshal(message)
	if err != nil {
		return nil, err
	}
	if maxBytes > 0 && int64(len(data)) > maxBytes {
		return nil, ErrMessageTooLarge
	}
	return data, nil
}

func MarshalID(value any) (json.RawMessage, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	if _, err := CanonicalID(data, false); err != nil {
		return nil, err
	}
	return data, nil
}

func CanonicalID(raw json.RawMessage, allowNull bool) (string, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return "", fmt.Errorf("%w: invalid id", ErrInvalidMessage)
	}
	switch typed := value.(type) {
	case string:
		return "s:" + typed, nil
	case json.Number:
		return "n:" + typed.String(), nil
	case nil:
		if allowNull {
			return "null", nil
		}
	}
	return "", fmt.Errorf("%w: id must be a string or number", ErrInvalidMessage)
}

func marshalOptional(value any) (json.RawMessage, error) {
	if value == nil {
		return nil, nil
	}
	return json.Marshal(value)
}

func cloneRaw(value json.RawMessage) json.RawMessage {
	return append(json.RawMessage(nil), value...)
}
