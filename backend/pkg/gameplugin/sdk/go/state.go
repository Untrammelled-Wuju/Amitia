package sdk

import (
	"context"
	"encoding/json"

	"github.com/u-ai/backend/pkg/gameplugin/protocol"
)

const (
	MethodStatePublish = "plugin.state.publish"
	MethodStateGet     = "plugin.state.get"
)

type StatePublishInput struct {
	StateID  string          `json:"stateId"`
	Payload  json.RawMessage `json:"payload"`
	Version  int64           `json:"version"`
	Metadata map[string]json.RawMessage `json:"metadata,omitempty"`
}

type StatePublishOutput struct {
	Acked    bool   `json:"acked"`
	StateID  string `json:"stateId"`
}

type StateGetInput struct {
	StateID string `json:"stateId"`
}

type StateGetOutput struct {
	StateID string          `json:"stateId"`
	Payload json.RawMessage `json:"payload"`
	Version int64           `json:"version"`
}

func (c *Client) PublishState(ctx context.Context, input StatePublishInput, opts ...MessageOption) (protocol.Envelope, error) {
	payload := map[string]any{
		"stateId": input.StateID,
		"payload": input.Payload,
		"version": input.Version,
	}
	if input.Metadata != nil {
		payload["metadata"] = input.Metadata
	}
	return c.sendHostNotification(ctx, MethodStatePublish, payload, opts...)
}

func (c *Client) GetState(ctx context.Context, input StateGetInput, opts ...MessageOption) (StateGetOutput, error) {
	envelope, err := c.SendReservedRequest(ctx, MethodStateGet, input, opts...)
	if err != nil {
		return StateGetOutput{}, err
	}
	var out StateGetOutput
	if len(envelope.Payload) > 0 {
		if err := json.Unmarshal(envelope.Payload, &out); err != nil {
			return StateGetOutput{}, NewEncodeError("unmarshal state get response failed: %v", err)
		}
	}
	return out, nil
}
