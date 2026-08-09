package ipc

import (
	"context"

	"github.com/u-ai/backend/pkg/gameplugin/protocol"
)

type Dispatcher interface {
	Dispatch(
		ctx context.Context,
		peer Peer,
		envelope protocol.Envelope,
	) error
}

type noopDispatcher struct{}

func (n *noopDispatcher) Dispatch(ctx context.Context, peer Peer, envelope protocol.Envelope) error {
	return nil
}

func NewNoopDispatcher() Dispatcher {
	return &noopDispatcher{}
}

type MainDispatcher struct {
	eventHandler EventHandler
}

func NewMainDispatcher(eventHandler EventHandler) *MainDispatcher {
	return &MainDispatcher{
		eventHandler: eventHandler,
	}
}

func (d *MainDispatcher) Dispatch(ctx context.Context, peer Peer, envelope protocol.Envelope) error {
	return nil
}

func GetEnvelopeMessageType(envelope protocol.Envelope) string {
	return string(envelope.Type)
}

func IsRequest(envelope protocol.Envelope) bool {
	return envelope.Type == protocol.MessageTypeRequest
}

func IsResponse(envelope protocol.Envelope) bool {
	return envelope.Type == protocol.MessageTypeResponse
}

func IsNotification(envelope protocol.Envelope) bool {
	return envelope.Type == protocol.MessageTypeNotification
}

func IsError(envelope protocol.Envelope) bool {
	return envelope.Type == protocol.MessageTypeError
}
