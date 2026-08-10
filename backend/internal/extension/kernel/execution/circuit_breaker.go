package execution

import (
	"context"
	"sync"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/capability"
)

type CircuitKey struct {
	RuntimeType capability.RuntimeType
	RuntimeID   string
	ToolID      string
}

func (k CircuitKey) String() string {
	if k.RuntimeID != "" {
		return "runtime:" + string(k.RuntimeType) + ":" + k.RuntimeID
	}
	return "tool:" + k.ToolID
}

type CircuitState string

const (
	CircuitStateClosed   CircuitState = "closed"
	CircuitStateOpen     CircuitState = "open"
	CircuitStateHalfOpen CircuitState = "half_open"
)

type CircuitBreakerConfig struct {
	FailureThreshold        int
	OpenTimeout             time.Duration
	HalfOpenMaxInflight     int
	HalfOpenSuccessThreshold int
}

func DefaultCircuitBreakerConfig() CircuitBreakerConfig {
	return CircuitBreakerConfig{
		FailureThreshold:        5,
		OpenTimeout:             30 * time.Second,
		HalfOpenMaxInflight:     1,
		HalfOpenSuccessThreshold: 1,
	}
}

type CircuitPermit struct {
	Key    CircuitKey
	Allowed bool
	Probe   bool
}

type CircuitOutcome string

const (
	CircuitOutcomeSuccess CircuitOutcome = "success"
	CircuitOutcomeFailure CircuitOutcome = "failure"
	CircuitOutcomeNeutral CircuitOutcome = "neutral"
)

type CircuitFailureOrigin string

const (
	CircuitFailureProvider  CircuitFailureOrigin = "provider"
	CircuitFailureTransport CircuitFailureOrigin = "transport"
	CircuitFailureRuntime   CircuitFailureOrigin = "runtime"
	CircuitFailureKernel    CircuitFailureOrigin = "kernel"
)

type circuitBreaker struct {
	state               CircuitState
	consecutiveFailures int
	openedAt            time.Time
	lastFailureAt       time.Time
	lastSuccessAt       time.Time
	halfOpenInFlight    int
	halfOpenSuccesses   int
}

func (cb *circuitBreaker) snapshot(key CircuitKey, now time.Time) CircuitSnapshot {
	return CircuitSnapshot{
		Key:                 key.String(),
		State:               cb.state,
		ConsecutiveFailures: cb.consecutiveFailures,
		HalfOpenInFlight:    cb.halfOpenInFlight,
		HalfOpenSuccesses:   cb.halfOpenSuccesses,
		OpenedAt:            cb.openedAt,
		LastFailureAt:       cb.lastFailureAt,
		LastSuccessAt:       cb.lastSuccessAt,
	}
}

type CircuitSnapshot struct {
	Key                 string
	State               CircuitState
	ConsecutiveFailures int
	HalfOpenInFlight    int
	HalfOpenSuccesses   int
	OpenedAt            time.Time
	LastFailureAt       time.Time
	LastSuccessAt       time.Time
}

type CircuitClock interface {
	Now() time.Time
}

type systemClock struct{}

func (systemClock) Now() time.Time { return time.Now().UTC() }

type CircuitBreakerCoordinator struct {
	mu      sync.Mutex
	config  CircuitBreakerConfig
	clock   CircuitClock
	circuits map[CircuitKey]*circuitBreaker

	onEvent func(snapshot CircuitSnapshot, from, to CircuitState, reason string)
}

func NewCircuitBreakerCoordinator() *CircuitBreakerCoordinator {
	return NewCircuitBreakerCoordinatorWithConfig(DefaultCircuitBreakerConfig())
}

func NewCircuitBreakerCoordinatorWithConfig(config CircuitBreakerConfig) *CircuitBreakerCoordinator {
	return &CircuitBreakerCoordinator{
		config:   config,
		clock:    systemClock{},
		circuits: make(map[CircuitKey]*circuitBreaker),
	}
}

func NewCircuitBreakerCoordinatorWithClock(config CircuitBreakerConfig, clock CircuitClock) *CircuitBreakerCoordinator {
	if clock == nil {
		clock = systemClock{}
	}
	return &CircuitBreakerCoordinator{
		config:   config,
		clock:    clock,
		circuits: make(map[CircuitKey]*circuitBreaker),
	}
}

func (c *CircuitBreakerCoordinator) SetEventHook(fn func(snapshot CircuitSnapshot, from, to CircuitState, reason string)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.onEvent = fn
}

func resolveCircuitKey(tool capability.ToolDefinition) CircuitKey {
	return CircuitKey{
		RuntimeType: tool.Runtime.RuntimeType,
		RuntimeID:   tool.Runtime.RuntimeID,
		ToolID:      tool.ID,
	}
}

func (c *CircuitBreakerCoordinator) Acquire(ctx context.Context, tool capability.ToolDefinition) CircuitPermit {
	key := resolveCircuitKey(tool)

	c.mu.Lock()
	defer c.mu.Unlock()

	now := c.clock.Now()
	cb := c.circuits[key]
	if cb == nil {
		return CircuitPermit{Key: key, Allowed: true, Probe: false}
	}

	switch cb.state {
	case CircuitStateClosed:
		return CircuitPermit{Key: key, Allowed: true, Probe: false}
	case CircuitStateOpen:
		if now.Sub(cb.openedAt) < c.config.OpenTimeout {
			return CircuitPermit{Key: key, Allowed: false}
		}
		cb.state = CircuitStateHalfOpen
		cb.halfOpenInFlight = 0
		cb.halfOpenSuccesses = 0
		if c.onEvent != nil {
			c.onEvent(cb.snapshot(key, now), CircuitStateOpen, CircuitStateHalfOpen, "timeout_elapsed")
		}
		if c.config.HalfOpenMaxInflight <= 0 {
			return CircuitPermit{Key: key, Allowed: false}
		}
		cb.halfOpenInFlight++
		return CircuitPermit{Key: key, Allowed: true, Probe: true}
	case CircuitStateHalfOpen:
		if cb.halfOpenInFlight >= c.config.HalfOpenMaxInflight {
			return CircuitPermit{Key: key, Allowed: false}
		}
		cb.halfOpenInFlight++
		return CircuitPermit{Key: key, Allowed: true, Probe: true}
	}
	return CircuitPermit{Key: key, Allowed: false}
}

func (c *CircuitBreakerCoordinator) Complete(permit CircuitPermit, outcome CircuitOutcome) {
	if !permit.Allowed {
		return
	}
	key := permit.Key

	c.mu.Lock()
	defer c.mu.Unlock()

	now := c.clock.Now()
	cb := c.circuits[key]
	if cb == nil {
		return
	}

	switch outcome {
	case CircuitOutcomeSuccess:
		c.recordSuccess(cb, key, now)
	case CircuitOutcomeFailure:
		c.recordFailure(cb, key, now)
	case CircuitOutcomeNeutral:
		c.recordNeutral(cb)
	}
}

func (c *CircuitBreakerCoordinator) recordSuccess(cb *circuitBreaker, key CircuitKey, now time.Time) {
	from := cb.state
	cb.consecutiveFailures = 0
	cb.lastSuccessAt = now

	switch cb.state {
	case CircuitStateHalfOpen:
		cb.halfOpenInFlight--
		cb.halfOpenSuccesses++
		if cb.halfOpenSuccesses >= c.config.HalfOpenSuccessThreshold {
			cb.state = CircuitStateClosed
			cb.halfOpenInFlight = 0
			cb.halfOpenSuccesses = 0
			c.deleteIfClean(key, cb)
			if c.onEvent != nil {
				c.onEvent(cb.snapshot(key, now), from, CircuitStateClosed, "recovery_success")
			}
		}
	case CircuitStateClosed:
		c.deleteIfClean(key, cb)
	}
}

func (c *CircuitBreakerCoordinator) recordFailure(cb *circuitBreaker, key CircuitKey, now time.Time) {
	from := cb.state
	cb.consecutiveFailures++
	cb.lastFailureAt = now
	cb.halfOpenInFlight = 0
	cb.halfOpenSuccesses = 0

	switch cb.state {
	case CircuitStateHalfOpen:
		cb.state = CircuitStateOpen
		cb.openedAt = now
		if c.onEvent != nil {
			c.onEvent(cb.snapshot(key, now), from, CircuitStateOpen, "probe_failure")
		}
	case CircuitStateClosed:
		if cb.consecutiveFailures >= c.config.FailureThreshold {
			cb.state = CircuitStateOpen
			cb.openedAt = now
			if c.onEvent != nil {
				c.onEvent(cb.snapshot(key, now), from, CircuitStateOpen, "threshold_reached")
			}
		}
	}
}

func (c *CircuitBreakerCoordinator) recordNeutral(cb *circuitBreaker) {
	if cb.halfOpenInFlight > 0 {
		cb.halfOpenInFlight--
	}
}

func (c *CircuitBreakerCoordinator) deleteIfClean(key CircuitKey, cb *circuitBreaker) {
	if cb != nil &&
		cb.state == CircuitStateClosed &&
		cb.consecutiveFailures == 0 &&
		cb.halfOpenInFlight == 0 &&
		cb.halfOpenSuccesses == 0 {
		delete(c.circuits, key)
	}
}

func (c *CircuitBreakerCoordinator) Reset(key CircuitKey) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.circuits, key)
}

func (c *CircuitBreakerCoordinator) ResetTool(tool capability.ToolDefinition) {
	c.Reset(resolveCircuitKey(tool))
}

func (c *CircuitBreakerCoordinator) Snapshot(tool capability.ToolDefinition) CircuitSnapshot {
	key := resolveCircuitKey(tool)
	return c.SnapshotByKey(key)
}

func (c *CircuitBreakerCoordinator) RecordResult(ctx context.Context, tool capability.ToolDefinition, result capability.UnifiedToolResult) {
	permit := c.Acquire(ctx, tool)
	if !permit.Allowed {
		return
	}
	outcome := CircuitOutcomeNeutral
	switch result.Status {
	case capability.ToolResultStatusSuccess:
		outcome = CircuitOutcomeSuccess
	case capability.ToolResultStatusFailed, capability.ToolResultStatusCancelled, capability.ToolResultStatusTimedOut:
		outcome = CircuitOutcomeFailure
	}
	c.Complete(permit, outcome)
}

func (c *CircuitBreakerCoordinator) SnapshotByKey(key CircuitKey) CircuitSnapshot {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := c.clock.Now()
	zero := CircuitSnapshot{State: CircuitStateClosed}

	cb := c.circuits[key]
	if cb == nil {
		return zero
	}

	if cb.state == CircuitStateOpen && now.Sub(cb.openedAt) >= c.config.OpenTimeout {
		zero.State = CircuitStateHalfOpen
		return zero
	}
	return cb.snapshot(key, now)
}

func (c *CircuitBreakerCoordinator) HasCircuit(tool capability.ToolDefinition) bool {
	key := resolveCircuitKey(tool)
	c.mu.Lock()
	defer c.mu.Unlock()
	_, ok := c.circuits[key]
	return ok
}
