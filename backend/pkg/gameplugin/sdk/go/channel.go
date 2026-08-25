package sdk

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/u-ai/backend/pkg/gameplugin/protocol"
)

const (
	MethodChannelPublish = "channel.publish"
	MethodChannelDeliver = "channel.deliver"
)

type ChannelPublishInput struct {
	ChannelID string                     `json:"channelId"`
	Payload   json.RawMessage            `json:"payload"`
	Metadata  map[string]json.RawMessage `json:"metadata,omitempty"`
}

// ChannelPublish sends one message from the plugin service to a channel that
// was declared in the plugin host manifest and negotiated during hello.
// Host-to-plugin delivery is available on channels declared host_to_plugin or bidirectional.
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

// ChannelDelivery is emitted by GameHost for host_to_plugin and bidirectional channels.
type ChannelDelivery struct {
	ChannelID string                     `json:"channelId"`
	Payload   json.RawMessage            `json:"payload"`
	Metadata  map[string]json.RawMessage `json:"metadata,omitempty"`
}

type ChannelDeliveryHandler func(context.Context, ChannelDelivery) error

// RegisterChannelDeliveryHandler binds the reserved channel.deliver notification
// to a typed plugin callback. The runner routes the notification to the service
// registry selected by envelope.serviceId.
func RegisterChannelDeliveryHandler(registry *HandlerRegistry, handler ChannelDeliveryHandler) {
	if registry == nil || handler == nil {
		return
	}
	registry.RegisterNotification(MethodChannelDeliver, func(ctx context.Context, notification protocol.Envelope) error {
		var delivery ChannelDelivery
		if err := json.Unmarshal(notification.Payload, &delivery); err != nil {
			return err
		}
		if delivery.ChannelID == "" {
			return fmt.Errorf("channel delivery missing channelId")
		}
		return handler(ctx, delivery)
	})
}
