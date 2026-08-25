package sdk

import (
	"context"
	"strings"
	"testing"

	"github.com/u-ai/game-plugin-sdk-go/protocol"
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
