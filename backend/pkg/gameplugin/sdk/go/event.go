package sdk

import (
	"context"
	"encoding/json"

	"github.com/u-ai/backend/pkg/gameplugin/protocol"
)

const MethodEventPublish = "plugin.event.publish"
const PluginAgentEventID = "plugin.agent_event"

type EventPublishInput struct {
	ChannelID string                     `json:"channelId"`
	EventID   string                     `json:"eventId"`
	Payload   json.RawMessage            `json:"payload"`
	Metadata  map[string]json.RawMessage `json:"metadata,omitempty"`
}

func (c *Client) PublishEvent(ctx context.Context, input EventPublishInput, opts ...MessageOption) (protocol.Envelope, error) {
	payload := map[string]any{"channelId": input.ChannelID, "eventId": input.EventID, "payload": input.Payload}
	if input.Metadata != nil {
		payload["metadata"] = input.Metadata
	}
	return c.sendHostNotification(ctx, MethodEventPublish, payload, opts...)
}

// PublishAgentEvent publishes a generic plugin-defined event that may wake the
// bound Agent context. GameHost treats Type and Payload as opaque data.
func (c *Client) PublishAgentEvent(ctx context.Context, event protocol.PluginEvent, metadata map[string]json.RawMessage, opts ...MessageOption) (protocol.Envelope, error) {
	payload, err := json.Marshal(event)
	if err != nil {
		return protocol.Envelope{}, err
	}
	return c.PublishEvent(ctx, EventPublishInput{ChannelID: event.SessionID, EventID: PluginAgentEventID, Payload: payload, Metadata: metadata}, opts...)
}
