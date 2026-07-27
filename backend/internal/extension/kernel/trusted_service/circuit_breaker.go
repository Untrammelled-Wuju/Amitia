package trusted_service

import (
	"errors"
	"fmt"
	"sync"
	"time"
)

type CircuitState string

const (
	CircuitClosed   CircuitState = "closed"
	CircuitOpen     CircuitState = "open"
	CircuitHalfOpen CircuitState = "half_open"
)

type CircuitConfig struct {
	FailureThreshold  int
	RecoveryAfter     time.Duration
	HalfOpenAttempts  int
	ResetAfterHealthy time.Duration
}

func DefaultCircuitConfig() CircuitConfig {
	return CircuitConfig{
		FailureThreshold:  5,
		RecoveryAfter:     60 * time.Second,
		HalfOpenAttempts:  1,
		ResetAfterHealthy: 120 * time.Second,
	}
}

type CircuitBreaker struct {
	mu               sync.Mutex
	state            CircuitState
	failureCount     int
	successCount     int
	lastFailure      time.Time
	lastSuccess      time.Time
	config           CircuitConfig
	openedAt         time.Time
	halfOpenAttempts int
}

func NewCircuitBreaker(config CircuitConfig) *CircuitBreaker {
	return &CircuitBreaker{
		state:  CircuitClosed,
		config: config,
	}
}

func (c *CircuitBreaker) State() CircuitState {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.evaluateTransition()
	return c.state
}

func (c *CircuitBreaker) RecordSuccess() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.lastSuccess = time.Now()
	c.successCount++

	if c.state == CircuitHalfOpen {
		if c.successCount >= c.config.HalfOpenAttempts {
			c.state = CircuitClosed
			c.failureCount = 0
			c.successCount = 0
			c.halfOpenAttempts = 0
		}
	} else if c.state == CircuitClosed {
		if time.Since(c.lastFailure) > c.config.ResetAfterHealthy {
			c.failureCount = 0
		}
	}
}

func (c *CircuitBreaker) RecordFailure() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.failureCount++
	c.lastFailure = time.Now()
	c.successCount = 0

	if c.state == CircuitHalfOpen {
		c.state = CircuitOpen
		c.openedAt = time.Now()
		c.halfOpenAttempts = 0
	} else if c.state == CircuitClosed {
		if c.failureCount >= c.config.FailureThreshold {
			c.state = CircuitOpen
			c.openedAt = time.Now()
		}
	}
}

func (c *CircuitBreaker) AllowStart() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.evaluateTransition()
	return c.state != CircuitOpen
}

func (c *CircuitBreaker) Reset() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.state = CircuitClosed
	c.failureCount = 0
	c.successCount = 0
	c.halfOpenAttempts = 0
}

func (c *CircuitBreaker) Status() CircuitState {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.evaluateTransition()
	return c.state
}

func (c *CircuitBreaker) FailureCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.failureCount
}

func (c *CircuitBreaker) evaluateTransition() {
	if c.state == CircuitOpen {
		if time.Since(c.openedAt) >= c.config.RecoveryAfter {
			c.state = CircuitHalfOpen
			c.halfOpenAttempts = 0
			c.successCount = 0
		}
	}
}

func (c *CircuitBreaker) String() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.evaluateTransition()
	return fmt.Sprintf("circuit(%s, failures=%d, successes=%d)", c.state, c.failureCount, c.successCount)
}

var ErrCircuitOpen = errors.New("trusted_service: circuit breaker is open")
