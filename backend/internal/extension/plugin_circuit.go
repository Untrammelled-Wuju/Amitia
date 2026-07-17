package extension

import (
	"sync"
	"time"
)

type pluginCircuit struct {
	mu               sync.Mutex
	state            CircuitState
	failures         int
	halfOpenInFlight int
	threshold        int
	openDuration     time.Duration
	openedAt         time.Time
}

func newPluginCircuit(threshold int, openDuration time.Duration) *pluginCircuit {
	return &pluginCircuit{state: CircuitClosed, threshold: threshold, openDuration: openDuration}
}

func (c *pluginCircuit) Allow(now time.Time) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.state == CircuitOpen {
		if now.Sub(c.openedAt) < c.openDuration {
			return false
		}
		c.state = CircuitHalfOpen
		c.halfOpenInFlight = 0
	}
	if c.state == CircuitHalfOpen {
		if c.halfOpenInFlight >= 1 {
			return false
		}
		c.halfOpenInFlight++
	}
	return true
}

func (c *pluginCircuit) Success() (CircuitState, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	changed := c.state != CircuitClosed
	c.state = CircuitClosed
	c.failures = 0
	c.halfOpenInFlight = 0
	return c.state, changed
}

func (c *pluginCircuit) Failure(now time.Time) (CircuitState, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	previous := c.state
	if c.state == CircuitHalfOpen {
		c.state = CircuitOpen
		c.openedAt = now
		c.halfOpenInFlight = 0
		return c.state, previous != c.state
	}
	c.failures++
	if c.failures >= c.threshold {
		c.state = CircuitOpen
		c.openedAt = now
	}
	return c.state, previous != c.state
}

func (c *pluginCircuit) Reset() {
	c.mu.Lock()
	c.state = CircuitClosed
	c.failures = 0
	c.halfOpenInFlight = 0
	c.openedAt = time.Time{}
	c.mu.Unlock()
}

func (c *pluginCircuit) View(now time.Time) CircuitView {
	c.mu.Lock()
	defer c.mu.Unlock()
	state := c.state
	if state == CircuitOpen && now.Sub(c.openedAt) >= c.openDuration {
		state = CircuitHalfOpen
	}
	view := CircuitView{State: state, Failures: c.failures}
	if !c.openedAt.IsZero() {
		view.OpenedAt = c.openedAt.UTC().Format(time.RFC3339Nano)
		view.NextProbeAt = c.openedAt.Add(c.openDuration).UTC().Format(time.RFC3339Nano)
	}
	return view
}
