package integration

import (
	"context"
	"fmt"

	"github.com/u-ai/backend/internal/gamehost/notification"
)

// ChannelValidatedSink makes channel.publish a validation gate for downstream
// durable/bridge sinks. Invalid channel payloads (including stale/forged binary
// references) must never be persisted or fanned out before GameHost has checked
// the authenticated route, permission, generation, declared channel and binary
// object ownership/readiness.
//
// Non-channel notifications are forwarded unchanged.
type ChannelValidatedSink struct {
	channels   *ChannelNotificationSink
	downstream notification.NotificationSink
}

func NewChannelValidatedSink(channels *ChannelNotificationSink, downstream notification.NotificationSink) *ChannelValidatedSink {
	return &ChannelValidatedSink{channels: channels, downstream: downstream}
}

func (s *ChannelValidatedSink) Publish(ctx context.Context, n notification.Notification) error {
	if s == nil {
		return fmt.Errorf("channel validated sink: nil receiver")
	}
	if n.Method == channelPublishMethod {
		if s.channels == nil {
			return fmt.Errorf("channel validated sink: channel validator unavailable")
		}
		canonical, err := s.channels.ValidateAndCanonicalize(ctx, n)
		if err != nil {
			return err
		}
		n = canonical
	}
	if s.downstream == nil {
		return nil
	}
	return s.downstream.Publish(ctx, n)
}
