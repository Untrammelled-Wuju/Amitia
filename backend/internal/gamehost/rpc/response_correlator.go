package rpc

import (
	"sync"

	"github.com/u-ai/backend/internal/gamehost/domain"
	"github.com/u-ai/backend/internal/gamehost/ipc"
	"github.com/u-ai/backend/pkg/gameplugin/protocol"
)

type pendingResponse struct {
	ch     chan *protocol.Envelope
	key    RequestKey
	cancel func()
}

type rpcResponseCorrelator struct {
	mu        sync.Mutex
	pending   map[RequestKey]*pendingResponse
	registry  PendingRequestRegistry
	keyIndex  map[string][]RequestKey
}

func NewRPCResponseCorrelator(registry PendingRequestRegistry) *rpcResponseCorrelator {
	if registry == nil {
		registry = NewPendingRequestRegistry(DefaultPendingRegistryConfig())
	}
	return &rpcResponseCorrelator{
		pending:  make(map[RequestKey]*pendingResponse),
		registry: registry,
		keyIndex: make(map[string][]RequestKey),
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

	for _, existingKey := range c.keyIndex[requestID] {
		if existingKey.RuntimeID != key.RuntimeID || existingKey.ServiceID != key.ServiceID {
			return nil, nil, false
		}
	}

	ch := make(chan *protocol.Envelope, 1)
	pr := &pendingResponse{
		ch:  ch,
		key: key,
	}
	c.pending[key] = pr
	c.keyIndex[requestID] = append(c.keyIndex[requestID], key)

	c.registry.Register(&PendingRequest{
		Key:       key,
		RequestID: requestID,
		State:     RequestStatePending,
		Done:      make(chan struct{}),
	})

	return ch, func() {
		c.remove(key)
	}, true
}

func (c *rpcResponseCorrelator) HandleResponse(peer ipc.Peer, envelope *protocol.Envelope) bool {
	key := RequestKey{
		RuntimeID: domain.RuntimeInstanceID(peer.RuntimeID),
		ServiceID: domain.ServiceID(peer.ServiceID),
		RequestID: envelope.RequestID,
	}

	c.mu.Lock()
	pr, ok := c.pending[key]
	if !ok {
		c.mu.Unlock()
		return false
	}
	delete(c.pending, key)
	c.removeKeyIndex(envelope.RequestID, key)
	c.mu.Unlock()

	c.registry.Complete(key, *envelope)

	select {
	case pr.ch <- envelope:
	default:
	}
	return true
}

func (c *rpcResponseCorrelator) CancelByPeer(peer ipc.Peer) {
	runtimeID := domain.RuntimeInstanceID(peer.RuntimeID)
	serviceID := domain.ServiceID(peer.ServiceID)

	c.mu.Lock()
	var toCancel []*pendingResponse
	for k, pr := range c.pending {
		if k.RuntimeID == runtimeID && k.ServiceID == serviceID {
			toCancel = append(toCancel, pr)
		}
	}
	for _, pr := range toCancel {
		delete(c.pending, pr.key)
		c.removeKeyIndex(pr.key.RequestID, pr.key)
	}
	c.mu.Unlock()

	for _, pr := range toCancel {
		c.registry.Cancel(pr.key)
		select {
		case pr.ch <- &protocol.Envelope{
			Protocol:  protocol.ProtocolVersion,
			Type:      protocol.MessageTypeResponse,
			ID:        pr.key.RequestID,
			RequestID: pr.key.RequestID,
			Error: &protocol.ProtocolError{
				Code:    protocol.ErrorCode(domain.ErrRuntimeUnavailable),
				Message: "connection closed",
			},
		}:
		default:
		}
	}
}

func (c *rpcResponseCorrelator) remove(key RequestKey) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if pr, ok := c.pending[key]; ok {
		delete(c.pending, key)
		c.removeKeyIndex(key.RequestID, key)
		c.registry.Remove(key)
		select {
		case pr.ch <- &protocol.Envelope{
			Protocol: protocol.ProtocolVersion,
			Type:     protocol.MessageTypeResponse,
			ID:       key.RequestID,
			RequestID: key.RequestID,
			Error: &protocol.ProtocolError{
				Code:    protocol.ErrorCode(domain.ErrCancelled),
				Message: "pending request cleaned up",
			},
		}:
		default:
		}
	}
}

func (c *rpcResponseCorrelator) removeKeyIndex(requestID string, key RequestKey) {
	keys := c.keyIndex[requestID]
	for i, k := range keys {
		if k == key {
			c.keyIndex[requestID] = append(keys[:i], keys[i+1:]...)
			break
		}
	}
	if len(c.keyIndex[requestID]) == 0 {
		delete(c.keyIndex, requestID)
	}
}

var _ ipc.ResponseCorrelator = (*rpcResponseCorrelator)(nil)
