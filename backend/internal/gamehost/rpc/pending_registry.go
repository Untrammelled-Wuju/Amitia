package rpc

import (
	"context"
	"sync"

	"github.com/u-ai/backend/internal/gamehost/domain"
	"github.com/u-ai/backend/internal/gamehost/ipc"
	"github.com/u-ai/backend/pkg/gameplugin/protocol"
)

type PendingRequestRegistry interface {
	Register(req *PendingRequest) (bool, error)
	Complete(key RequestKey, result protocol.Envelope) (bool, error)
	Fail(key RequestKey, err error) (bool, error)
	Timeout(key RequestKey) (bool, error)
	Cancel(key RequestKey) (bool, error)
	Remove(key RequestKey) bool
	ListByPeer(runtimeID, serviceID string) []*PendingRequest
	Count() int
}

type pendingRequestRegistry struct {
	mu       sync.Mutex
	requests map[RequestKey]*PendingRequest
	maxPerPeer int
	peerCount map[RequestKey]int
}

type PendingRegistryConfig struct {
	MaxPerPeer int
	MaxGlobal  int
}

func DefaultPendingRegistryConfig() PendingRegistryConfig {
	return PendingRegistryConfig{
		MaxPerPeer: 256,
		MaxGlobal:  4096,
	}
}

func NewPendingRequestRegistry(config PendingRegistryConfig) PendingRequestRegistry {
	if config.MaxPerPeer <= 0 {
		config.MaxPerPeer = 256
	}
	if config.MaxGlobal <= 0 {
		config.MaxGlobal = 4096
	}
	return &pendingRequestRegistry{
		requests:  make(map[RequestKey]*PendingRequest),
		maxPerPeer: config.MaxPerPeer,
		peerCount: make(map[RequestKey]int),
	}
}

func (r *pendingRequestRegistry) Register(req *PendingRequest) (bool, error) {
	if req == nil {
		return false, nil
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if existing, ok := r.requests[req.Key]; ok {
		if existing.Fingerprint == req.Fingerprint && !existing.IsTerminal() {
			return false, nil
		}
		if existing.Fingerprint != req.Fingerprint {
			return false, NewRPCErrorWithCause(
				"duplicate_request_id",
				domain.ErrInvalidArgument,
				"request id reused with different fingerprint",
				nil,
			)
		}
	}

	peerKey := req.Key
	count := r.peerCount[peerKey]
	if count >= r.maxPerPeer {
		return false, NewRPCErrorWithCause(
			"resource_exhausted",
			domain.ErrResourceExhausted,
			"maximum pending requests per peer reached",
			nil,
		)
	}

	if len(r.requests) >= 4096 {
		return false, NewRPCErrorWithCause(
			"resource_exhausted",
			domain.ErrResourceExhausted,
			"maximum global pending requests reached",
			nil,
		)
	}

	r.requests[req.Key] = req
	r.peerCount[peerKey] = count + 1
	return true, nil
}

func (r *pendingRequestRegistry) Complete(key RequestKey, result protocol.Envelope) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.transitionLocked(key, RequestStateCompleted, &result, nil)
}

func (r *pendingRequestRegistry) Fail(key RequestKey, err error) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.transitionLocked(key, RequestStateFailed, nil, err)
}

func (r *pendingRequestRegistry) Timeout(key RequestKey) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.transitionLocked(key, RequestStateTimedOut, nil, nil)
}

func (r *pendingRequestRegistry) Cancel(key RequestKey) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.transitionLocked(key, RequestStateCancelled, nil, nil)
}

func (r *pendingRequestRegistry) transitionLocked(
	key RequestKey,
	targetState RequestState,
	result *protocol.Envelope,
	err error,
) (bool, error) {
	req, ok := r.requests[key]
	if !ok {
		return false, nil
	}

	if req.State.IsTerminal() {
		return false, nil
	}

	req.State = targetState

	if result != nil {
		req.Result = *result
	}
	req.Error = err

	if req.CancelFunc != nil {
		req.CancelFunc()
	}

	if req.Done != nil {
		select {
		case <-req.Done:
		default:
			close(req.Done)
		}
	}

	return true, nil
}

func (r *pendingRequestRegistry) Remove(key RequestKey) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	req, ok := r.requests[key]
	if !ok {
		return false
	}

	if req.CancelFunc != nil {
		req.CancelFunc()
	}

	if req.Done != nil {
		select {
		case <-req.Done:
		default:
			close(req.Done)
		}
	}

	delete(r.requests, key)
	peerKey := key
	if r.peerCount[peerKey] > 0 {
		r.peerCount[peerKey]--
	}
	return true
}

func (r *pendingRequestRegistry) ListByPeer(runtimeID, serviceID string) []*PendingRequest {
	r.mu.Lock()
	defer r.mu.Unlock()

	result := make([]*PendingRequest, 0)
	rk := RequestKey{
		RuntimeID: domain.RuntimeInstanceID(runtimeID),
		ServiceID: domain.ServiceID(serviceID),
	}
	for k, req := range r.requests {
		if k.RuntimeID == rk.RuntimeID && k.ServiceID == rk.ServiceID {
			result = append(result, req)
		}
	}
	return result
}

func (r *pendingRequestRegistry) Count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.requests)
}

func (r *pendingRequestRegistry) get(key RequestKey) *PendingRequest {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.requests[key]
}

func (r *pendingRequestRegistry) shutdown() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.shutdownLocked()
}

func (r *pendingRequestRegistry) shutdownLocked() {
	for k, req := range r.requests {
		if !req.State.IsTerminal() {
			req.State = RequestStateCancelled
			if req.CancelFunc != nil {
				req.CancelFunc()
			}
			if req.Done != nil {
				select {
				case <-req.Done:
				default:
					close(req.Done)
				}
			}
		}
		delete(r.requests, k)
	}
	r.peerCount = make(map[RequestKey]int)
}

type ConnectionEventType string

const (
	EventAttach   ConnectionEventType = "attach"
	EventDetach   ConnectionEventType = "detach"
	EventShutdown ConnectionEventType = "shutdown"
)

type ConnectionEventHandler func(ctx context.Context, event ConnectionEvent)

type ConnectionEvent struct {
	Type         ConnectionEventType
	ConnectionID string
	Peer         ipc.Peer
}

