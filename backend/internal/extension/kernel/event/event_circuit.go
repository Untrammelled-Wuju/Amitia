package event

import (
	"sync"
	"time"
)

type CircuitState string

const (
	CircuitClosed   CircuitState = "closed"
	CircuitOpen     CircuitState = "open"
	CircuitHalfOpen CircuitState = "half_open"
)

type CircuitBreakerConfig struct {
	FailureThreshold int
	SuccessThreshold int
	OpenTimeout      time.Duration
	HalfOpenMax      int
	TriggerErrors    []string
}

func DefaultCircuitConfig() CircuitBreakerConfig {
	return CircuitBreakerConfig{
		FailureThreshold: 5,
		SuccessThreshold: 3,
		OpenTimeout:      30 * time.Second,
		HalfOpenMax:      1,
		TriggerErrors: []string{
			"timeout",
			"runtime_error",
			"handler_not_found",
			"invalid_result",
			"protocol_error",
			"host_api_abuse",
		},
	}
}

type CircuitBreaker struct {
	mu                 sync.Mutex
	state              CircuitState
	consecutiveFails   int
	consecutiveSuccess int
	totalFails         int64
	totalSuccess       int64
	lastFailCode       string
	lastFailTime       time.Time
	openedAt           time.Time
	config             CircuitBreakerConfig
}

func NewCircuitBreaker(config CircuitBreakerConfig) *CircuitBreaker {
	return &CircuitBreaker{
		state:  CircuitClosed,
		config: config,
	}
}

func (cb *CircuitBreaker) Allow() (bool, CircuitState) {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	switch cb.state {
	case CircuitClosed:
		return true, CircuitClosed
	case CircuitOpen:
		if time.Since(cb.openedAt) >= cb.config.OpenTimeout {
			cb.state = CircuitHalfOpen
			cb.consecutiveSuccess = 0
			return true, CircuitHalfOpen
		}
		return false, CircuitOpen
	case CircuitHalfOpen:
		if cb.consecutiveSuccess >= cb.config.SuccessThreshold {
			cb.state = CircuitClosed
			cb.consecutiveFails = 0
			return true, CircuitClosed
		}
		return true, CircuitHalfOpen
	default:
		return true, CircuitClosed
	}
}

func (cb *CircuitBreaker) RecordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.consecutiveSuccess++
	cb.consecutiveFails = 0
	cb.totalSuccess++
	if cb.state == CircuitHalfOpen && cb.consecutiveSuccess >= cb.config.SuccessThreshold {
		cb.state = CircuitClosed
		cb.consecutiveFails = 0
	}
}

func (cb *CircuitBreaker) RecordFailure(code string) {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	if !cb.shouldTrigger(code) {
		return
	}
	cb.consecutiveFails++
	cb.consecutiveSuccess = 0
	cb.totalFails++
	cb.lastFailCode = code
	cb.lastFailTime = time.Now().UTC()
	if cb.state == CircuitHalfOpen {
		cb.openCircuit()
		return
	}
	if cb.consecutiveFails >= cb.config.FailureThreshold {
		cb.openCircuit()
	}
}

func (cb *CircuitBreaker) shouldTrigger(code string) bool {
	if len(cb.config.TriggerErrors) == 0 {
		return true
	}
	for _, e := range cb.config.TriggerErrors {
		if e == code {
			return true
		}
	}
	return false
}

func (cb *CircuitBreaker) openCircuit() {
	cb.state = CircuitOpen
	cb.openedAt = time.Now().UTC()
}

func (cb *CircuitBreaker) Reset() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.state = CircuitClosed
	cb.consecutiveFails = 0
	cb.consecutiveSuccess = 0
}

func (cb *CircuitBreaker) State() CircuitState {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	return cb.state
}

func (cb *CircuitBreaker) Stats() CircuitStats {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	return CircuitStats{
		State:              cb.state,
		ConsecutiveFails:   cb.consecutiveFails,
		ConsecutiveSuccess: cb.consecutiveSuccess,
		TotalFails:         cb.totalFails,
		TotalSuccess:       cb.totalSuccess,
		LastFailCode:       cb.lastFailCode,
		LastFailTime:       cb.lastFailTime,
		OpenedAt:           cb.openedAt,
	}
}

type CircuitStats struct {
	State              CircuitState
	ConsecutiveFails   int
	ConsecutiveSuccess int
	TotalFails         int64
	TotalSuccess       int64
	LastFailCode       string
	LastFailTime       time.Time
	OpenedAt           time.Time
}

type CircuitBreakerRegistry struct {
	mu       sync.RWMutex
	breakers map[string]*CircuitBreaker
	config   CircuitBreakerConfig
}

func NewCircuitBreakerRegistry(config CircuitBreakerConfig) *CircuitBreakerRegistry {
	return &CircuitBreakerRegistry{
		breakers: make(map[string]*CircuitBreaker),
		config:   config,
	}
}

func (r *CircuitBreakerRegistry) GetOrCreate(key string) *CircuitBreaker {
	r.mu.RLock()
	cb, ok := r.breakers[key]
	r.mu.RUnlock()
	if ok {
		return cb
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if cb, ok := r.breakers[key]; ok {
		return cb
	}
	cb = NewCircuitBreaker(r.config)
	r.breakers[key] = cb
	return cb
}

func (r *CircuitBreakerRegistry) Get(key string) (*CircuitBreaker, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	cb, ok := r.breakers[key]
	return cb, ok
}

func (r *CircuitBreakerRegistry) Reset(key string) {
	r.mu.RLock()
	cb, ok := r.breakers[key]
	r.mu.RUnlock()
	if ok {
		cb.Reset()
	}
}

func (r *CircuitBreakerRegistry) All() map[string]CircuitStats {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make(map[string]CircuitStats, len(r.breakers))
	for k, cb := range r.breakers {
		result[k] = cb.Stats()
	}
	return result
}
