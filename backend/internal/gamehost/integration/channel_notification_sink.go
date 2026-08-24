package integration

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/u-ai/backend/internal/gamehost/channel"
	"github.com/u-ai/backend/internal/gamehost/domain"
	"github.com/u-ai/backend/internal/gamehost/ipc"
	"github.com/u-ai/backend/internal/gamehost/notification"
)

const channelPublishMethod = "channel.publish"

type ChannelNotificationSink struct {
	router *channel.Router
}

func NewChannelNotificationSink(router *channel.Router) *ChannelNotificationSink {
	return &ChannelNotificationSink{router: router}
}

type channelPublishPayload struct {
	ChannelID string                     `json:"channelId"`
	Payload   json.RawMessage            `json:"payload"`
	Metadata  map[string]json.RawMessage `json:"metadata,omitempty"`
}

func (s *ChannelNotificationSink) Publish(ctx context.Context, n notification.Notification) error {
	if n.Method != channelPublishMethod {
		return nil
	}
	if s == nil || s.router == nil {
		return fmt.Errorf("channel notification sink: router is nil")
	}
	var payload channelPublishPayload
	if err := json.Unmarshal(n.Payload, &payload); err != nil {
		return fmt.Errorf("channel.publish: decode payload: %w", err)
	}
	if payload.ChannelID == "" {
		return domain.NewHostError(domain.ErrInvalidArgument, "channel.publish: channelId is required")
	}
	return s.router.Route(ctx, channel.IncomingChannelMessage{
		Peer: ipc.Peer{
			PluginID:  n.PluginID,
			RuntimeID: n.RuntimeID,
			ServiceID: n.ServiceID,
		},
		ChannelID: domain.ChannelID(payload.ChannelID),
		Payload:   payload.Payload,
		Metadata:  payload.Metadata,
	})
}
