package channel

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/u-ai/backend/internal/gamehost/domain"
	"github.com/u-ai/backend/internal/gamehost/ipc"
	"github.com/u-ai/backend/pkg/gameplugin/protocol"
)

type outboundGenerationStub struct{ generation int64 }

func (s outboundGenerationStub) GetCurrentGeneration(domain.RuntimeInstanceID) (int64, error) {
	return s.generation, nil
}

type outboundControlStub struct {
	peer     ipc.Peer
	envelope protocol.Envelope
	calls    int
}

func (s *outboundControlStub) Send(_ context.Context, peer ipc.Peer, envelope protocol.Envelope) error {
	s.peer = peer
	s.envelope = envelope
	s.calls++
	return nil
}

func registerOutboundTestChannel(t *testing.T, reg Registry, direction protocol.ChannelDirection) {
	t.Helper()
	ch := RuntimeChannel{
		ID:        NewRuntimeChannelID("runtime-1", "service-a", "host-events"),
		PluginID:  "plugin-x",
		RuntimeID: "runtime-1",
		ServiceID: "service-a",
		ChannelID: "host-events",
		Kind:      domain.ChannelKindCustom,
		Direction: direction,
	}
	if err := reg.Register(context.Background(), ch); err != nil {
		t.Fatalf("register channel: %v", err)
	}
}

func TestOutboundPublisherPublishesHostToPlugin(t *testing.T) {
	reg := NewRegistry(Options{})
	registerOutboundTestChannel(t, reg, protocol.ChannelDirectionHostToPlugin)
	control := &outboundControlStub{}
	publisher, err := NewOutboundPublisher(reg, control, outboundGenerationStub{generation: 7})
	if err != nil {
		t.Fatal(err)
	}

	if err := publisher.Publish(context.Background(), OutboundMessage{
		RuntimeID: "runtime-1",
		ServiceID: "service-a",
		ChannelID: "host-events",
		Payload:   json.RawMessage(`{"input":"jump"}`),
	}); err != nil {
		t.Fatalf("publish: %v", err)
	}
	if control.calls != 1 {
		t.Fatalf("expected one send, got %d", control.calls)
	}
	if control.envelope.Method != MethodChannelDeliver || control.envelope.Generation != 7 {
		t.Fatalf("unexpected envelope: method=%s generation=%d", control.envelope.Method, control.envelope.Generation)
	}
	if control.peer.PluginID != "plugin-x" || control.peer.RuntimeID != "runtime-1" || control.peer.ServiceID != "service-a" || control.peer.Generation != 7 {
		t.Fatalf("unexpected peer: %#v", control.peer)
	}
	var payload channelDeliveryPayload
	if err := json.Unmarshal(control.envelope.Payload, &payload); err != nil {
		t.Fatal(err)
	}
	if payload.ChannelID != "host-events" || string(payload.Payload) != `{"input":"jump"}` {
		t.Fatalf("unexpected payload: %#v", payload)
	}
}

func TestOutboundPublisherRejectsPluginToHostOnlyChannel(t *testing.T) {
	reg := NewRegistry(Options{})
	registerOutboundTestChannel(t, reg, protocol.ChannelDirectionPluginToHost)
	control := &outboundControlStub{}
	publisher, err := NewOutboundPublisher(reg, control, outboundGenerationStub{generation: 1})
	if err != nil {
		t.Fatal(err)
	}
	if err := publisher.Publish(context.Background(), OutboundMessage{
		RuntimeID: "runtime-1",
		ServiceID: "service-a",
		ChannelID: "host-events",
		Payload:   json.RawMessage(`{}`),
	}); err == nil {
		t.Fatal("expected direction rejection")
	}
	if control.calls != 0 {
		t.Fatalf("unexpected sends: %d", control.calls)
	}
}

func TestOutboundPublisherAllowsBidirectionalChannel(t *testing.T) {
	reg := NewRegistry(Options{})
	registerOutboundTestChannel(t, reg, protocol.ChannelDirectionBidirectional)
	publisher, err := NewOutboundPublisher(reg, &outboundControlStub{}, outboundGenerationStub{generation: 2})
	if err != nil {
		t.Fatal(err)
	}
	if err := publisher.Publish(context.Background(), OutboundMessage{
		RuntimeID: "runtime-1",
		ServiceID: "service-a",
		ChannelID: "host-events",
		Payload:   json.RawMessage(`null`),
	}); err != nil {
		t.Fatalf("bidirectional outbound publish: %v", err)
	}
}
