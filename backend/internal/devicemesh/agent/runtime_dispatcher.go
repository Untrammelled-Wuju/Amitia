package agent

import (
	"context"
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

type defaultRuntimeDispatcher struct {
	handlers            map[string]RuntimeInvokeHandler
	cancellableHandlers map[string]CancellableRuntimeInvokeHandler
	mu                  sync.Mutex
	cancels             map[string]context.CancelFunc
}

func NewRuntimeDispatcher() *defaultRuntimeDispatcher {
	return &defaultRuntimeDispatcher{
		handlers:            make(map[string]RuntimeInvokeHandler),
		cancellableHandlers: make(map[string]CancellableRuntimeInvokeHandler),
		cancels:             make(map[string]context.CancelFunc),
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
	if handler := d.cancellableHandlers[handlerName]; handler != nil {
		return func(invoke protocol.RuntimeInvokePayload) (*protocol.RuntimeResultPayload, error) {
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
	}
	return d.handlers[handlerName]
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
