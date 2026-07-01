package mindruntime

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
	FailureThreshold   int           `json:"failureThreshold"`
	SuccessThreshold   int           `json:"successThreshold"`
	OpenTimeout        time.Duration `json:"openTimeout"`
	HalfOpenMaxRequest int           `json:"halfOpenMaxRequest"`
	Enabled            bool          `json:"enabled"`
}

func DefaultCircuitBreakerConfig() CircuitBreakerConfig {
	return CircuitBreakerConfig{
		FailureThreshold:   5,
		SuccessThreshold:   3,
		OpenTimeout:        30 * time.Second,
		HalfOpenMaxRequest: 1,
		Enabled:            true,
	}
}

type CircuitBreaker struct {
	Name         string              `json:"name"`
	State        CircuitState        `json:"state"`
	Config       CircuitBreakerConfig `json:"config"`
	Failures     int                 `json:"failures"`
	Successes    int                 `json:"successes"`
	LastFailure  time.Time           `json:"lastFailure"`
	LastSuccess  time.Time           `json:"lastSuccess"`
	OpenedAt     time.Time           `json:"openedAt"`
	TotalCalls   int64               `json:"totalCalls"`
	TotalFail    int64               `json:"totalFail"`
	HalfOpenCount int                `json:"halfOpenCount"`
	mu           sync.RWMutex
}

func NewCircuitBreaker(name string, config CircuitBreakerConfig) *CircuitBreaker {
	return &CircuitBreaker{
		Name:   name,
		State:  CircuitClosed,
		Config: config,
	}
}

func (cb *CircuitBreaker) Allow() bool {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	if !cb.Config.Enabled {
		return true
	}
	switch cb.State {
	case CircuitClosed:
		return true
	case CircuitOpen:
		if time.Since(cb.OpenedAt) >= cb.Config.OpenTimeout {
			return true
		}
		return false
	case CircuitHalfOpen:
		return cb.HalfOpenCount < cb.Config.HalfOpenMaxRequest
	default:
		return true
	}
}

func (cb *CircuitBreaker) RecordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.TotalCalls++
	cb.LastSuccess = time.Now().UTC()
	switch cb.State {
	case CircuitHalfOpen:
		cb.Successes++
		if cb.Successes >= cb.Config.SuccessThreshold {
			cb.State = CircuitClosed
			cb.Failures = 0
			cb.Successes = 0
			cb.HalfOpenCount = 0
		}
	case CircuitOpen:
		if time.Since(cb.OpenedAt) >= cb.Config.OpenTimeout {
			cb.State = CircuitHalfOpen
			cb.Successes = 1
			cb.HalfOpenCount = 1
			if cb.Successes >= cb.Config.SuccessThreshold {
				cb.State = CircuitClosed
				cb.Failures = 0
				cb.Successes = 0
				cb.HalfOpenCount = 0
			}
		}
	}
}

func (cb *CircuitBreaker) RecordFailure() {
 	cb.mu.Lock()
 	defer cb.mu.Unlock()
 	cb.TotalCalls++
 	cb.TotalFail++
 	cb.LastFailure = time.Now().UTC()
	effectiveState := cb.State
	if cb.State == CircuitOpen && time.Since(cb.OpenedAt) >= cb.Config.OpenTimeout {
		effectiveState = CircuitHalfOpen
	}
	switch effectiveState {
 	case CircuitClosed:
 		cb.Failures++
 		if cb.Failures >= cb.Config.FailureThreshold {
 			cb.State = CircuitOpen
 			cb.OpenedAt = time.Now().UTC()
 			cb.Successes = 0
 			cb.HalfOpenCount = 0
 		}
 	case CircuitHalfOpen:
 		cb.State = CircuitOpen
 		cb.OpenedAt = time.Now().UTC()
 		cb.Failures = 0
 		cb.Successes = 0
 		cb.HalfOpenCount = 0
 	case CircuitOpen:
 	}
	
}

func (cb *CircuitBreaker) Reset() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.State = CircuitClosed
	cb.Failures = 0
	cb.Successes = 0
	cb.HalfOpenCount = 0
	cb.OpenedAt = time.Time{}
}

func (cb *CircuitBreaker) Status() CircuitState {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	if cb.State == CircuitOpen && time.Since(cb.OpenedAt) >= cb.Config.OpenTimeout {
		return CircuitHalfOpen
	}
	return cb.State
}

type DependencyHealth struct {
	Name            string          `json:"name"`
	CircuitBreaker  *CircuitBreaker `json:"circuitBreaker"`
	LastHealthCheck time.Time       `json:"lastHealthCheck"`
	Healthy         bool            `json:"healthy"`
	CheckMessage    string          `json:"checkMessage,omitempty"`
}

type CircuitBreakerRegistry struct {
	breakers map[string]*CircuitBreaker
	mu       sync.RWMutex
}

var DefaultCircuitBreakerRegistry = NewCircuitBreakerRegistry()

func NewCircuitBreakerRegistry() *CircuitBreakerRegistry {
	return &CircuitBreakerRegistry{
		breakers: make(map[string]*CircuitBreaker),
	}
}

func (r *CircuitBreakerRegistry) Register(name string, config CircuitBreakerConfig) *CircuitBreaker {
	r.mu.Lock()
	defer r.mu.Unlock()
	if existing, ok := r.breakers[name]; ok {
		return existing
	}
	cb := NewCircuitBreaker(name, config)
	r.breakers[name] = cb
	return cb
}

func (r *CircuitBreakerRegistry) Get(name string) *CircuitBreaker {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.breakers[name]
}

func (r *CircuitBreakerRegistry) All() []*CircuitBreaker {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]*CircuitBreaker, 0, len(r.breakers))
	for _, cb := range r.breakers {
		result = append(result, cb)
	}
	return result
}

func (r *CircuitBreakerRegistry) Allowed(name string) bool {
	cb := r.Get(name)
	if cb == nil {
		return true
	}
	return cb.Allow()
}

func (r *CircuitBreakerRegistry) RecordSuccess(name string) {
	if cb := r.Get(name); cb != nil {
		cb.RecordSuccess()
	}
}

func (r *CircuitBreakerRegistry) RecordFailure(name string) {
	if cb := r.Get(name); cb != nil {
		cb.RecordFailure()
	}
}

func (r *CircuitBreakerRegistry) ResetAll() {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, cb := range r.breakers {
		cb.Reset()
	}
}

func (r *CircuitBreakerRegistry) HealthReport(name string) DependencyHealth {
	cb := r.Get(name)
	if cb == nil {
		return DependencyHealth{
			Name:         name,
			Healthy:      true,
			CheckMessage: "no circuit breaker registered",
		}
	}
	status := cb.Status()
	return DependencyHealth{
		Name:            name,
		CircuitBreaker:  cb,
		LastHealthCheck: time.Now().UTC(),
		Healthy:         status == CircuitClosed,
		CheckMessage:    "circuit state: " + string(status),
	}
}

func (r *CircuitBreakerRegistry) AllHealthReports() []DependencyHealth {
	r.mu.RLock()
	defer r.mu.RUnlock()
	results := make([]DependencyHealth, 0, len(r.breakers))
	for name, cb := range r.breakers {
		status := cb.Status()
		results = append(results, DependencyHealth{
			Name:            name,
			CircuitBreaker:  cb,
			LastHealthCheck: time.Now().UTC(),
			Healthy:         status == CircuitClosed,
			CheckMessage:    "circuit state: " + string(status),
		})
	}
	return results
}
