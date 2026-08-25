package sdk

import (
	"context"
	"encoding/json"

	"github.com/u-ai/game-plugin-sdk-go/protocol"
)

const MethodAgentEventPublish = "plugin.event.publish"
const PluginAgentEventID = "plugin.agent_event"

// PublishAgentEvent publishes a plugin-defined event as an Agent wake-up hint.
// This reserved host method is deliberately not a generic event bus. Generic
// plugin event/state traffic belongs on channels declared in the GameHost
// manifest and must use ChannelPublish.
func (c *Client) PublishAgentEvent(ctx context.Context, event protocol.PluginEvent, metadata map[string]json.RawMessage, opts ...MessageOption) (protocol.Envelope, error) {
	payload, err := json.Marshal(event)
	if err != nil {
		return protocol.Envelope{}, err
	}
	envelope := map[string]any{
		"eventId": PluginAgentEventID,
		"payload": json.RawMessage(payload),
	}
	if metadata != nil {
		envelope["metadata"] = metadata
	}
	return c.sendHostNotification(ctx, MethodAgentEventPublish, envelope, opts...)
}
