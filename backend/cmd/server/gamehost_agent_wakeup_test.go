package main

import (
	"testing"

	"github.com/u-ai/backend/internal/gamehost/agentbridge"
	"github.com/u-ai/backend/internal/gamehost/notification"
	"github.com/u-ai/backend/pkg/gameplugin/protocol"
)

func TestGameHostPluginEventRequestIDIsRouteAndGenerationScoped(t *testing.T) {
	base := notification.AgentWakeRequest{
		Scope: agentbridge.SessionScope{
			PluginID: "plugin-a", RuntimeID: "runtime-a", ServiceID: "service-a", Generation: 1,
		},
		Event: protocol.PluginEvent{ID: "1", SessionID: "player", Type: "vendor.event"},
	}
	id := gameHostPluginEventRequestID(base)
	if id == "" {
		t.Fatal("request id is empty")
	}
	if got := gameHostPluginEventRequestID(base); got != id {
		t.Fatalf("request id is not deterministic: %q != %q", got, id)
	}

	variants := []notification.AgentWakeRequest{base, base, base, base}
	variants[0].Scope.PluginID = "plugin-b"
	variants[1].Scope.ServiceID = "service-b"
	variants[2].Scope.Generation = 2
	variants[3].Event.ID = "2"
	for i, variant := range variants {
		if got := gameHostPluginEventRequestID(variant); got == id {
			t.Fatalf("variant %d collided with base request id", i)
		}
	}
}
