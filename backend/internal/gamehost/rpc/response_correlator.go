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
	pending  map[RequestKey]chan *protocol.Envelope
}

func NewRPCResponseCorrelator(registry PendingRequestRegistry) *rpcResponseCorrelator {
	if registry == nil {
		registry = NewPendingRequestRegistry(DefaultPendingRegistryConfig())
	}
	return &rpcResponseCorrelator{
		registry: registry,
		pending:  make(map[RequestKey]chan *protocol.Envelope),
	}
}

func (c *rpcResponseCorrelator) RegisterPending(peer ipc.Peer, requestID string) (chan *protocol.Envelope, func(), bool) {
	key := RequestKey{
		RuntimeID: domain.RuntimeInstanceID(peer.RuntimeID),
		ServiceID: domain.ServiceID(peer.ServiceID),
		RequestID: requestID,
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if _, exists := c.pending[key]; exists {
		return nil, nil, false
	}

	req := &PendingRequest{
		Key:       key,
		RequestID: requestID,
		State:     RequestStatePending,
		Done:      make(chan struct{}),
	}

	if _, err := c.registry.Register(req); err != nil {
		return nil, nil, false
	}

	respCh := make(chan *protocol.Envelope, 1)
	c.pending[key] = respCh

	cancel := func() {
		c.remove(key)
	}

	return respCh, cancel, true
}

func (c *rpcResponseCorrelator) HandleResponse(peer ipc.Peer, envelope *protocol.Envelope) bool {
	key := RequestKey{
		RuntimeID: domain.RuntimeInstanceID(peer.RuntimeID),
		ServiceID: domain.ServiceID(peer.ServiceID),
		RequestID: envelope.RequestID,
	}

	c.mu.Lock()
	respCh, ok := c.pending[key]
	if !ok {
		c.mu.Unlock()
		return false
	}
	delete(c.pending, key)
	c.mu.Unlock()

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

	select {
	case respCh <- envelope:
	default:
	}
	return true
}

func (c *rpcResponseCorrelator) CancelByPeer(peer ipc.Peer) {
	runtimeID := domain.RuntimeInstanceID(peer.RuntimeID)
	serviceID := domain.ServiceID(peer.ServiceID)

	c.mu.Lock()
	var toCancel []RequestKey
	for k := range c.pending {
		if k.RuntimeID == runtimeID && k.ServiceID == serviceID {
			toCancel = append(toCancel, k)
		}
	}
	for _, k := range toCancel {
		delete(c.pending, k)
	}
	c.mu.Unlock()

	for _, k := range toCancel {
		c.registry.Cancel(k)
	}
}

func (c *rpcResponseCorrelator) remove(key RequestKey) {
	c.mu.Lock()
	if _, ok := c.pending[key]; !ok {
		c.mu.Unlock()
		return
	}
	delete(c.pending, key)
	c.mu.Unlock()

	c.registry.Remove(key)
}

var _ ipc.ResponseCorrelator = (*rpcResponseCorrelator)(nil)
