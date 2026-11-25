package trusted_service

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

type BreakerKey struct {
	ScopeKey   string
	ServiceID  string
	ProviderID string
}

const breakerKeySeparator = '\x1e'

var ErrBreakerKeyInvalid = errors.New("trusted_service: breaker key is invalid")

func (k BreakerKey) Normalize() (BreakerKey, error) {
	values := [3]string{k.ScopeKey, k.ServiceID, k.ProviderID}
	normalized := BreakerKey{}
	for index, value := range values {
		trimmed := strings.TrimSpace(value)
		switch {
		case trimmed == "":
			return BreakerKey{}, fmt.Errorf("%w: field %d is empty", ErrBreakerKeyInvalid, index)
		case len(trimmed) > 256:
			return BreakerKey{}, fmt.Errorf("%w: field %d is longer than 256 bytes", ErrBreakerKeyInvalid, index)
		case strings.ContainsRune(trimmed, breakerKeySeparator):
			return BreakerKey{}, fmt.Errorf("%w: field %d contains the separator", ErrBreakerKeyInvalid, index)
		}
		switch index {
		case 0:
			normalized.ScopeKey = trimmed
		case 1:
			normalized.ServiceID = trimmed
		case 2:
			normalized.ProviderID = trimmed
		}
	}
	return normalized, nil
}

func (k BreakerKey) String() string {
	return strings.Join([]string{k.ScopeKey, k.ServiceID, k.ProviderID}, string(breakerKeySeparator))
}

type BreakerSnapshot struct {
	Key      BreakerKey
	State    CircuitState
	Failures int
	Probes   int
}

type ScopedBreakers struct {
	mu       sync.Mutex
	breakers map[string]*CircuitBreaker
	keys     map[string]BreakerKey
	config   CircuitConfig
	now      func() time.Time
}

func NewScopedBreakers(config CircuitConfig) *ScopedBreakers {
	return newScopedBreakers(config, time.Now)
}

func newScopedBreakers(config CircuitConfig, now func() time.Time) *ScopedBreakers {
	defaults := DefaultCircuitConfig()
	if config.FailureThreshold < 1 {
		config.FailureThreshold = defaults.FailureThreshold
	}
	if config.HalfOpenAttempts < 1 {
		config.HalfOpenAttempts = defaults.HalfOpenAttempts
	}
	if config.RecoveryAfter <= 0 {
		config.RecoveryAfter = defaults.RecoveryAfter
	}
	if config.ResetAfterHealthy <= 0 {
		config.ResetAfterHealthy = defaults.ResetAfterHealthy
	}
	if now == nil {
		now = time.Now
	}
	return &ScopedBreakers{
		breakers: make(map[string]*CircuitBreaker),
		keys:     make(map[string]BreakerKey),
		config:   config,
		now:      now,
	}
}

func (r *ScopedBreakers) resolve(key BreakerKey) (*CircuitBreaker, BreakerKey, error) {
	normalized, err := key.Normalize()
	if err != nil {
		return nil, BreakerKey{}, err
	}
	id := normalized.String()
	r.mu.Lock()
	defer r.mu.Unlock()
	if existing, ok := r.breakers[id]; ok {
		return existing, normalized, nil
	}
	created := newCircuitBreaker(r.config, r.now)
	r.breakers[id] = created
	r.keys[id] = normalized
	return created, normalized, nil
}

func (r *ScopedBreakers) Acquire(key BreakerKey) error {
	breaker, _, err := r.resolve(key)
	if err != nil {
		return err
	}
	return breaker.TryAcquire()
}

func (r *ScopedBreakers) Success(key BreakerKey) error {
	breaker, _, err := r.resolve(key)
	if err != nil {
		return err
	}
	breaker.RecordSuccess()
	return nil
}

func (r *ScopedBreakers) Failure(key BreakerKey) error {
	breaker, _, err := r.resolve(key)
	if err != nil {
		return err
	}
	breaker.RecordFailure()
	return nil
}

func (r *ScopedBreakers) Abort(key BreakerKey) error {
	breaker, _, err := r.resolve(key)
	if err != nil {
		return err
	}
	breaker.AbortProbe()
	return nil
}

func (r *ScopedBreakers) State(key BreakerKey) (CircuitState, error) {
	breaker, _, err := r.resolve(key)
	if err != nil {
		return "", err
	}
	return breaker.State(), nil
}

func (r *ScopedBreakers) Failures(key BreakerKey) (int, error) {
	breaker, _, err := r.resolve(key)
	if err != nil {
		return 0, err
	}
	return breaker.FailureCount(), nil
}

func (r *ScopedBreakers) Probes(key BreakerKey) (int, error) {
	breaker, _, err := r.resolve(key)
	if err != nil {
		return 0, err
	}
	return breaker.ProbeCount(), nil
}

func (r *ScopedBreakers) Reset(key BreakerKey) error {
	breaker, _, err := r.resolve(key)
	if err != nil {
		return err
	}
	breaker.Reset()
	return nil
}

func (r *ScopedBreakers) Len() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.breakers)
}

func (r *ScopedBreakers) Snapshot() []BreakerSnapshot {
	r.mu.Lock()
	pairs := make([]struct {
		key     BreakerKey
		breaker *CircuitBreaker
	}, 0, len(r.breakers))
	for id, breaker := range r.breakers {
		pairs = append(pairs, struct {
			key     BreakerKey
			breaker *CircuitBreaker
		}{key: r.keys[id], breaker: breaker})
	}
	r.mu.Unlock()
	snapshot := make([]BreakerSnapshot, 0, len(pairs))
	for _, pair := range pairs {
		snapshot = append(snapshot, BreakerSnapshot{
			Key:      pair.key,
			State:    pair.breaker.State(),
			Failures: pair.breaker.FailureCount(),
			Probes:   pair.breaker.ProbeCount(),
		})
	}
	sort.Slice(snapshot, func(left, right int) bool {
		return snapshot[left].Key.String() < snapshot[right].Key.String()
	})
	return snapshot
}

func (r *ScopedBreakers) OpenInScope(scopeKey string) ([]BreakerKey, error) {
	trimmed := strings.TrimSpace(scopeKey)
	if trimmed == "" {
		return nil, fmt.Errorf("%w: scope is empty", ErrBreakerKeyInvalid)
	}
	open := make([]BreakerKey, 0)
	for _, entry := range r.Snapshot() {
		if entry.Key.ScopeKey != trimmed || entry.State != CircuitOpen {
			continue
		}
		open = append(open, entry.Key)
	}
	return open, nil
}
