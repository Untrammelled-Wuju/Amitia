package sdk

import (
	"strconv"
	"testing"

	"github.com/u-ai/game-plugin-sdk-go/protocol"
)

type generationTestIDGenerator struct{ next int }

func (g *generationTestIDGenerator) NewID() string {
	g.next++
	return "msg-" + strconv.Itoa(g.next)
}

func TestAdoptPeerRoutingBindsEveryEnvelopeType(t *testing.T) {
	client := NewClient(nil, WithIDGenerator(&generationTestIDGenerator{}))
	if err := client.AdoptPeerRouting(protocol.Envelope{
		Protocol:   protocol.ProtocolVersion,
		Type:       protocol.MessageTypeResponse,
		ID:         "hello-response",
		RequestID:  "hello-request",
		RuntimeID:  "rt-1",
		PluginID:   "extension/plugin",
		ServiceID:  "svc-1",
		Generation: 7,
	}); err != nil {
		t.Fatalf("AdoptPeerRouting failed: %v", err)
	}

	request, err := client.NewRequest("example.game.operation.submit", nil)
	if err != nil {
		t.Fatalf("NewRequest failed: %v", err)
	}
	response, err := client.NewResponse(request, map[string]bool{"ok": true})
	if err != nil {
		t.Fatalf("NewResponse failed: %v", err)
	}
	notification, err := client.NewNotification("vendor.event.changed", nil)
	if err != nil {
		t.Fatalf("NewNotification failed: %v", err)
	}
	protocolError, err := client.NewError(request, protocol.ErrorCode("permission_denied"), "denied", false, nil)
	if err != nil {
		t.Fatalf("NewError failed: %v", err)
	}

	for name, envelope := range map[string]protocol.Envelope{
		"request":      request,
		"response":     response,
		"notification": notification,
		"error":        protocolError,
	} {
		if envelope.Generation != 7 || envelope.RuntimeID != "rt-1" || envelope.PluginID != "extension/plugin" || envelope.ServiceID != "svc-1" {
			t.Fatalf("%s route = runtime=%q plugin=%q service=%q generation=%d", name, envelope.RuntimeID, envelope.PluginID, envelope.ServiceID, envelope.Generation)
		}
	}
}

func TestAdoptPeerRoutingRejectsRebind(t *testing.T) {
	client := NewClient(nil, WithClientPluginID("extension/plugin"), WithClientGeneration(3))
	if err := client.AdoptPeerRouting(protocol.Envelope{Generation: 4, PluginID: "extension/plugin"}); err == nil {
		t.Fatal("expected generation rebind to be rejected")
	}
	if err := client.AdoptPeerRouting(protocol.Envelope{Generation: 3, PluginID: "other/plugin"}); err == nil {
		t.Fatal("expected plugin route rebind to be rejected")
	}
}

func TestAdoptPeerRoutingRejectsIncompleteRouteWithoutMutation(t *testing.T) {
	client := NewClient(nil)
	if err := client.AdoptPeerRouting(protocol.Envelope{
		RuntimeID:  "rt-partial",
		Generation: 8,
	}); err == nil {
		t.Fatal("expected incomplete authoritative route to be rejected")
	}
	if client.runtimeID != "" || client.pluginID != "" || client.serviceID != "" || client.generation != 0 {
		t.Fatalf("failed handshake partially mutated routing: runtime=%q plugin=%q service=%q generation=%d", client.runtimeID, client.pluginID, client.serviceID, client.generation)
	}
}

func TestAdoptPeerRoutingMismatchIsTransactional(t *testing.T) {
	client := NewClient(nil,
		WithClientRuntimeID("rt-1"),
		WithClientPluginID("extension/plugin"),
		WithClientServiceID("svc-1"),
		WithClientGeneration(4),
	)
	if err := client.AdoptPeerRouting(protocol.Envelope{
		RuntimeID:  "rt-1",
		PluginID:   "other/plugin",
		ServiceID:  "svc-2",
		Generation: 4,
	}); err == nil {
		t.Fatal("expected route mismatch")
	}
	if client.runtimeID != "rt-1" || client.pluginID != "extension/plugin" || client.serviceID != "svc-1" || client.generation != 4 {
		t.Fatalf("rejected rebind mutated client routing: runtime=%q plugin=%q service=%q generation=%d", client.runtimeID, client.pluginID, client.serviceID, client.generation)
	}
}
