package sdk

import (
	"context"
	"encoding/json"
)

const (
	MethodServiceRegister   = "service.register"
	MethodServiceUnregister = "service.unregister"
)

type ServiceDescriptor struct {
	ServiceID    string   `json:"serviceId"`
	Capabilities []string `json:"capabilities,omitempty"`
}

type ServiceRegisterInput struct {
	ServiceID    string   `json:"serviceId"`
	Capabilities []string `json:"capabilities,omitempty"`
	Metadata     map[string]json.RawMessage `json:"metadata,omitempty"`
}

type ServiceUnregisterInput struct {
	ServiceID string `json:"serviceId"`
}

type ServiceRegisterOutput struct {
	ServiceID string `json:"serviceId"`
	Token     string `json:"token"`
}

func (c *Client) RegisterService(ctx context.Context, input ServiceRegisterInput, opts ...MessageOption) (ServiceRegisterOutput, error) {
	payload := map[string]any{
		"serviceId": input.ServiceID,
	}
	if input.Capabilities != nil {
		payload["capabilities"] = input.Capabilities
	}
	if input.Metadata != nil {
		payload["metadata"] = input.Metadata
	}

	envelope, err := c.sendHostRequest(ctx, MethodServiceRegister, payload, opts...)
	if err != nil {
		return ServiceRegisterOutput{}, err
	}
	var out ServiceRegisterOutput
	if len(envelope.Payload) > 0 {
		if err := json.Unmarshal(envelope.Payload, &out); err != nil {
			return ServiceRegisterOutput{}, NewEncodeError("unmarshal service register response failed: %v", err)
		}
	}
	return out, nil
}

func (c *Client) UnregisterService(ctx context.Context, input ServiceUnregisterInput, opts ...MessageOption) error {
	_, err := c.sendHostRequest(ctx, MethodServiceUnregister, input, opts...)
	return err
}
