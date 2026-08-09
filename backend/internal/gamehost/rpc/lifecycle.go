package rpc

import (
	"context"
	"sync"
	"time"

	"github.com/u-ai/backend/internal/gamehost/domain"
	"github.com/u-ai/backend/internal/gamehost/ipc"
	"github.com/u-ai/backend/pkg/gameplugin/protocol"
)

type RequestLifecycleManager struct {
	mu           sync.Mutex
	pending      PendingRequestRegistry
	correlation  *CorrelationMap
	cache        *CompletedResponseCache
	idGenerator  func() string
	timeoutCfg   TimeoutConfig
	cp           ipc.ControlPlane
	connHandler  ConnectionEventHandler
	shuttingDown bool
}

type LifecycleManagerConfig struct {
	Pending   PendingRequestRegistry
	Correlation *CorrelationMap
	Cache     *CompletedResponseCache
	IDGenerator func() string
	Timeout   TimeoutConfig
}

func NewLifecycleManager(config LifecycleManagerConfig) *RequestLifecycleManager {

	pending := config.Pending
	if pending == nil {
		pending = NewPendingRequestRegistry(DefaultPendingRegistryConfig())
	}

	corr := config.Correlation
	if corr == nil {
		corr = NewCorrelationMap()
	}

	cache := config.Cache
	if cache == nil {
		cache = NewCompletedResponseCache(DefaultCompletedResponseCacheConfig())
	}

	idGen := config.IDGenerator
	if idGen == nil {
		idGen = defaultIDGenerator()
	}

 timeoutCfg := config.Timeout
	if timeoutCfg.Default == 0 {
		timeoutCfg = DefaultTimeoutConfig()
	}

	return &RequestLifecycleManager{
		pending:     pending,
		correlation: corr,
		cache:       cache,
		idGenerator: idGen,
		timeoutCfg:  timeoutCfg,
	}
}

func (m *RequestLifecycleManager) SetControlPlane(cp ipc.ControlPlane) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cp = cp
}

func (m *RequestLifecycleManager) SetConnectionEventHandler(h ConnectionEventHandler) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.connHandler = h
}

func (m *RequestLifecycleManager) getCP() ipc.ControlPlane {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.cp
}

func (m *RequestLifecycleManager) HandleOutgoingRequest(
	ctx context.Context,
	sourcePeer ipc.Peer,
	request protocol.Envelope,
	customTimeoutMS int64,
) (*PendingRequest, error) {
	m.mu.Lock()
	if m.shuttingDown {
		m.mu.Unlock()
		return nil, NewLifecycleError(LifecycleErrorCancelled, domain.ErrInvalidState, "lifecycle manager shutting down", nil)
	}
	m.mu.Unlock()

	key := RequestKeyFromIPC(request.ID, sourcePeer)
	fp := ComputeRequestFingerprint(request)

	if cached, ok := m.cache.Lookup(key, fp); ok {
		return &PendingRequest{
			Key:     key,
			State:   RequestStateCompleted,
			Request: request,
			Result:  clonedEnvelope(cached),
		}, nil
	}

	deadline, _ := EffectiveDeadline(ctx, customTimeoutMS, m.timeoutCfg, time.Now().UTC())

	cancelCtx, cancel := context.WithDeadline(ctx, deadline)

	req := &PendingRequest{
		Key:         key,
		Method:      Method(request.Method),
		RequestID:   request.ID,
		Namespace:   Namespace(request.Method),
		Request:     request,
		State:       RequestStatePending,
		CreatedAt:   time.Now().UTC(),
		Ctx:         cancelCtx,
		CancelFunc:  cancel,
		Done:        make(chan struct{}),
		Fingerprint: fp,
	}

	if _, err := m.pending.Register(req); err != nil {
		cancel()
		return nil, err
	}

	req.State = RequestStateRunning

	return req, nil
}

func (m *RequestLifecycleManager) HandleIncomingResponse(
	sourcePeer ipc.Peer,
	response protocol.Envelope,
) error {
	targetKey := RequestKey{
		RuntimeID: domain.RuntimeInstanceID(sourcePeer.RuntimeID),
		ServiceID: domain.ServiceID(sourcePeer.ServiceID),
		RequestID: response.RequestID,
	}

	if correlation, ok := m.correlation.ByDownstream(sourcePeer, response.RequestID); ok {
		upstreamKey := correlation.Upstream

		ok, _ := m.pending.Complete(upstreamKey, response)
		if ok {
			cachedReq := &CompletedRequest{
				Key:         upstreamKey,
				Fingerprint: "",
				Response:    cloneEnvelope(response),
				FinishedAt:  time.Now().UTC(),
			}
			m.cache.Save(*cachedReq)
		}

		m.correlation.Remove(upstreamKey)

		if cp := m.getCP(); cp != nil {
			upstreamResponse := response
			upstreamResponse.RequestID = upstreamKey.RequestID
			if ipcPeer := keyToPeer(upstreamKey); ipcPeer != nil {
				_ = cp.Send(context.Background(), *ipcPeer, upstreamResponse)
			}
		}

		return nil
	}

	ok, _ := m.pending.Complete(targetKey, response)

	if !ok {
		req := m.pending.(*pendingRequestRegistry)
		_ = req
		return nil
	}

	m.cache.Save(CompletedRequest{
		Key:        targetKey,
		Response:   cloneEnvelope(response),
		FinishedAt: time.Now().UTC(),
	})

	m.pending.Remove(targetKey)
	return nil
}

func (m *RequestLifecycleManager) HandleCancel(
	sourcePeer ipc.Peer,
	cancel CancelRequest,
) error {
	key := RequestKey{
		RuntimeID: domain.RuntimeInstanceID(sourcePeer.RuntimeID),
		ServiceID: domain.ServiceID(sourcePeer.ServiceID),
		RequestID: cancel.RequestID,
	}

	req := findPending(m.pending, key)
	if req == nil {
		return NewLifecycleError(
			LifecycleErrorCancelled,
			domain.ErrNotFound,
			"cannot cancel request not owned by peer",
			nil,
		)
	}

	ok, _ := m.pending.Cancel(key)
	if !ok {
		return nil
	}

	if correlation, ok := m.correlation.ByUpstream(key); ok {
		cp := m.getCP()
		if cp != nil {
			cancelEnv := BuildCancelEnvelope(correlation.DownstreamReqID, "upstream cancelled")
			_ = cp.Send(context.Background(), correlation.DownstreamPeer, cancelEnv)
		}
	}

	return nil
}

func (m *RequestLifecycleManager) MarkCompleted(key RequestKey, err error) {
	if err != nil {
		m.pending.Fail(key, err)
	} else {
		m.pending.Remove(key)
	}
}

func (m *RequestLifecycleManager) NotifyConnectionDetach(peer ipc.Peer) {
	runtimeID := string(peer.RuntimeID)
	serviceID := string(peer.ServiceID)

	m.pending.ListByPeer(runtimeID, serviceID)
}

func (m *RequestLifecycleManager) Shutdown(ctx context.Context) error {
	m.mu.Lock()
	m.shuttingDown = true
	m.mu.Unlock()

	done := make(chan struct{})
	go func() {
		if r, ok := m.pending.(*pendingRequestRegistry); ok {
			r.shutdown()
		}
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func clonedEnvelope(env protocol.Envelope) protocol.Envelope {
	return cloneEnvelope(env)
}

func keyToPeer(key RequestKey) *ipc.Peer {
	if key.RuntimeID == "" || key.ServiceID == "" {
		return nil
	}
	return &ipc.Peer{
		RuntimeID: key.RuntimeID,
		ServiceID: key.ServiceID,
	}
}

func findPending(reg PendingRequestRegistry, key RequestKey) *PendingRequest {
	if r, ok := reg.(*pendingRequestRegistry); ok {
		return r.get(key)
	}
	return nil
}
