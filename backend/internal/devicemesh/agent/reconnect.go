package agent

import (
	"math/rand"
	"sync"
	"time"
)

type Backoff struct {
	mu         sync.Mutex
	attempt    int
	baseDelay  time.Duration
	maxDelay   time.Duration
	resetAfter time.Duration
	lastReady  time.Time
	increments []time.Duration
}

func NewBackoff() *Backoff {
	return &Backoff{
		baseDelay: 1 * time.Second,
		maxDelay:  60 * time.Second,
		resetAfter: 30 * time.Second,
		increments: []time.Duration{
			1 * time.Second,
			2 * time.Second,
			4 * time.Second,
			8 * time.Second,
			15 * time.Second,
			30 * time.Second,
			60 * time.Second,
		},
		lastReady: time.Now(),
	}
}

func (b *Backoff) Duration() time.Duration {
	b.mu.Lock()
	defer b.mu.Unlock()

	if time.Since(b.lastReady) > b.resetAfter && b.attempt > 0 {
		b.attempt = 0
	}

	delay := b.baseDelay
	if b.attempt < len(b.increments) {
		delay = b.increments[b.attempt]
	} else {
		delay = b.maxDelay
	}

	b.attempt++

	jitter := time.Duration(float64(delay) * 0.2 * rand.Float64())
	if rand.Intn(2) == 0 {
		delay = delay - jitter/2 + jitter
	}

	if delay > b.maxDelay {
		delay = b.maxDelay
	}

	return delay
}

func (b *Backoff) Reset() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.attempt = 0
	b.lastReady = time.Now()
}
