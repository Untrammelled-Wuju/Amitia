package sdk

import (
	"context"
	"encoding/json"

	"github.com/u-ai/backend/pkg/gameplugin/protocol"
)

const (
	MethodChannelPublish = "channel.publish"
	MethodChannelSubscribe = "channel.subscribe"
	MethodChannelUnsubscribe = "channel.unsubscribe"
)

type ChannelPublishInput struct {
	ChannelID string          `json:"channelId"`
	Payload   json.RawMessage `json:"payload"`
	Metadata  map[string]json.RawMessage `json:"metadata,omitempty"`
}

type ChannelSubscribeInput struct {
	ChannelID string `json:"channelId"`
	Cursor    string `json:"cursor,omitempty"`
}

type ChannelSubscribeOutput struct {
	Cursor string `json:"cursor"`
}

type ChannelUnsubscribeInput struct {
	ChannelID string `json:"channelId"`
}

func (c *Client) ChannelPublish(ctx context.Context, input ChannelPublishInput, opts ...MessageOption) (protocol.Envelope, error) {
	payload := map[string]any{
		"channelId": input.ChannelID,
		"payload":   input.Payload,
	}
	if input.Metadata != nil {
		payload["metadata"] = input.Metadata
	}
	return c.sendHostNotification(ctx, MethodChannelPublish, payload, opts...)
}

func (c *Client) ChannelSubscribe(ctx context.Context, input ChannelSubscribeInput, opts ...MessageOption) (ChannelSubscribeOutput, error) {
	envelope, err := c.SendReservedRequest(ctx, MethodChannelSubscribe, input, opts...)
	if err != nil {
		return ChannelSubscribeOutput{}, err
	}
	var out ChannelSubscribeOutput
	if len(envelope.Payload) > 0 {
		if err := json.Unmarshal(envelope.Payload, &out); err != nil {
			return ChannelSubscribeOutput{}, NewEncodeError("unmarshal channel subscribe response failed: %v", err)
		}
	}
	return out, nil
}

func (c *Client) ChannelUnsubscribe(ctx context.Context, input ChannelUnsubscribeInput, opts ...MessageOption) error {
	_, err := c.SendReservedRequest(ctx, MethodChannelUnsubscribe, input, opts...)
	return err
}
