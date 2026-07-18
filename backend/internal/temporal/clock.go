package temporal

import (
	"sync"
	"time"
)

type Clock interface {
	Now() time.Time
	Since(time.Time) time.Duration
	Until(time.Time) time.Duration
}

type SystemClock struct{}

func (SystemClock) Now() time.Time                      { return time.Now() }
func (SystemClock) Since(value time.Time) time.Duration { return time.Since(value) }
func (SystemClock) Until(value time.Time) time.Duration { return time.Until(value) }

type FakeClock struct {
	mu  sync.RWMutex
	now time.Time
}

func NewFakeClock(now time.Time) *FakeClock { return &FakeClock{now: now} }

func (c *FakeClock) Now() time.Time {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.now
}

func (c *FakeClock) Since(value time.Time) time.Duration { return c.Now().Sub(value) }
func (c *FakeClock) Until(value time.Time) time.Duration { return value.Sub(c.Now()) }

func (c *FakeClock) Set(now time.Time) {
	c.mu.Lock()
	c.now = now
	c.mu.Unlock()
}

func (c *FakeClock) Advance(duration time.Duration) {
	c.mu.Lock()
	c.now = c.now.Add(duration)
	c.mu.Unlock()
}
