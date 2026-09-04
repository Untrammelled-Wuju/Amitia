package protocol

import (
	"encoding/json"
	"fmt"
)

func NewResponse(id string, requestID string, payload any) (Envelope, error) {
	if err := ValidateMessageID(id); err != nil {
		return Envelope{}, err
	}
	if requestID == "" {
		return Envelope{}, fmt.Errorf("requestID must not be empty")
	}

	var rawPayload json.RawMessage
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return Envelope{}, fmt.Errorf("failed to marshal payload: %w", err)
		}
		rawPayload = data
	}

	return Envelope{
		Protocol:  ProtocolVersion,
		Type:      MessageTypeResponse,
		ID:        id,
		RequestID: requestID,
		Payload:   rawPayload,
	}, nil
}

func NewResponseWithMetadata(id string, requestID string, payload any, metadata map[string]json.RawMessage) (Envelope, error) {
	env, err := NewResponse(id, requestID, payload)
	if err != nil {
		return Envelope{}, err
	}
	env.Metadata = metadata
	return env, nil
}
