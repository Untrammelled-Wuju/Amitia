package integration

import (
	"testing"

	"github.com/u-ai/backend/internal/extension/kernel/event"
	"github.com/u-ai/backend/internal/gamehost/notification"
)

func TestGameHostNotificationPublishOptionsUseTrustedSystemProducer(t *testing.T) {
	opts := gameHostNotificationPublishOptions("com.example/game", notification.Notification{
		ID:        "trace-1",
		PluginID:  "plugin-1",
		RuntimeID: "runtime-1",
		ServiceID: "service-1",
		Method:    "plugin.event.updated",
	})
	if opts.ProducerID != "com.example/game" ||
		opts.ProducerType != event.EventProducerTypeSystem ||
		opts.ProducerExtensionID != "com.example/game" ||
		opts.TraceID != "trace-1" {
		t.Fatalf("unexpected trusted producer options: %+v", opts)
	}
}
