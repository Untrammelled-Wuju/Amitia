package sdk

import (
	"context"
	"encoding/json"

	"github.com/u-ai/backend/pkg/gameplugin/protocol"
)

const (
	MethodEventPublish = "plugin.event.publish"
)

type EventPublishInput struct {
	ChannelID string          `json:"channelId"`
	EventID   string          `json:"eventId"`
	Payload   json.RawMessage `json:"payload"`
	Metadata  map[string]json.RawMessage `json:"metadata,omitempty"`
}

func (c *Client) PublishEvent(ctx context.Context, input EventPublishInput, opts ...MessageOption) (protocol.Envelope, error) {
	payload := map[string]any{
		"channelId": input.ChannelID,
		"eventId":   input.EventID,
		"payload":   input.Payload,
	}
	if input.Metadata != nil {
		payload["metadata"] = input.Metadata
	}
	return c.SendNotification(ctx, MethodEventPublish, payload, opts...)
}
