package execution

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/capability"
)

func NewConcurrencyController() *ConcurrencyController {
	return &ConcurrencyController{
		globalSem: make(chan struct{}, 100),
		toolSem:   make(map[string]chan struct{}),
		extSem:    make(map[string]chan struct{}),
		mu:        sync.Mutex{},
	}
}

type ConcurrencyController struct {
	globalSem chan struct{}
	toolSem   map[string]chan struct{}
	extSem    map[string]chan struct{}
	mu        sync.Mutex
	Policy    ConcurrencyPolicy
}

type ConcurrencyPolicy struct {
	GlobalLimit          int
	PerToolLimit         int
	PerExtensionLimit    int
	PerCharacterLimit    int
	PerConversationLimit int
}

type concurrencySlot struct {
	Global   bool
	Tool     string
	Ext      string
	Released bool
	ctrl     *ConcurrencyController
}

func (c *ConcurrencyController) Acquire(ctx context.Context, tool capability.ToolDefinition, inv capability.ToolInvocationContext) (*concurrencySlot, error) {
	slot := &concurrencySlot{ctrl: c}

	select {
	case c.globalSem <- struct{}{}:
		slot.Global = true
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	if c.Policy.PerToolLimit > 0 {
		toolKey := tool.ID
		c.mu.Lock()
		if _, ok := c.toolSem[toolKey]; !ok {
			c.toolSem[toolKey] = make(chan struct{}, c.Policy.PerToolLimit)
		}
		sem := c.toolSem[toolKey]
		c.mu.Unlock()
		select {
		case sem <- struct{}{}:
			slot.Tool = toolKey
		case <-ctx.Done():
			c.releaseSlot(slot)
			return nil, ctx.Err()
		}
	}

	return slot, nil
}

func (c *ConcurrencyController) Release(slot *concurrencySlot) {
	if slot == nil || slot.Released {
		return
	}
	c.releaseSlot(slot)
}

func (c *ConcurrencyController) releaseSlot(slot *concurrencySlot) {
	if slot.Released {
		return
	}
	slot.Released = true
	if slot.Global {
		<-c.globalSem
	}
	if slot.Tool != "" {
		c.mu.Lock()
		if sem, ok := c.toolSem[slot.Tool]; ok {
			select {
			case <-sem:
			default:
			}
		}
		c.mu.Unlock()
	}
}

func NewRateLimiter() *RateLimiter {
	return &RateLimiter{}
}

type RateLimiter struct {
	OnAllow func(ctx context.Context, tool capability.ToolDefinition) error
}

func (r *RateLimiter) Allow(ctx context.Context, tool capability.ToolDefinition) error {
	if r.OnAllow != nil {
		return r.OnAllow(ctx, tool)
	}
	return nil
}

func NewIdempotencyGuard() *IdempotencyGuard {
	return &IdempotencyGuard{
		store: make(map[string]capability.UnifiedToolResult),
	}
}

type IdempotencyGuard struct {
	store map[string]capability.UnifiedToolResult
	mu    sync.RWMutex
}

func (g *IdempotencyGuard) Check(ctx context.Context, key, toolID string) (capability.UnifiedToolResult, bool) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	result, ok := g.store[key]
	return result, ok
}

func (g *IdempotencyGuard) Record(ctx context.Context, key, toolID string, result *capability.UnifiedToolResult) {
	if result == nil {
		return
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	if _, exists := g.store[key]; !exists {
		g.store[key] = *result
	}
}

func NewRetryController() *RetryController {
	return &RetryController{}
}

type RetryController struct {
	OnShouldRetry func(ctx context.Context, tool capability.ToolDefinition, result capability.UnifiedToolResult) (bool, error)
}

func (r *RetryController) ShouldRetry(ctx context.Context, tool capability.ToolDefinition, result capability.UnifiedToolResult) (bool, error) {
	if tool.ExecutionPolicy.RetryPolicy.MaxRetries == 0 {
		return false, nil
	}
	if result.Error == nil {
		return false, nil
	}
	if !result.Error.Retryable && !tool.Retryable {
		return false, fmt.Errorf("not retryable")
	}
	if result.Status == capability.ToolResultStatusCancelled {
		return false, fmt.Errorf("cancelled")
	}
	if r.OnShouldRetry != nil {
		return r.OnShouldRetry(ctx, tool, result)
	}
	return result.Error.Retryable, nil
}

func (r *RetryController) Backoff(attempt int) time.Duration {
	base := 100 * time.Millisecond
	return base * time.Duration(1<<uint(attempt-1))
}

func NewTimeoutController() *TimeoutController {
	return &TimeoutController{}
}

type TimeoutController struct{}

func (t *TimeoutController) WithTimeout(ctx context.Context, tool capability.ToolDefinition, inv capability.ToolInvocationContext) (context.Context, context.CancelFunc) {
	if tool.TimeoutMS > 0 {
		return context.WithTimeout(ctx, time.Duration(tool.TimeoutMS)*time.Millisecond)
	}
	if !inv.ExpiresAt.IsZero() {
		deadline := time.Until(inv.ExpiresAt)
		if deadline > 0 {
			return context.WithTimeout(ctx, deadline)
		}
	}
	return context.WithTimeout(ctx, 30*time.Second)
}

func NewCancellationController() *CancellationController {
	return &CancellationController{}
}

type CancellationController struct{}

func (c *CancellationController) Wrap(ctx context.Context, inv capability.ToolInvocationContext) context.Context {
	return ctx
}

func NewDepthGuard() *DepthGuard {
	return &DepthGuard{MaxDepth: 10}
}

type DepthGuard struct {
	MaxDepth int
}

func (g *DepthGuard) Check(ctx context.Context, inv capability.ToolInvocationContext) error {
	if inv.ParentID != "" {
		chain := g.getChain(ctx, inv)
		if len(chain) >= g.MaxDepth {
			return fmt.Errorf("max call depth %d exceeded", g.MaxDepth)
		}
	}
	return nil
}

func (g *DepthGuard) getChain(ctx context.Context, inv capability.ToolInvocationContext) []string {
	chain := []string{inv.InvocationID}
	if inv.ParentID != "" {
		chain = append([]string{inv.ParentID}, chain...)
	}
	return chain
}

func NewRuntimeDispatcher(adapterRegistry *capability.RuntimeAdapterRegistry) *RuntimeDispatcher {
	return &RuntimeDispatcher{adapterRegistry: adapterRegistry}
}

type RuntimeDispatcher struct {
	adapterRegistry *capability.RuntimeAdapterRegistry
}

func (d *RuntimeDispatcher) Dispatch(ctx context.Context, tool capability.ToolDefinition, inv capability.ToolInvocationContext, input json.RawMessage) capability.UnifiedToolResult {
	if d.adapterRegistry == nil {
		return capability.UnifiedToolResult{
			InvocationID: inv.InvocationID,
			Status:       capability.ToolResultStatusFailed,
			Error: &capability.ToolError{
				Code:    capability.ErrorCodeRuntimeUnavailable,
				Message: "no adapter registry configured",
			},
		}
	}

	adapter, ok := d.adapterRegistry.Resolve(tool.Runtime)
	if !ok {
		return capability.UnifiedToolResult{
			InvocationID: inv.InvocationID,
			Status:       capability.ToolResultStatusFailed,
			Error: &capability.ToolError{
				Code:    capability.ErrorCodeRuntimeUnavailable,
				Message: "no adapter for runtime: " + string(tool.Runtime.RuntimeType),
			},
		}
	}

	return adapter.Execute(ctx, tool.Runtime, inv, input)
}
