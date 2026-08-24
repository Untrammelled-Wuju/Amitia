package ipc

import (
	"fmt"

	"github.com/u-ai/backend/internal/gamehost/domain"
	"github.com/u-ai/backend/pkg/gameplugin/protocol"
)

type EnvelopeEvent string

const (
	EventConnectionAttached EnvelopeEvent = "ipc.connection.attached"
	EventConnectionDetached EnvelopeEvent = "ipc.connection.detached"
	EventConnectionError    EnvelopeEvent = "ipc.connection.error"
)

type ConnectionEvent struct {
	Type         EnvelopeEvent
	ConnectionID ConnectionID
	Peer         Peer
	Error        error
}

type EventHandler func(event ConnectionEvent)

func ValidateEnvelopePeer(envelope protocol.Envelope, peer Peer) error {
	if envelope.RuntimeID != "" && envelope.RuntimeID != string(peer.RuntimeID) {
		return NewIPCErrorWithCause(
			IPCErrorPeerRoute,
			domain.ErrInvalidArgument,
			fmt.Sprintf("envelope runtimeId mismatch: expected %s, got %s", peer.RuntimeID, envelope.RuntimeID),
			nil,
		)
	}
	if envelope.PluginID != "" && envelope.PluginID != string(peer.PluginID) {
		return NewIPCErrorWithCause(
			IPCErrorPeerRoute,
			domain.ErrInvalidArgument,
			fmt.Sprintf("envelope pluginId mismatch: expected %s, got %s", peer.PluginID, envelope.PluginID),
			nil,
		)
	}
	if envelope.ServiceID != "" && envelope.ServiceID != string(peer.ServiceID) {
		return NewIPCErrorWithCause(
			IPCErrorPeerRoute,
			domain.ErrInvalidArgument,
			fmt.Sprintf("envelope serviceId mismatch: expected %s, got %s", peer.ServiceID, envelope.ServiceID),
			nil,
		)
	}
	if peer.Generation > 0 {
		if envelope.Generation == 0 || envelope.Generation != uint64(peer.Generation) {
			return NewIPCErrorWithCause(IPCErrorPeerRoute, domain.ErrInvalidArgument, fmt.Sprintf("envelope generation mismatch: expected %d, got %d", peer.Generation, envelope.Generation), nil)
		}
	}
	return nil
}

func FillRouting(envelope *protocol.Envelope, peer Peer) {
	if envelope.RuntimeID == "" {
		envelope.RuntimeID = string(peer.RuntimeID)
	}
	if envelope.PluginID == "" {
		envelope.PluginID = string(peer.PluginID)
	}
	if envelope.ServiceID == "" {
		envelope.ServiceID = string(peer.ServiceID)
	}
	if envelope.Generation == 0 {
		envelope.Generation = uint64(peer.Generation)
	}
}
