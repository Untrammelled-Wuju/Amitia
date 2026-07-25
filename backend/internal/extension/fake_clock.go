package extension

import (
	"sync"
	"time"
)

type FakeClock struct {
	mu   sync.RWMutex
	now  time.Time
	tick chan time.Time
}

func NewFakeClock(t time.Time) *FakeClock {
	return &FakeClock{now: t, tick: make(chan time.Time, 1)}
}

func (c *FakeClock) Now() time.Time {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.now
}

func (c *FakeClock) Advance(d time.Duration) time.Time {
	c.mu.Lock()
	c.now = c.now.Add(d)
	now := c.now
	c.mu.Unlock()
	select {
	case c.tick <- now:
	default:
	}
	return now
}

func (c *FakeClock) Set(t time.Time) {
	c.mu.Lock()
	c.now = t
	c.mu.Unlock()
	select {
	case c.tick <- t:
	default:
	}
}

func (c *FakeClock) Tick() <-chan time.Time {
	return c.tick
}
