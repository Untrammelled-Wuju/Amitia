package rpc

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/u-ai/backend/internal/gamehost/ipc"
	"github.com/u-ai/backend/internal/gamehost/notification"
	"github.com/u-ai/backend/pkg/gameplugin/protocol"
)

type captureNotificationBridge struct {
	route  notification.RouteContext
	method string
}

func (c *captureNotificationBridge) Handle(_ context.Context, route notification.RouteContext, method string, _ json.RawMessage, _ map[string]json.RawMessage) error {
	c.route = route
	c.method = method
	return nil
}

func TestRPCDispatcherNotificationPreservesTrustedPeerGeneration(t *testing.T) {
	bridge := &captureNotificationBridge{}
	dispatcher := NewRPCDispatcher(DispatcherConfig{Notifications: bridge})
	err := dispatcher.Dispatch(context.Background(), ipc.DispatchSource{
		ConnectionID: "connection-1",
		Peer: ipc.Peer{
			PluginID:   "plugin-1",
			RuntimeID:  "runtime-1",
			ServiceID:  "service-1",
			Generation: 7,
		},
	}, protocol.Envelope{
		Protocol: protocol.ProtocolVersion,
		Type:     protocol.MessageTypeNotification,
		ID:       "notification-1",
		Method:   "channel.publish",
		Payload:  json.RawMessage(`{"channelId":"events","payload":{}}`),
	})
	if err != nil {
		t.Fatalf("dispatch notification: %v", err)
	}
	if bridge.method != "channel.publish" || bridge.route.PluginID != "plugin-1" || bridge.route.RuntimeID != "runtime-1" || bridge.route.ServiceID != "service-1" || bridge.route.Generation != 7 {
		t.Fatalf("trusted notification route was not preserved: method=%q route=%+v", bridge.method, bridge.route)
	}
}
