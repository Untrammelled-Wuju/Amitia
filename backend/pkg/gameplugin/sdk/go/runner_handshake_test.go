package sdk

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/u-ai/backend/pkg/gameplugin/protocol"
)

type handshakeTestTransport struct {
	sent     []protocol.Envelope
	response protocol.Envelope
}

func (t *handshakeTestTransport) Send(_ context.Context, message protocol.Envelope) error {
	t.sent = append(t.sent, message)
	return nil
}

func (t *handshakeTestTransport) Receive(_ context.Context) (protocol.Envelope, error) {
	return t.response, nil
}

func (t *handshakeTestTransport) Close() error { return nil }

func TestRunnerHandshakeSurfacesResponseEnvelopeError(t *testing.T) {
	transport := &handshakeTestTransport{
		response: protocol.Envelope{
			Protocol:   protocol.ProtocolVersion,
			Type:       protocol.MessageTypeResponse,
			ID:         "hello-response",
			RequestID:  "hello-request",
			RuntimeID:  "runtime-1",
			PluginID:   "extension/plugin",
			ServiceID:  "service-1",
			Generation: 3,
			Error: &protocol.ProtocolError{
				Code:    protocol.ErrorCode("protocol_mismatch"),
				Message: "unsupported protocol",
			},
		},
	}
	client := NewClient(transport, WithIDGenerator(&fixedRunnerIDGenerator{id: "hello-request"}))
	runner := NewRunner(client, RunnerConfig{Hello: HelloConfiguration{
		SupportedProtocols: []string{protocol.ProtocolVersion},
	}})

	_, err := runner.performHandshake(context.Background())
	if err == nil || !strings.Contains(err.Error(), "unsupported protocol") {
		t.Fatalf("expected host handshake error to be surfaced, got %v", err)
	}
	if got := client.GetGeneration(); got != 0 {
		t.Fatalf("failed handshake must not bind peer generation, got %d", got)
	}
}

type fixedRunnerIDGenerator struct{ id string }

func (g *fixedRunnerIDGenerator) NewID() string { return g.id }

func TestRunnerHandshakeAdoptsAuthoritativePeerRouting(t *testing.T) {
	payload, err := json.Marshal(HelloResponse{Protocol: protocol.ProtocolVersion})
	if err != nil {
		t.Fatalf("marshal response: %v", err)
	}
	transport := &handshakeTestTransport{
		response: protocol.Envelope{
			Protocol:   protocol.ProtocolVersion,
			Type:       protocol.MessageTypeResponse,
			ID:         "hello-response",
			RequestID:  "hello-request",
			RuntimeID:  "runtime-1",
			PluginID:   "extension/plugin",
			ServiceID:  "service-1",
			Generation: 9,
			Payload:    payload,
		},
	}
	client := NewClient(transport, WithIDGenerator(&fixedRunnerIDGenerator{id: "hello-request"}))
	runner := NewRunner(client, RunnerConfig{Hello: HelloConfiguration{
		SupportedProtocols: []string{protocol.ProtocolVersion},
	}})

	if _, err := runner.performHandshake(context.Background()); err != nil {
		t.Fatalf("performHandshake failed: %v", err)
	}
	request, err := client.NewRequest("vendor.operation.invoke", map[string]bool{"ok": true})
	if err != nil {
		t.Fatalf("NewRequest failed after handshake: %v", err)
	}
	if request.RuntimeID != "runtime-1" || request.PluginID != "extension/plugin" || request.ServiceID != "service-1" || request.Generation != 9 {
		t.Fatalf("post-handshake request route = runtime=%q plugin=%q service=%q generation=%d", request.RuntimeID, request.PluginID, request.ServiceID, request.Generation)
	}
}

func TestRunnerHandshakeRejectsWrongEnvelopeProtocolWithoutBinding(t *testing.T) {
	transport := &handshakeTestTransport{
		response: protocol.Envelope{
			Protocol:   "future-protocol/99",
			Type:       protocol.MessageTypeResponse,
			ID:         "hello-response",
			RequestID:  "hello-request",
			RuntimeID:  "runtime-1",
			PluginID:   "extension/plugin",
			ServiceID:  "service-1",
			Generation: 5,
		},
	}
	client := NewClient(transport, WithIDGenerator(&fixedRunnerIDGenerator{id: "hello-request"}))
	runner := NewRunner(client, RunnerConfig{Hello: HelloConfiguration{
		SupportedProtocols: []string{protocol.ProtocolVersion},
	}})

	if _, err := runner.performHandshake(context.Background()); err == nil || !strings.Contains(err.Error(), "envelope protocol mismatch") {
		t.Fatalf("expected envelope protocol mismatch, got %v", err)
	}
	if client.generation != 0 || client.runtimeID != "" || client.pluginID != "" || client.serviceID != "" {
		t.Fatalf("rejected handshake mutated routing: runtime=%q plugin=%q service=%q generation=%d", client.runtimeID, client.pluginID, client.serviceID, client.generation)
	}
}
