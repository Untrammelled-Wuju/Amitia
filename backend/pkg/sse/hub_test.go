package sse

import "testing"

func TestScheduledNotificationTargetsAuthenticatedDevice(t *testing.T) {
	hub := &Hub{clients: map[string]*Client{}}
	current := hub.SubscribeDevice("one", "space", "device")
	otherDevice := hub.SubscribeDevice("two", "space", "other")
	otherSpace := hub.SubscribeDevice("three", "other", "device")
	anonymous := hub.SubscribeScoped("four", "space")
	if hub.SendToDevice("space", "device", "scheduled_notification", map[string]interface{}{"requestId": "notification"}) != 1 {
		t.Fatal("notification not scoped to exactly one device")
	}
	select {
	case event := <-current.Events:
		if event["event"] != "scheduled_notification" {
			t.Fatal(event)
		}
	default:
		t.Fatal("target missed notification")
	}
	for _, client := range []*Client{otherDevice, otherSpace, anonymous} {
		select {
		case event := <-client.Events:
			t.Fatal("notification leaked", client.ID, event)
		default:
		}
	}
	if hub.SendToDevice("", "device", "scheduled_notification", nil) != 0 || hub.SendToDevice("space", "", "scheduled_notification", nil) != 0 {
		t.Fatal("empty identity accepted")
	}
}

func TestDeviceSubscriptionPreservesHostRegistryAddressAndAvoidsCrossDeviceReplacement(t *testing.T) {
	hub := &Hub{clients: map[string]*Client{}}
	first := hub.SubscribeDevice("ui-host", "space", "first")
	if !hub.SendToClient("ui-host", "ui_notify", nil) {
		t.Fatal("host registry address no longer resolves")
	}
	<-first.Events
	second := hub.SubscribeDevice("ui-host", "space", "second")
	if hub.SendToClient("ui-host", "ui_notify", nil) {
		t.Fatal("ambiguous host identity must fail closed")
	}
	if hub.SendToDevice("space", "first", "scheduled_notification", nil) != 1 || hub.SendToDevice("space", "second", "scheduled_notification", nil) != 1 {
		t.Fatal("one device replaced another device subscription")
	}
	if _, ok := <-first.Events; !ok {
		t.Fatal("first device disconnected")
	}
	if _, ok := <-second.Events; !ok {
		t.Fatal("second device disconnected")
	}
	hub.UnsubscribeClient(second)
	if !hub.SendToClient("ui-host", "ui_notify", nil) {
		t.Fatal("host address not restored")
	}
}
