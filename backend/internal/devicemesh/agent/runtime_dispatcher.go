package agent

import (
	"context"
	"fmt"
	"sync"
	"time"

	protocol "github.com/u-ai/backend/internal/deviceruntime/protocol"
)

type RuntimeInvokeHandler func(invoke protocol.RuntimeInvokePayload) (*protocol.RuntimeResultPayload, error)

// CancellableRuntimeInvokeHandler is used for device-local handlers that can
// honor Device Mesh runtime.cancel and caller deadlines. The dispatcher owns
// the invocation context so cancellation is centralized instead of being
// reimplemented by individual handlers.
type CancellableRuntimeInvokeHandler func(ctx context.Context, invoke protocol.RuntimeInvokePayload) (*protocol.RuntimeResultPayload, error)

type RuntimeDispatcher interface {
	Resolve(handlerName string) RuntimeInvokeHandler
}

type RuntimeCancelDispatcher interface {
	CancelInvocation(invocationID string) bool
}

// RuntimeDisconnectDispatcher is implemented by dispatchers that can stop all
// active work when the Device Mesh control channel is lost. This prevents a
// detached device-side invocation from continuing after Cloud has already
// expired its lease and reassigned the workflow node.
type RuntimeDisconnectDispatcher interface {
	CancelAllInvocations(reason string) int
}

type cachedRuntimeInvokeResult struct {
	result    protocol.RuntimeResultPayload
	expiresAt time.Time
}

type runtimeInvokeFlight struct {
	done   chan struct{}
	result *protocol.RuntimeResultPayload
	err    error
}

type runtimeFenceOwner struct {
	token        int64
	invocationID string
}

type defaultRuntimeDispatcher struct {
	handlers            map[string]RuntimeInvokeHandler
	cancellableHandlers map[string]CancellableRuntimeInvokeHandler
	mu                  sync.Mutex
	cancels             map[string]context.CancelFunc
	completed           map[string]cachedRuntimeInvokeResult
	inflight            map[string]*runtimeInvokeFlight
	fencing             map[string]int64
	fenceOwners         map[string]runtimeFenceOwner
}

func NewRuntimeDispatcher() *defaultRuntimeDispatcher {
	return &defaultRuntimeDispatcher{
		handlers:            make(map[string]RuntimeInvokeHandler),
		cancellableHandlers: make(map[string]CancellableRuntimeInvokeHandler),
		cancels:             make(map[string]context.CancelFunc),
		completed:           make(map[string]cachedRuntimeInvokeResult),
		inflight:            make(map[string]*runtimeInvokeFlight),
		fencing:             make(map[string]int64),
		fenceOwners:         make(map[string]runtimeFenceOwner),
	}
}

func (d *defaultRuntimeDispatcher) Register(handlerName string, handler RuntimeInvokeHandler) {
	if d == nil || handlerName == "" || handler == nil {
		return
	}
	d.handlers[handlerName] = handler
}

func (d *defaultRuntimeDispatcher) RegisterCancellable(handlerName string, handler CancellableRuntimeInvokeHandler) {
	if d == nil || handlerName == "" || handler == nil {
		return
	}
	d.cancellableHandlers[handlerName] = handler
}

func (d *defaultRuntimeDispatcher) Resolve(handlerName string) RuntimeInvokeHandler {
	if d == nil {
		return nil
	}
	var base RuntimeInvokeHandler
	if handler := d.cancellableHandlers[handlerName]; handler != nil {
		base = func(invoke protocol.RuntimeInvokePayload) (*protocol.RuntimeResultPayload, error) {
			ctx := context.Background()
			var cancel context.CancelFunc
			if invoke.DeadlineMs > 0 {
				ctx, cancel = context.WithTimeout(ctx, time.Duration(invoke.DeadlineMs)*time.Millisecond)
			} else {
				ctx, cancel = context.WithCancel(ctx)
			}
			d.mu.Lock()
			d.cancels[invoke.InvocationID] = cancel
			d.mu.Unlock()
			defer func() {
				cancel()
				d.mu.Lock()
				delete(d.cancels, invoke.InvocationID)
				d.mu.Unlock()
			}()
			return handler(ctx, invoke)
		}
	} else {
		base = d.handlers[handlerName]
	}
	if base == nil {
		return nil
	}
	return d.withReliability(base)
}

func (d *defaultRuntimeDispatcher) withReliability(handler RuntimeInvokeHandler) RuntimeInvokeHandler {
	return func(invoke protocol.RuntimeInvokePayload) (*protocol.RuntimeResultPayload, error) {
		now := time.Now().UTC()
		idemKey := invoke.IdempotencyKey
		fenceKey := ""
		if invoke.WorkflowRunID != "" && invoke.WorkflowNodeID != "" && invoke.FencingToken > 0 {
			fenceKey = invoke.WorkflowRunID + "\x00" + invoke.WorkflowNodeID
		}

		d.mu.Lock()
		var supersededCancel context.CancelFunc
		for key, cached := range d.completed {
			if !cached.expiresAt.After(now) {
				delete(d.completed, key)
			}
		}
		if fenceKey != "" {
			current := d.fencing[fenceKey]
			if current > invoke.FencingToken {
				d.mu.Unlock()
				return nil, fmt.Errorf("workflow stale fencing token: current=%d received=%d", current, invoke.FencingToken)
			}
			if invoke.FencingToken > current {
				if previous := d.fenceOwners[fenceKey]; previous.invocationID != "" && previous.token < invoke.FencingToken {
					supersededCancel = d.cancels[previous.invocationID]
				}
				d.fencing[fenceKey] = invoke.FencingToken
				d.fenceOwners[fenceKey] = runtimeFenceOwner{token: invoke.FencingToken, invocationID: invoke.InvocationID}
			} else if owner := d.fenceOwners[fenceKey]; owner.invocationID == "" {
				d.fenceOwners[fenceKey] = runtimeFenceOwner{token: invoke.FencingToken, invocationID: invoke.InvocationID}
			}
		}
		if idemKey != "" {
			if cached, ok := d.completed[idemKey]; ok && cached.expiresAt.After(now) {
				result := cached.result
				d.mu.Unlock()
				return &result, nil
			}
			if flight, ok := d.inflight[idemKey]; ok && flight != nil {
				d.mu.Unlock()
				if invoke.DeadlineMs > 0 {
					timer := time.NewTimer(time.Duration(invoke.DeadlineMs) * time.Millisecond)
					defer timer.Stop()
					select {
					case <-flight.done:
					case <-timer.C:
						return nil, fmt.Errorf("duplicate invocation wait timed out")
					}
				} else {
					<-flight.done
				}
				if flight.result != nil {
					result := *flight.result
					return &result, flight.err
				}
				return nil, flight.err
			}
			d.inflight[idemKey] = &runtimeInvokeFlight{done: make(chan struct{})}
		}
		d.mu.Unlock()
		if supersededCancel != nil {
			supersededCancel()
		}

		result, err := handler(invoke)
		if result != nil {
			result.IdempotencyKey = invoke.IdempotencyKey
			result.FencingToken = invoke.FencingToken
		}

		d.mu.Lock()
		if fenceKey != "" && d.fencing[fenceKey] != invoke.FencingToken {
			err = fmt.Errorf("workflow stale fencing token after execution: current=%d received=%d", d.fencing[fenceKey], invoke.FencingToken)
			result = nil
		}
		if idemKey != "" {
			if flight := d.inflight[idemKey]; flight != nil {
				flight.result = result
				flight.err = err
				delete(d.inflight, idemKey)
				close(flight.done)
			}
			if err == nil && result != nil && result.Status == "success" {
				d.completed[idemKey] = cachedRuntimeInvokeResult{result: *result, expiresAt: now.Add(24 * time.Hour)}
			}
		}
		d.mu.Unlock()
		return result, err
	}
}

func (d *defaultRuntimeDispatcher) CancelInvocation(invocationID string) bool {
	if d == nil || invocationID == "" {
		return false
	}
	d.mu.Lock()
	cancel, ok := d.cancels[invocationID]
	d.mu.Unlock()
	if !ok || cancel == nil {
		return false
	}
	cancel()
	return true
}

func (d *defaultRuntimeDispatcher) CancelAllInvocations(_ string) int {
	if d == nil {
		return 0
	}
	d.mu.Lock()
	cancels := make([]context.CancelFunc, 0, len(d.cancels))
	for _, cancel := range d.cancels {
		if cancel != nil {
			cancels = append(cancels, cancel)
		}
	}
	d.mu.Unlock()
	for _, cancel := range cancels {
		cancel()
	}
	return len(cancels)
}

type chainedRuntimeDispatcher struct {
	primary  RuntimeDispatcher
	fallback RuntimeDispatcher
}

func NewChainedRuntimeDispatcher(primary RuntimeDispatcher, fallback RuntimeDispatcher) RuntimeDispatcher {
	return &chainedRuntimeDispatcher{primary: primary, fallback: fallback}
}

func (d *chainedRuntimeDispatcher) Resolve(handlerName string) RuntimeInvokeHandler {
	if d == nil {
		return nil
	}
	if d.primary != nil {
		if handler := d.primary.Resolve(handlerName); handler != nil {
			return handler
		}
	}
	if d.fallback != nil {
		return d.fallback.Resolve(handlerName)
	}
	return nil
}

func (d *chainedRuntimeDispatcher) CancelInvocation(invocationID string) bool {
	if d == nil {
		return false
	}
	cancelled := false
	if primary, ok := d.primary.(RuntimeCancelDispatcher); ok && primary != nil {
		cancelled = primary.CancelInvocation(invocationID) || cancelled
	}
	if fallback, ok := d.fallback.(RuntimeCancelDispatcher); ok && fallback != nil {
		cancelled = fallback.CancelInvocation(invocationID) || cancelled
	}
	return cancelled
}
func (d *chainedRuntimeDispatcher) CancelAllInvocations(reason string) int {
	if d == nil {
		return 0
	}
	cancelled := 0
	if primary, ok := d.primary.(RuntimeDisconnectDispatcher); ok && primary != nil {
		cancelled += primary.CancelAllInvocations(reason)
	}
	if fallback, ok := d.fallback.(RuntimeDisconnectDispatcher); ok && fallback != nil {
		cancelled += fallback.CancelAllInvocations(reason)
	}
	return cancelled
}
