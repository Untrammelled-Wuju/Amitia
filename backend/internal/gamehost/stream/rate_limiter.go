package stream

import (
	"sync"
	"time"
)

type rateLimiter struct {
	mu         sync.Mutex
	rate       float64
	burst      int
	tokens     float64
	lastRefill time.Time
	clock      func() time.Time
}

func newRateLimiter(policy RateLimitPolicy) *rateLimiter {
	now := time.Now()
	return &rateLimiter{
		rate:       float64(policy.MessagesPerSecond),
		burst:      policy.Burst,
		tokens:     float64(policy.Burst),
		lastRefill: now,
		clock:      time.Now,
	}
}

func newRateLimiterWithClock(policy RateLimitPolicy, clock func() time.Time) *rateLimiter {
	return &rateLimiter{
		rate:       float64(policy.MessagesPerSecond),
		burst:      policy.Burst,
		tokens:     float64(policy.Burst),
		lastRefill: clock(),
		clock:      clock,
	}
}

func (r *rateLimiter) Allow() bool {
	return r.AllowN(1)
}

func (r *rateLimiter) AllowN(n int) bool {
	if n <= 0 {
		return true
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.refill()
	if r.tokens >= float64(n) {
		r.tokens -= float64(n)
		return true
	}
	return false
}

func (r *rateLimiter) TryAcquire(timeout time.Duration) bool {
	r.mu.Lock()
	r.refill()
	if r.tokens >= 1 {
		r.tokens--
		r.mu.Unlock()
		return true
	}
	waitTime := time.Duration((1 - r.tokens) / r.rate * float64(time.Second))
	r.mu.Unlock()
	if waitTime > timeout {
		return false
	}
	time.Sleep(waitTime)
	r.mu.Lock()
	defer r.mu.Unlock()
	r.refill()
	if r.tokens >= 1 {
		r.tokens--
		return true
	}
	return false
}

func (r *rateLimiter) refill() {
	now := r.clock()
	elapsed := now.Sub(r.lastRefill).Seconds()
	r.tokens += elapsed * r.rate
	if r.tokens > float64(r.burst) {
		r.tokens = float64(r.burst)
	}
	if r.tokens < 0 {
		r.tokens = 0
	}
	r.lastRefill = now
}

func (r *rateLimiter) Rate() float64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.rate
}
