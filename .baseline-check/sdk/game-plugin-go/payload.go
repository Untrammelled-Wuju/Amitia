package sdk

import (
	"encoding/json"

	"github.com/u-ai/game-plugin-sdk-go/protocol"
)

func MarshalPayload(v any) (json.RawMessage, error) {
	if v == nil {
		return nil, nil
	}
	data, err := json.Marshal(v)
	if err != nil {
		return nil, NewEncodeError("failed to marshal payload: %v", err)
	}
	return data, nil
}

func DecodePayload[T any](message protocol.Envelope) (T, error) {
	var result T
	if message.Payload == nil || len(message.Payload) == 0 {
		return result, nil
	}
	if err := json.Unmarshal(message.Payload, &result); err != nil {
		return result, NewDecodeError("failed to decode payload: %v", err)
	}
	return result, nil
}

func EncodeRawPayload(data json.RawMessage) json.RawMessage {
	return data
}

func DecodeRawPayload(message protocol.Envelope) json.RawMessage {
	return message.Payload
}
