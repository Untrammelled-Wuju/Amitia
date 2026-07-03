package circuitbreaker

import (
	"sync"
	"time"
)

type State string

const (
	StateClosed     State = "closed"
	StateOpen       State = "open"
	StateHalfOpen   State = "half_open"
)

type CircuitBreaker struct {
	name         string
	mu           sync.Mutex
	state        State
	failureCount int
	successCount int
	lastFailure  time.Time

	maxFailures  int
	timeout      time.Duration
	halfOpenMax  int
}

func New(name string, maxFailures int, timeout time.Duration, halfOpenMax int) *CircuitBreaker {
	return &CircuitBreaker{
		name:        name,
		state:       StateClosed,
		maxFailures: maxFailures,
		timeout:     timeout,
		halfOpenMax: halfOpenMax,
	}
}

func (cb *CircuitBreaker) State() State {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	return cb.currentState()
}

func (cb *CircuitBreaker) currentState() State {
	if cb.state == StateOpen {
		if time.Since(cb.lastFailure) > cb.timeout {
			cb.state = StateHalfOpen
			cb.successCount = 0
		}
	}
	return cb.state
}

func (cb *CircuitBreaker) Allow() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.currentState() {
	case StateClosed:
		return true
	case StateOpen:
		return false
	case StateHalfOpen:
		return cb.successCount < cb.halfOpenMax
	}
	return false
}

func (cb *CircuitBreaker) RecordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.currentState()
	cb.failureCount = 0
	if cb.state == StateHalfOpen {
		cb.successCount++
		if cb.successCount >= cb.halfOpenMax {
			cb.state = StateClosed
			cb.successCount = 0
		}
	}
}

func (cb *CircuitBreaker) RecordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.currentState()
	cb.failureCount++
	cb.lastFailure = time.Now()

	if cb.state == StateHalfOpen || cb.failureCount >= cb.maxFailures {
		cb.state = StateOpen
	}
}

type DegradationMatrix struct {
	mu       sync.RWMutex
	fallback map[string]func() error
}

func NewDegradationMatrix() *DegradationMatrix {
	return &DegradationMatrix{
		fallback: map[string]func() error{},
	}
}

func (dm *DegradationMatrix) Register(service string, fallback func() error) {
	dm.mu.Lock()
	defer dm.mu.Unlock()
	dm.fallback[service] = fallback
}

func (dm *DegradationMatrix) Fallback(service string) func() error {
	dm.mu.RLock()
	defer dm.mu.RUnlock()
	return dm.fallback[service]
}
