package jsonrpc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"
)

type HandlerFunc func(ctx context.Context, params json.RawMessage) (any, error)

type NotificationHandlerFunc func(ctx context.Context, params json.RawMessage) error

type MethodRegistry struct {
	mu            sync.RWMutex
	methods       map[string]HandlerFunc
	notifications map[string]NotificationHandlerFunc
	middlewares   []Middleware
}

type Middleware func(ctx context.Context, params json.RawMessage, next HandlerFunc) (any, error)

func NewMethodRegistry() *MethodRegistry {
	return &MethodRegistry{
		methods:       make(map[string]HandlerFunc),
		notifications: make(map[string]NotificationHandlerFunc),
	}
}

func (r *MethodRegistry) Register(method string, h HandlerFunc) {
	r.mu.Lock()
	r.methods[method] = h
	r.mu.Unlock()
}

func (r *MethodRegistry) RegisterNotification(method string, h NotificationHandlerFunc) {
	r.mu.Lock()
	r.notifications[method] = h
	r.mu.Unlock()
}

func (r *MethodRegistry) Use(mw Middleware) {
	r.mu.Lock()
	r.middlewares = append(r.middlewares, mw)
	r.mu.Unlock()
}

func (r *MethodRegistry) Lookup(method string) (HandlerFunc, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	h, ok := r.methods[method]
	return h, ok
}

func (r *MethodRegistry) LookupNotification(method string) (NotificationHandlerFunc, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	h, ok := r.notifications[method]
	return h, ok
}

func (r *MethodRegistry) Dispatch(ctx context.Context, method string, params json.RawMessage) (any, error) {
	r.mu.RLock()
	h, ok := r.methods[method]
	middlewares := make([]Middleware, len(r.middlewares))
	copy(middlewares, r.middlewares)
	r.mu.RUnlock()
	if !ok {
		return nil, MethodNotFoundError(method)
	}
	current := h
	for i := len(middlewares) - 1; i >= 0; i-- {
		mw := middlewares[i]
		next := current
		current = func(ctx context.Context, p json.RawMessage) (any, error) {
			return mw(ctx, p, next)
		}
	}
	return current(ctx, params)
}

func (r *MethodRegistry) DispatchNotification(ctx context.Context, method string, params json.RawMessage) error {
	r.mu.RLock()
	h, ok := r.notifications[method]
	r.mu.RUnlock()
	if !ok {
		return MethodNotFoundError(method)
	}
	return h(ctx, params)
}

func (r *MethodRegistry) Methods() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]string, 0, len(r.methods))
	for k := range r.methods {
		out = append(out, k)
	}
	return out
}

func (r *MethodRegistry) Notifications() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]string, 0, len(r.notifications))
	for k := range r.notifications {
		out = append(out, k)
	}
	return out
}

type RateLimiter struct {
	mu        sync.Mutex
	bucket    map[string][]time.Time
	maxPerSec int
	window    time.Duration
}

func NewRateLimiter(maxPerSec int) *RateLimiter {
	if maxPerSec <= 0 {
		maxPerSec = 100
	}
	return &RateLimiter{
		bucket:    make(map[string][]time.Time),
		maxPerSec: maxPerSec,
		window:    time.Second,
	}
}

func (rl *RateLimiter) Allow(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	now := time.Now()
	cutoff := now.Add(-rl.window)
	history := rl.bucket[key]
	pruned := history[:0]
	for _, t := range history {
		if t.After(cutoff) {
			pruned = append(pruned, t)
		}
	}
	if len(pruned) >= rl.maxPerSec {
		rl.bucket[key] = pruned
		return false
	}
	rl.bucket[key] = append(pruned, now)
	return true
}

func (rl *RateLimiter) Reset(key string) {
	rl.mu.Lock()
	delete(rl.bucket, key)
	rl.mu.Unlock()
}

func (rl *RateLimiter) Stats(key string) (count int, max int) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	cutoff := time.Now().Add(-rl.window)
	count = 0
	for _, t := range rl.bucket[key] {
		if t.After(cutoff) {
			count++
		}
	}
	return count, rl.maxPerSec
}

type NotificationDispatcher struct {
	registry *MethodRegistry
	limiter  *RateLimiter
}

func NewNotificationDispatcher(registry *MethodRegistry, limiter *RateLimiter) *NotificationDispatcher {
	return &NotificationDispatcher{registry: registry, limiter: limiter}
}

func (d *NotificationDispatcher) Handle(ctx context.Context, n *Notification) error {
	if d.limiter != nil && !d.limiter.Allow(n.Method) {
		return NewError(
			ErrCodeResourceExhausted,
			fmt.Sprintf("notification rate limit exceeded for %s", n.Method),
			true,
			CategoryResource,
		)
	}
	return d.registry.DispatchNotification(ctx, n.Method, n.Params)
}

type CallContext struct {
	Session        *Session
	InvocationID   string
	Deadline       time.Time
	CancelSignal   context.Context
	StreamID       *StreamID
	IdempotencyKey string
	Trace          map[string]string
	Metadata       map[string]any
}

func (c *CallContext) IsCancelled() bool {
	if c.CancelSignal == nil {
		return false
	}
	select {
	case <-c.CancelSignal.Done():
		return true
	default:
		return false
	}
}

var (
	ErrSessionExpired = errors.New("jsonrpc: session expired")
	ErrUnauthorized   = errors.New("jsonrpc: unauthorized")
)

func ValidateSession(session *Session) error {
	if session == nil {
		return ErrUnauthorized
	}
	if session.IsClosed() {
		return ErrSessionExpired
	}
	if session.IsExpired() {
		return ErrSessionExpired
	}
	return nil
}

type SessionMiddleware struct {
	session *Session
}

func NewSessionMiddleware(session *Session) *SessionMiddleware {
	return &SessionMiddleware{session: session}
}

func (m *SessionMiddleware) Wrap() Middleware {
	return func(ctx context.Context, params json.RawMessage, next HandlerFunc) (any, error) {
		if err := ValidateSession(m.session); err != nil {
			return nil, PermissionDeniedError(err.Error())
		}
		m.session.Touch()
		return next(ctx, params)
	}
}

type CancelParams struct {
	TargetRequestID string `json:"target_request_id"`
	Reason          string `json:"reason"`
}

type HealthRequest struct {
	IncludeDetails bool `json:"include_details,omitempty"`
}

type HealthResponse struct {
	Healthy      bool           `json:"healthy"`
	InstanceID   string         `json:"instance_id"`
	Generation   int64          `json:"generation"`
	Now          time.Time      `json:"now"`
	Uptime       time.Duration  `json:"uptime"`
	ActiveCalls  int            `json:"active_calls"`
	QueueDepth   int            `json:"queue_depth"`
	MemoryMB     int64          `json:"memory_mb,omitempty"`
	EventLoopLag time.Duration  `json:"event_loop_lag,omitempty"`
	Details      map[string]any `json:"details,omitempty"`
}

type ShutdownRequest struct {
	Reason string        `json:"reason"`
	Grace  time.Duration `json:"grace"`
}

type ShutdownAck struct {
	Accepted bool `json:"accepted"`
}
