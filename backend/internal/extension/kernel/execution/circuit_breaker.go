package execution

import (
	"context"
	"sync"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/capability"
)

type CircuitState string

const (
	CircuitClosed   CircuitState = "closed"
	CircuitOpen     CircuitState = "open"
	CircuitHalfOpen CircuitState = "half_open"
)

func NewCircuitBreakerCoordinator() *CircuitBreakerCoordinator {
	return &CircuitBreakerCoordinator{
		circuits:  make(map[string]*CircuitBreaker),
		FailThreshold: 5,
		ResetTimeout:  30 * time.Second,
	}
}

type CircuitBreaker struct {
	State         CircuitState
	FailCount     int
	LastFailTime  time.Time
	LastSuccess   time.Time
	HalfOpenCount int
}

type CircuitBreakerCoordinator struct {
	circuits      map[string]*CircuitBreaker
	mu            sync.RWMutex
	FailThreshold int
	ResetTimeout  time.Duration
	MaxHalfOpen   int
}

func (c *CircuitBreakerCoordinator) Allow(ctx context.Context, toolID string) bool {
	c.mu.RLock()
	cb, ok := c.circuits[toolID]
	c.mu.RUnlock()

	if !ok {
		return true
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if cb.State == CircuitClosed {
		return true
	}

	if cb.State == CircuitOpen {
		if time.Since(cb.LastFailTime) > c.ResetTimeout {
			cb.State = CircuitHalfOpen
			cb.HalfOpenCount = 0
			return true
		}
		return false
	}

	if cb.State == CircuitHalfOpen {
		if c.MaxHalfOpen > 0 && cb.HalfOpenCount >= c.MaxHalfOpen {
			return false
		}
		cb.HalfOpenCount++
		return true
	}

	return true
}

func (c *CircuitBreakerCoordinator) RecordResult(ctx context.Context, tool capability.ToolDefinition, result capability.UnifiedToolResult) {
	toolID := tool.ID

	c.mu.Lock()
	defer c.mu.Unlock()

	cb, ok := c.circuits[toolID]
	if !ok {
		cb = &CircuitBreaker{State: CircuitClosed}
		c.circuits[toolID] = cb
	}

	if result.Status == capability.ToolResultStatusSuccess {
		cb.FailCount = 0
		cb.LastSuccess = time.Now()
		if cb.State == CircuitHalfOpen {
			cb.State = CircuitClosed
		}
		return
	}

	if result.Status == capability.ToolResultStatusFailed || result.Status == capability.ToolResultStatusTimedOut {
		cb.FailCount++
		cb.LastFailTime = time.Now()
		if cb.FailCount >= c.FailThreshold && cb.State == CircuitClosed {
			cb.State = CircuitOpen
		}
		if cb.State == CircuitHalfOpen {
			cb.State = CircuitOpen
			cb.FailCount = c.FailThreshold
		}
	}
}

func (c *CircuitBreakerCoordinator) GetState(toolID string) CircuitState {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if cb, ok := c.circuits[toolID]; ok {
		return cb.State
	}
	return CircuitClosed
}

func (c *CircuitBreakerCoordinator) Reset(toolID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.circuits, toolID)
}
