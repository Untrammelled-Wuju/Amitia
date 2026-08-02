package behavior

import (
	"sync/atomic"
	"time"
)

type Clock interface {
	Now() time.Time
	Since(t time.Time) time.Duration
}

type realClock struct{}

func NewRealClock() Clock { return &realClock{} }

func (c *realClock) Now() time.Time                  { return time.Now() }
func (c *realClock) Since(t time.Time) time.Duration { return time.Since(t) }

type FakeClock struct {
	current time.Time
}

func NewFakeClock(at time.Time) *FakeClock {
	return &FakeClock{current: at}
}

func (c *FakeClock) Now() time.Time                  { return c.current }
func (c *FakeClock) Since(t time.Time) time.Duration { return c.current.Sub(t) }
func (c *FakeClock) Advance(d time.Duration)         { c.current = c.current.Add(d) }
func (c *FakeClock) Set(t time.Time)                 { c.current = t }

type MonotonicClock struct {
	epoch time.Time
	mono  atomic.Int64
}

func NewMonotonicClock(now time.Time) *MonotonicClock {
	mc := &MonotonicClock{epoch: now}
	mc.mono.Store(0)
	return mc
}

func (c *MonotonicClock) Now() time.Time {
	elapsed := time.Duration(c.mono.Load()) * time.Nanosecond
	return c.epoch.Add(elapsed)
}

func (c *MonotonicClock) Since(t time.Time) time.Duration {
	return c.Now().Sub(t)
}

func (c *MonotonicClock) Advance(d time.Duration) {
	c.mono.Add(int64(d))
}

type IDGenerator interface {
	NewID() string
}

type uuidIDGen struct{}

func NewUUIDIDGen() IDGenerator { return &uuidIDGen{} }

func (g *uuidIDGen) NewID() string {
	return UUIDNew()
}

type sequentialIDGen struct {
	counter atomic.Int64
	prefix  string
}

func NewSequentialIDGen(prefix string) IDGenerator {
	return &sequentialIDGen{prefix: prefix}
}

func (g *sequentialIDGen) NewID() string {
	n := g.counter.Add(1)
	return g.prefix + "_" + intToStr(n)
}

func intToStr(n int64) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}
