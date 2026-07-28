package schedule

import (
	"context"
	"sync"
	"time"
)

type Clock interface {
	Now() time.Time
	After(d time.Duration) <-chan time.Time
	NewTimer(d time.Duration) Timer
}

type Timer interface {
	C() <-chan time.Time
	Stop() bool
	Reset(d time.Duration) bool
}

type RealClock struct{}

func NewRealClock() *RealClock { return &RealClock{} }

func (c *RealClock) Now() time.Time { return time.Now().UTC() }

func (c *RealClock) After(d time.Duration) <-chan time.Time { return time.After(d) }

func (c *RealClock) NewTimer(d time.Duration) Timer {
	t := time.NewTimer(d)
	return &realTimer{t: t}
}

type realTimer struct {
	t *time.Timer
}

func (t *realTimer) C() <-chan time.Time        { return t.t.C }
func (t *realTimer) Stop() bool                 { return t.t.Stop() }
func (t *realTimer) Reset(d time.Duration) bool { return t.t.Reset(d) }

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

func (c *FakeClock) After(d time.Duration) <-chan time.Time {
	now := c.Now()
	target := now.Add(d)
	ch := make(chan time.Time, 1)
	go func() {
		for {
			current := c.Now()
			if !current.Before(target) {
				ch <- current
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
	}()
	return ch
}

func (c *FakeClock) NewTimer(d time.Duration) Timer {
	return &fakeTimer{
		clock:  c,
		target: c.Now().Add(d),
		ch:     make(chan time.Time, 1),
	}
}

type fakeTimer struct {
	clock  *FakeClock
	target time.Time
	ch     chan time.Time
	mu     sync.Mutex
	active bool
}

func (t *fakeTimer) C() <-chan time.Time { return t.ch }

func (t *fakeTimer) Stop() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	was := t.active
	t.active = false
	return was
}

func (t *fakeTimer) Reset(d time.Duration) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	was := t.active
	t.target = t.clock.Now().Add(d)
	t.active = true
	go func() {
		for {
			t.clock.mu.RLock()
			now := t.clock.now
			t.clock.mu.RUnlock()
			if !now.Before(t.target) {
				t.mu.Lock()
				if t.active {
					t.ch <- now
				}
				t.mu.Unlock()
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
	}()
	return was
}

type clockContextKey struct{}

func ContextWithClock(ctx context.Context, clock Clock) context.Context {
	return context.WithValue(ctx, clockContextKey{}, clock)
}

func ClockFromContext(ctx context.Context) Clock {
	if v, ok := ctx.Value(clockContextKey{}).(Clock); ok {
		return v
	}
	return nil
}
