//go:build rsi_netbudget

package task_runtime

import (
	"errors"
	"fmt"
	"math"
	"sync"
	"time"
)

const MaxNetworkRequestBytes = 1 << 20

var (
	ErrNetworkBudgetInvalid   = errors.New("task_runtime: network budget invalid")
	ErrNetworkBudgetExceeded  = errors.New("task_runtime: network budget exceeded")
)

type NetworkBudget struct {
	RequestsPerSecond float64
	BytesPerSecond    float64
	Burst             int
}

func (b NetworkBudget) validate() error {
	for name, value := range map[string]float64{"requestsPerSecond": b.RequestsPerSecond, "bytesPerSecond": b.BytesPerSecond} {
		if math.IsNaN(value) || math.IsInf(value, 0) || value <= 0 {
			return fmt.Errorf("%w: %s=%v", ErrNetworkBudgetInvalid, name, value)
		}
	}
	if b.Burst <= 0 {
		return fmt.Errorf("%w: burst=%d", ErrNetworkBudgetInvalid, b.Burst)
	}
	return nil
}

type NetworkBudgetError struct {
	Dimension  string
	RetryAfter time.Duration
}

func (e *NetworkBudgetError) Error() string {
	return fmt.Sprintf("%v: dimension=%s retry_after=%s", ErrNetworkBudgetExceeded, e.Dimension, e.RetryAfter)
}

func (e *NetworkBudgetError) Is(target error) bool {
	return target == ErrNetworkBudgetExceeded
}

func (e *NetworkBudgetError) Unwrap() error {
	return WrapTaskError(ErrTaskQueueFull, "network budget exceeded", ErrNetworkBudgetExceeded)
}

func (e *NetworkBudgetError) Retryable() bool {
	return e.RetryAfter > 0
}

type NetworkBucket struct {
	mu          sync.Mutex
	budget      NetworkBudget
	requestToks float64
	byteToks    float64
	last        time.Time
	now         func() time.Time
}

func NewNetworkBucket(budget NetworkBudget) (*NetworkBucket, error) {
	if err := budget.validate(); err != nil {
		return nil, err
	}
	now := time.Now
	bucket := &NetworkBucket{
		budget:      budget,
		requestToks: float64(budget.Burst),
		byteToks:    budget.BytesPerSecond,
		last:        now(),
		now:         now,
	}
	return bucket, nil
}

func (b *NetworkBucket) WithClock(now func() time.Time) *NetworkBucket {
	b.mu.Lock()
	defer b.mu.Unlock()
	if now != nil {
		b.now = now
		b.last = now()
	}
	return b
}

func (b *NetworkBucket) refillLocked(now time.Time) {
	elapsed := now.Sub(b.last).Seconds()
	if elapsed <= 0 {
		return
	}
	b.last = now
	b.requestToks = math.Min(float64(b.budget.Burst), b.requestToks+elapsed*b.budget.RequestsPerSecond)
	b.byteToks = math.Min(b.budget.BytesPerSecond, b.byteToks+elapsed*b.budget.BytesPerSecond)
}

func (b *NetworkBucket) Admit(requestBytes int64) error {
	if requestBytes < 0 || requestBytes > MaxNetworkRequestBytes {
		return fmt.Errorf("%w: bytes=%d", ErrNetworkBudgetInvalid, requestBytes)
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.refillLocked(b.now())
	if b.requestToks < 1 {
		wait := (1 - b.requestToks) / b.budget.RequestsPerSecond
		return &NetworkBudgetError{Dimension: "requests", RetryAfter: time.Duration(wait * float64(time.Second))}
	}
	if b.byteToks < float64(requestBytes) {
		wait := (float64(requestBytes) - b.byteToks) / b.budget.BytesPerSecond
		return &NetworkBudgetError{Dimension: "bytes", RetryAfter: time.Duration(wait * float64(time.Second))}
	}
	b.requestToks -= 1
	b.byteToks -= float64(requestBytes)
	return nil
}

func (b *NetworkBucket) Tokens() (float64, float64) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.requestToks, b.byteToks
}

func (b *NetworkBucket) Pressure() float64 {
	b.mu.Lock()
	defer b.mu.Unlock()
	requestRatio := b.requestToks / float64(b.budget.Burst)
	byteRatio := b.byteToks / b.budget.BytesPerSecond
	if requestRatio < byteRatio {
		return requestRatio
	}
	return byteRatio
}
