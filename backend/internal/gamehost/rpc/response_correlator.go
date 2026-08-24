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

func (c *rpcResponseCorrelator) RegisterPending(peer ipc.Peer, requestID string, generation uint64, method string, payload []byte) (ipc.PendingRequestHandle, bool) {
	key := RequestKey{
		RuntimeID: domain.RuntimeInstanceID(peer.RuntimeID),
		ServiceID: domain.ServiceID(peer.ServiceID),
		RequestID: requestID,
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	fingerprint := ComputeFingerprint(method, payload)

	req := &PendingRequest{
		Key:         key,
		RequestID:   requestID,
		Method:      Method(method),
		State:       RequestStatePending,
		done:        make(chan struct{}),
		Generation:  RequestGeneration(generation),
		Fingerprint: fingerprint,
	}

	registered, err := c.registry.Register(req)
	if err != nil {
		return nil, false
	}

	if !registered {
		existing := c.registry.Get(key)
		if existing != nil {
			return existing, false
		}
		return nil, false
	}

	return req, true
}

func (c *rpcResponseCorrelator) HandleResponse(peer ipc.Peer, envelope *protocol.Envelope) bool {
	if envelope == nil || envelope.RequestID == "" {
		return false
	}
	key := RequestKey{
		RuntimeID: domain.RuntimeInstanceID(peer.RuntimeID),
		ServiceID: domain.ServiceID(peer.ServiceID),
		RequestID: envelope.RequestID,
	}

	if c.registry.Get(key) == nil {
		return false
	}

	var completed bool
	if envelope.Type == protocol.MessageTypeError {
		if envelope.Error == nil {
			return false
		}
		completed, _ = c.registry.Fail(key, NewRPCErrorWithCause(
			"protocol_error",
			domain.ErrorCode(envelope.Error.Code),
			envelope.Error.Message,
			nil,
		))
	} else {
		completed, _ = c.registry.Complete(key, *envelope)
	}
	return completed
}

func (c *rpcResponseCorrelator) CancelByPeer(peer ipc.Peer) {
	runtimeID := domain.RuntimeInstanceID(peer.RuntimeID)
	serviceID := domain.ServiceID(peer.ServiceID)
	c.registry.CancelByPeer(runtimeID, serviceID)
}

func (c *rpcResponseCorrelator) CancelByRuntime(runtimeID string) {
	c.registry.CancelByRuntime(domain.RuntimeInstanceID(runtimeID))
}

func (c *rpcResponseCorrelator) Terminalize(key ipc.TerminalKey, state ipc.TerminalState, err error) {
	requestKey := RequestKey{
		RuntimeID: domain.RuntimeInstanceID(key.RuntimeID),
		ServiceID: domain.ServiceID(key.ServiceID),
		RequestID: key.RequestID,
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	switch state {
	case ipc.TerminalCompleted:
		c.registry.Complete(requestKey, protocol.Envelope{})
	case ipc.TerminalFailed:
		c.registry.Fail(requestKey, err)
	case ipc.TerminalTimedOut:
		c.registry.Timeout(requestKey)
	case ipc.TerminalCancelled:
		c.registry.Cancel(requestKey)
	}

	c.registry.Remove(requestKey)
}

var _ ipc.ResponseCorrelator = (*rpcResponseCorrelator)(nil)
