package jsonrpc

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

var (
	ErrRequestNotFound = errors.New("jsonrpc: request not found")
	ErrDuplicateResp   = errors.New("jsonrpc: duplicate response")
	ErrTransportClosed = errors.New("jsonrpc: transport closed")
)

type PendingRequest struct {
	ID        RequestID
	Method    string
	Deadline  time.Time
	CreatedAt time.Time
	OnDone    func(*Response)
	OnCancel  func(reason string)
	cancel    context.CancelFunc
	done      chan *Response
	once      sync.Once
}

func newPending(id RequestID, method string, timeout time.Duration, onDone func(*Response), onCancel func(string)) *PendingRequest {
	_, cancel := context.WithTimeout(context.Background(), timeout)
	pr := &PendingRequest{
		ID:        id,
		Method:    method,
		Deadline:  time.Now().Add(timeout),
		CreatedAt: time.Now(),
		OnDone:    onDone,
		OnCancel:  onCancel,
		done:      make(chan *Response, 1),
	}
	pr.cancel = cancel
	return pr
}

func (p *PendingRequest) Wait(ctx context.Context) (*Response, error) {
	select {
	case resp := <-p.done:
		return resp, nil
	case <-ctx.Done():
		p.cancelWith(ctx.Err().Error())
		return nil, ctx.Err()
	}
}

func (p *PendingRequest) deliver(resp *Response) bool {
	delivered := false
	p.once.Do(func() {
		select {
		case p.done <- resp:
			delivered = true
		default:
		}
		if p.OnDone != nil {
			p.OnDone(resp)
		}
	})
	return delivered
}

func (p *PendingRequest) cancelWith(reason string) {
	p.once.Do(func() {
		p.cancel()
		if p.OnCancel != nil {
			p.OnCancel(reason)
		}
		select {
		case p.done <- &Response{
			JSONRPC: ProtocolVersion,
			ID:      p.ID,
			Error:   CancelledError(reason),
		}:
		default:
		}
	})
}

type RequestTracker struct {
	mu       sync.Mutex
	pending  map[string]*PendingRequest
	closed   bool
	closeCh  chan struct{}
	failAll  chan struct{}
	counter  int64
	onCancel func(targetID RequestID, reason string)
}

func NewRequestTracker() *RequestTracker {
	return &RequestTracker{
		pending: make(map[string]*PendingRequest),
		closeCh: make(chan struct{}),
		failAll: make(chan struct{}),
	}
}

func (t *RequestTracker) SetCancelHandler(h func(targetID RequestID, reason string)) {
	t.mu.Lock()
	t.onCancel = h
	t.mu.Unlock()
}

func (t *RequestTracker) nextID() RequestID {
	n := atomic.AddInt64(&t.counter, 1)
	return NewNumberID(n)
}

func (t *RequestTracker) Track(method string, timeout time.Duration) (*PendingRequest, RequestID, error) {
	t.mu.Lock()
	if t.closed {
		t.mu.Unlock()
		return nil, RequestID{}, ErrTransportClosed
	}
	id := t.nextID()
	pr := newPending(id, method, timeout, nil, nil)
	t.pending[id.String()] = pr
	t.mu.Unlock()
	return pr, id, nil
}

func (t *RequestTracker) TrackWith(id RequestID, method string, timeout time.Duration) (*PendingRequest, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.closed {
		return nil, ErrTransportClosed
	}
	if _, exists := t.pending[id.String()]; exists {
		return nil, fmt.Errorf("jsonrpc: duplicate request id %s", id.String())
	}
	pr := newPending(id, method, timeout, nil, nil)
	t.pending[id.String()] = pr
	return pr, nil
}

func (t *RequestTracker) Resolve(resp *Response) error {
	t.mu.Lock()
	pr, ok := t.pending[resp.ID.String()]
	if !ok {
		t.mu.Unlock()
		return ErrRequestNotFound
	}
	delete(t.pending, resp.ID.String())
	t.mu.Unlock()
	if !pr.deliver(resp) {
		return ErrDuplicateResp
	}
	return nil
}

func (t *RequestTracker) Cancel(targetID RequestID, reason string) error {
	t.mu.Lock()
	pr, ok := t.pending[targetID.String()]
	cancelHandler := t.onCancel
	t.mu.Unlock()
	if !ok {
		return ErrRequestNotFound
	}
	pr.cancelWith(reason)
	if cancelHandler != nil {
		cancelHandler(targetID, reason)
	}
	return nil
}

func (t *RequestTracker) FailAll(reason string) {
	t.mu.Lock()
	if t.closed {
		t.mu.Unlock()
		return
	}
	t.closed = true
	close(t.failAll)
	pending := t.pending
	t.pending = make(map[string]*PendingRequest)
	t.mu.Unlock()
	for _, pr := range pending {
		pr.cancelWith(reason)
	}
}

func (t *RequestTracker) Close() {
	t.mu.Lock()
	if t.closed {
		t.mu.Unlock()
		return
	}
	t.closed = true
	close(t.closeCh)
	t.mu.Unlock()
	t.FailAll("tracker closed")
}

func (t *RequestTracker) PendingCount() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return len(t.pending)
}

func (t *RequestTracker) Closed() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.closed
}

func (t *RequestTracker) FailAllCh() <-chan struct{} {
	return t.failAll
}

type CancelRequest struct {
	TargetRequestID string `json:"target_request_id"`
	Reason          string `json:"reason"`
}

type CancellationRegistry struct {
	mu      sync.RWMutex
	signals map[string]context.CancelFunc
	reasons map[string]string
}

func NewCancellationRegistry() *CancellationRegistry {
	return &CancellationRegistry{
		signals: make(map[string]context.CancelFunc),
		reasons: make(map[string]string),
	}
}

func (r *CancellationRegistry) Register(id string, cancel context.CancelFunc) {
	r.mu.Lock()
	r.signals[id] = cancel
	r.mu.Unlock()
}

func (r *CancellationRegistry) Unregister(id string) {
	r.mu.Lock()
	delete(r.signals, id)
	delete(r.reasons, id)
	r.mu.Unlock()
}

func (r *CancellationRegistry) Cancel(id, reason string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	cancel, ok := r.signals[id]
	if !ok {
		return false
	}
	r.reasons[id] = reason
	cancel()
	return true
}

func (r *CancellationRegistry) Reason(id string) string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.reasons[id]
}

func (r *CancellationRegistry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.signals)
}

func (r *CancellationRegistry) CancelAll(reason string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for id, cancel := range r.signals {
		r.reasons[id] = reason
		cancel()
	}
}
