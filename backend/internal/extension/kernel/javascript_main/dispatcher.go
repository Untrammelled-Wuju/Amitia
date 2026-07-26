package javascript_main

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/runtime"
)

type InvocationContext struct {
	InvocationID    string
	TraceID         string
	Deadline        time.Time
	CancelSignal    CancelSignal
	ScopeSummary    string
	IdempotencyKey  string
	EntryType       string
	EntryName       string
}

type CancelSignal struct {
	mu     sync.Mutex
	done   chan struct{}
	reason string
}

func NewCancelSignal() *CancelSignal {
	return &CancelSignal{done: make(chan struct{})}
}

func (c *CancelSignal) Abort(reason string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	select {
	case <-c.done:
	default:
		c.reason = reason
		close(c.done)
	}
}

func (c *CancelSignal) Done() <-chan struct{} {
	return c.done
}

func (c *CancelSignal) Reason() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.reason
}

func (c *CancelSignal) IsAborted() bool {
	select {
	case <-c.done:
		return true
	default:
		return false
	}
}

type InvocationStatus string

const (
	InvocationStatusQueued   InvocationStatus = "queued"
	InvocationStatusRunning  InvocationStatus = "running"
	InvocationStatusSucceeded InvocationStatus = "succeeded"
	InvocationStatusFailed   InvocationStatus = "failed"
	InvocationStatusCancelled InvocationStatus = "cancelled"
	InvocationStatusTimedOut InvocationStatus = "timed_out"
)

type Invocation struct {
	InvocationID string
	EntryType    string
	EntryName    string
	Input        interface{}
	Status       InvocationStatus
	StartedAt    time.Time
	FinishedAt   *time.Time
	CancelSignal *CancelSignal
	Result       interface{}
	Error        error
}

type InvocationResult struct {
	InvocationID string
	Status       InvocationStatus
	Result       interface{}
	Error        string
	Duration     time.Duration
}

type InvocationDispatcher struct {
	mu               sync.RWMutex
	limits           runtime.ResourceLimits
	invocations      map[string]*Invocation
	activeCount      int32
	queuedCount      int32
	rejectNew        bool
	completedSignal  chan struct{}
}

func NewInvocationDispatcher(limits runtime.ResourceLimits) *InvocationDispatcher {
	return &InvocationDispatcher{
		limits:          limits,
		invocations:     make(map[string]*Invocation),
		completedSignal: make(chan struct{}),
	}
}

func (d *InvocationDispatcher) Dispatch(ctx context.Context, handlerType HandlerType, entryName string, input interface{}, invocationID string, deadline time.Time, handler HandlerFunc) InvocationResult {
	d.mu.Lock()
	if d.rejectNew {
		d.mu.Unlock()
		return InvocationResult{
			InvocationID: invocationID,
			Status:       InvocationStatusFailed,
			Error:        "runtime shutting down; new invocations rejected",
		}
	}
	maxConcurrent := d.limits.MaxConcurrentCalls
	if maxConcurrent <= 0 {
		maxConcurrent = 8
	}
	if int(d.activeCount) >= maxConcurrent {
		d.mu.Unlock()
		return InvocationResult{
			InvocationID: invocationID,
			Status:       InvocationStatusFailed,
			Error:        "max concurrent invocations reached",
		}
	}

	cancelSignal := NewCancelSignal()
	invocation := &Invocation{
		InvocationID: invocationID,
		EntryType:    string(handlerType),
		EntryName:    entryName,
		Input:        input,
		Status:       InvocationStatusRunning,
		StartedAt:    time.Now().UTC(),
		CancelSignal: cancelSignal,
	}
	d.invocations[invocationID] = invocation
	d.activeCount++
	d.mu.Unlock()

	defer func() {
		d.mu.Lock()
		delete(d.invocations, invocationID)
		d.activeCount--
		d.mu.Unlock()
		select {
		case d.completedSignal <- struct{}{}:
		default:
		}
	}()

	invCtx := InvocationContext{
		InvocationID: invocationID,
		Deadline:     deadline,
		CancelSignal: *cancelSignal,
		EntryType:    string(handlerType),
		EntryName:    entryName,
	}

	timeout := time.Until(deadline)
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	if d.limits.SingleCallTimeout != "" {
		if configured, err := time.ParseDuration(d.limits.SingleCallTimeout); err == nil && configured > 0 && configured < timeout {
			timeout = configured
		}
	}

	done := make(chan struct{})
	var result interface{}
	var err error

	go func() {
		defer func() {
			if r := recover(); r != nil {
				err = fmt.Errorf("javascript_main: handler panic: %v", r)
			}
			close(done)
		}()
		result, err = handler(input, invCtx)
	}()

	select {
	case <-done:
		now := time.Now().UTC()
		invocation.FinishedAt = &now
		if err != nil {
			invocation.Status = InvocationStatusFailed
			invocation.Error = err
			return InvocationResult{
				InvocationID: invocationID,
				Status:       InvocationStatusFailed,
				Error:        err.Error(),
				Duration:     time.Since(invocation.StartedAt),
			}
		}
		invocation.Status = InvocationStatusSucceeded
		invocation.Result = result
		return InvocationResult{
			InvocationID: invocationID,
			Status:       InvocationStatusSucceeded,
			Result:       result,
			Duration:     time.Since(invocation.StartedAt),
		}
	case <-cancelSignal.Done():
		now := time.Now().UTC()
		invocation.FinishedAt = &now
		invocation.Status = InvocationStatusCancelled
		invocation.Error = errors.New(cancelSignal.Reason())
		return InvocationResult{
			InvocationID: invocationID,
			Status:       InvocationStatusCancelled,
			Error:        cancelSignal.Reason(),
			Duration:     time.Since(invocation.StartedAt),
		}
	case <-time.After(timeout):
		now := time.Now().UTC()
		invocation.FinishedAt = &now
		invocation.Status = InvocationStatusTimedOut
		cancelSignal.Abort("timeout")
		return InvocationResult{
			InvocationID: invocationID,
			Status:       InvocationStatusTimedOut,
			Error:        "invocation timed out",
			Duration:     timeout,
		}
	}
}

func (d *InvocationDispatcher) Cancel(invocationID, reason string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	inv, exists := d.invocations[invocationID]
	if !exists {
		return fmt.Errorf("javascript_main: invocation %s not found", invocationID)
	}
	inv.CancelSignal.Abort(reason)
	return nil
}

func (d *InvocationDispatcher) ActiveCount() int {
	return int(atomic.LoadInt32(&d.activeCount))
}

func (d *InvocationDispatcher) QueuedCount() int {
	return int(atomic.LoadInt32(&d.queuedCount))
}

func (d *InvocationDispatcher) RejectNewInvocations() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.rejectNew = true
}

func (d *InvocationDispatcher) CancelQueued(reason string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	for _, inv := range d.invocations {
		if inv.Status == InvocationStatusQueued {
			inv.CancelSignal.Abort(reason)
		}
	}
}

func (d *InvocationDispatcher) WaitForRunning(ctx context.Context, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		if d.ActiveCount() == 0 {
			return true
		}
		if time.Now().After(deadline) {
			return false
		}
		select {
		case <-ctx.Done():
			return false
		case <-ticker.C:
		}
	}
}

func (d *InvocationDispatcher) ListActive() []*Invocation {
	d.mu.RLock()
	defer d.mu.RUnlock()
	result := make([]*Invocation, 0, len(d.invocations))
	for _, inv := range d.invocations {
		if inv.Status == InvocationStatusRunning {
			result = append(result, inv)
		}
	}
	return result
}
