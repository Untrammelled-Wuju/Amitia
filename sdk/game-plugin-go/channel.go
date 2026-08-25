package sdk

import (
	"context"
	"encoding/json"

	"github.com/u-ai/game-plugin-sdk-go/protocol"
)

const MethodChannelPublish = "channel.publish"

type ChannelPublishInput struct {
	ChannelID string                     `json:"channelId"`
	Payload   json.RawMessage            `json:"payload"`
	Metadata  map[string]json.RawMessage `json:"metadata,omitempty"`
}

// ChannelPublish sends one message from the plugin service to a channel that
// was declared in the plugin host manifest and negotiated during hello.
// Host-to-plugin subscriptions are intentionally not part of host protocol v1.
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
