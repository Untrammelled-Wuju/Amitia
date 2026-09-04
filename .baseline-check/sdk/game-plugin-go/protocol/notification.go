package protocol

import (
	"encoding/json"
	"fmt"
)

func NewNotification(id string, method string, payload any) (Envelope, error) {
	if err := ValidateMessageID(id); err != nil {
		return Envelope{}, err
	}
	if err := ValidateMethod(method); err != nil {
		return Envelope{}, err
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
		Protocol: ProtocolVersion,
		Type:     MessageTypeNotification,
		ID:       id,
		Method:   method,
		Payload:  rawPayload,
	}, nil
}

func NewNotificationWithRoute(id string, method string, payload any, runtimeID, pluginID, serviceID string) (Envelope, error) {
	env, err := NewNotification(id, method, payload)
	if err != nil {
		return Envelope{}, err
	}
	env.RuntimeID = runtimeID
	env.PluginID = pluginID
	env.ServiceID = serviceID
	return env, nil
}
