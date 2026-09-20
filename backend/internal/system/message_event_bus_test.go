package system

import (
	"testing"
	"time"
)

func TestPublishMessageCreatedCarriesSentStatus(t *testing.T) {
	bus := GetMessageEventBus()
	subscriber := bus.Subscribe("message-created-status-test", []string{"web"})
	defer bus.Unsubscribe(subscriber.ID)

	bus.PublishMessageCreated(
		"conversation-1",
		"message-1",
		"web",
		"outbound",
		"assistant",
		"sent",
		"回复",
		"2026-09-20 12:00:00",
		1,
		map[string]interface{}{"requestId": "request-1"},
	)

	select {
	case event := <-subscriber.Events:
		if event.Status != "sent" {
			t.Fatalf("expected sent status, got %q", event.Status)
		}
	case <-time.After(time.Second):
		t.Fatal("message created event was not published")
	}
}
