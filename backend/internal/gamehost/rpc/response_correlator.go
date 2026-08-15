package rpc

import (
	"sync"

	"github.com/u-ai/backend/internal/gamehost/domain"
	"github.com/u-ai/backend/internal/gamehost/ipc"
	"github.com/u-ai/backend/pkg/gameplugin/protocol"
)

type rpcResponseCorrelator struct {
	mu       sync.Mutex
	registry PendingRequestRegistry
}

func NewRPCResponseCorrelator(registry PendingRequestRegistry) *rpcResponseCorrelator {
	if registry == nil {
		registry = NewPendingRequestRegistry(DefaultPendingRegistryConfig())
	}
	return &rpcResponseCorrelator{
		registry: registry,
	}
}

func (c *rpcResponseCorrelator) RegisterPending(peer ipc.Peer, requestID string, generation uint64) (ipc.PendingRequestHandle, bool) {
	key := RequestKey{
		RuntimeID: domain.RuntimeInstanceID(peer.RuntimeID),
		ServiceID: domain.ServiceID(peer.ServiceID),
		RequestID: requestID,
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	req := &PendingRequest{
		Key:         key,
		RequestID:   requestID,
		State:       RequestStatePending,
		done:        make(chan struct{}),
		Generation:  RequestGeneration(generation),
	}

	if _, err := c.registry.Register(req); err != nil {
		return nil, false
	}

	return req, true
}

func (c *rpcResponseCorrelator) HandleResponse(peer ipc.Peer, envelope *protocol.Envelope) bool {
	key := RequestKey{
		RuntimeID: domain.RuntimeInstanceID(peer.RuntimeID),
		ServiceID: domain.ServiceID(peer.ServiceID),
		RequestID: envelope.RequestID,
	}

	if envelope.Type == protocol.MessageTypeError {
		c.registry.Fail(key, NewRPCErrorWithCause(
			"protocol_error",
			domain.ErrorCode(envelope.Error.Code),
			envelope.Error.Message,
			nil,
		))
	} else {
		c.registry.Complete(key, *envelope)
	}
	return true
}

func (c *rpcResponseCorrelator) CancelByPeer(peer ipc.Peer) {
	runtimeID := domain.RuntimeInstanceID(peer.RuntimeID)
	serviceID := domain.ServiceID(peer.ServiceID)
	c.registry.CancelByPeer(runtimeID, serviceID)
}

func (c *rpcResponseCorrelator) CancelByRuntime(runtimeID string) {
	c.registry.CancelByRuntime(domain.RuntimeInstanceID(runtimeID))
}

var _ ipc.ResponseCorrelator = (*rpcResponseCorrelator)(nil)
