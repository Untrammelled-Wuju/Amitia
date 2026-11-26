//go:build rsi_ratelimit

package task_runtime

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type rateClock struct {
	mu sync.Mutex
	at time.Time
}

func (c *rateClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.at.IsZero() {
		c.at = time.Unix(1700000000, 0)
	}
	return c.at
}

func (c *rateClock) Advance(delta time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.at.IsZero() {
		c.at = time.Unix(1700000000, 0)
	}
	c.at = c.at.Add(delta)
}

func newEventLimiter(t *testing.T, config RateLimitConfig, clock *rateClock) *EventLimiter {
	t.Helper()
	limiter, err := NewEventLimiter(config)
	if err != nil {
		t.Fatalf("limiter: %v", err)
	}
	return limiter.WithClock(clock.Now)
}

func TestEventLimiterAdmitsWithinLimits(t *testing.T) {
	clock := &rateClock{}
	limiter := newEventLimiter(t, RateLimitConfig{PerPluginPerSecond: 5, PerTopicPerSecond: 3, Capacity: 10, DeadLetterCapacity: 8}, clock)
	if err := limiter.Admit("plugin-a", "chat.message"); err != nil {
		t.Fatalf("admit: %v", err)
	}
	if err := limiter.Admit("plugin-a", "chat.message"); err != nil {
		t.Fatalf("admit: %v", err)
	}
	if limiter.Pending() != 2 || len(limiter.DeadLetters()) != 0 {
		t.Fatalf("pending=%d dead=%v", limiter.Pending(), limiter.DeadLetters())
	}
	fmt.Printf("m07-s14 admit pending=2 dead=0\n")
}

func TestEventLimiterPluginRate(t *testing.T) {
	clock := &rateClock{}
	limiter := newEventLimiter(t, RateLimitConfig{PerPluginPerSecond: 1, PerTopicPerSecond: 10, Capacity: 10, DeadLetterCapacity: 8}, clock)
	if err := limiter.Admit("plugin-a", "a"); err != nil {
		t.Fatalf("first: %v", err)
	}
	err := limiter.Admit("plugin-a", "b")
	var limit *RateLimitError
	if !errors.As(err, &limit) || limit.Scope != "plugin" {
		t.Fatalf("err=%v", err)
	}
	dead := limiter.DeadLetters()
	if len(dead) != 1 || dead[0].Reason != "plugin_rate" || dead[0].Count != 1 {
		t.Fatalf("dead=%+v", dead)
	}
	fmt.Printf("m07-s14 plugin_rate dead=1 reason=plugin_rate\n")
}

func TestEventLimiterTopicRate(t *testing.T) {
	clock := &rateClock{}
	limiter := newEventLimiter(t, RateLimitConfig{PerPluginPerSecond: 10, PerTopicPerSecond: 1, Capacity: 10, DeadLetterCapacity: 8}, clock)
	if err := limiter.Admit("plugin-a", "topic"); err != nil {
		t.Fatalf("first: %v", err)
	}
	err := limiter.Admit("plugin-a", "topic")
	var limit *RateLimitError
	if !errors.As(err, &limit) || limit.Scope != "topic" {
		t.Fatalf("err=%v", err)
	}
	if limiter.DeadLetters()[0].Reason != "topic_rate" {
		t.Fatalf("dead=%+v", limiter.DeadLetters())
	}
	fmt.Printf("m07-s14 topic_rate dead=1 reason=topic_rate\n")
}

func TestEventLimiterRetryable(t *testing.T) {
	clock := &rateClock{}
	limiter := newEventLimiter(t, RateLimitConfig{PerPluginPerSecond: 1, PerTopicPerSecond: 1, Capacity: 10, DeadLetterCapacity: 8}, clock)
	if err := limiter.Admit("plugin", "topic"); err != nil {
		t.Fatalf("first: %v", err)
	}
	err := limiter.Admit("plugin", "topic")
	var limit *RateLimitError
	if !errors.As(err, &limit) || !limit.Retryable() || limit.RetryAfter <= 0 {
		t.Fatalf("err=%v", err)
	}
	if !IsTaskErrorCode(err, ErrTaskQueueFull) || HTTPStatusForErrorCode(ErrTaskQueueFull) != 429 {
		t.Fatalf("task code err=%v", err)
	}
	fmt.Printf("m07-s14 retry after_ms>0 http=429\n")
}

func TestEventLimiterQueueFull(t *testing.T) {
	clock := &rateClock{}
	limiter := newEventLimiter(t, RateLimitConfig{PerPluginPerSecond: 10, PerTopicPerSecond: 10, Capacity: 1, DeadLetterCapacity: 8}, clock)
	if err := limiter.Admit("plugin", "a"); err != nil {
		t.Fatalf("first: %v", err)
	}
	err := limiter.Admit("plugin", "b")
	var limit *RateLimitError
	if !errors.As(err, &limit) || limit.Scope != "queue" {
		t.Fatalf("err=%v", err)
	}
	if limiter.DeadLetters()[0].Reason != "queue_full" {
		t.Fatalf("dead=%+v", limiter.DeadLetters())
	}
	fmt.Printf("m07-s14 queue_full dead=1 reason=queue_full\n")
}

func TestEventLimiterDeadLetterBounded(t *testing.T) {
	clock := &rateClock{}
	limiter := newEventLimiter(t, RateLimitConfig{PerPluginPerSecond: 1, PerTopicPerSecond: 1, Capacity: 10, DeadLetterCapacity: 2}, clock)
	if err := limiter.Admit("p", "t1"); err != nil {
		t.Fatalf("first: %v", err)
	}
	for _, topic := range []string{"t2", "t3", "t4"} {
		if err := limiter.Admit("p", topic); err == nil {
			t.Fatalf("topic %s should be limited", topic)
		}
	}
	if len(limiter.DeadLetters()) != 2 || limiter.DeadDropped() != 1 {
		t.Fatalf("dead=%d dropped=%d", len(limiter.DeadLetters()), limiter.DeadDropped())
	}
	fmt.Printf("m07-s14 dead_bounded len=2 dropped=1\n")
}

func TestEventLimiterTrace(t *testing.T) {
	clock := &rateClock{}
	limiter := newEventLimiter(t, RateLimitConfig{PerPluginPerSecond: 1, PerTopicPerSecond: 1, Capacity: 10, DeadLetterCapacity: 8}, clock)
	if err := limiter.Admit("p", "t"); err != nil {
		t.Fatalf("first: %v", err)
	}
	for i := 0; i < 2; i++ {
		if err := limiter.Admit("p", "t"); err == nil {
			t.Fatalf("should be limited")
		}
	}
	dead := limiter.DeadLetters()
	record, ok := limiter.Trace(dead[0].Seq)
	if !ok || record.Count != 2 || record.Plugin != "p" || record.Topic != "t" {
		t.Fatalf("trace=%+v ok=%v", record, ok)
	}
	if _, ok := limiter.Trace(999); ok {
		t.Fatalf("missing seq found")
	}
	fmt.Printf("m07-s14 trace count=2 ok=true\n")
}

func TestEventLimiterWindowReset(t *testing.T) {
	clock := &rateClock{}
	limiter := newEventLimiter(t, RateLimitConfig{PerPluginPerSecond: 1, PerTopicPerSecond: 1, Capacity: 10, DeadLetterCapacity: 8}, clock)
	if err := limiter.Admit("p", "t"); err != nil {
		t.Fatalf("first: %v", err)
	}
	if err := limiter.Admit("p", "t"); err == nil {
		t.Fatalf("should be limited")
	}
	clock.Advance(time.Second)
	if err := limiter.Admit("p", "t"); err != nil {
		t.Fatalf("after reset: %v", err)
	}
	fmt.Printf("m07-s14 window_reset ok=true\n")
}

func TestEventLimiterInvalidConfig(t *testing.T) {
	cases := []RateLimitConfig{
		{PerPluginPerSecond: 0, PerTopicPerSecond: 1, Capacity: 1},
		{PerPluginPerSecond: 1, PerTopicPerSecond: 0, Capacity: 1},
		{PerPluginPerSecond: 1, PerTopicPerSecond: 1, Capacity: 0},
		{PerPluginPerSecond: 1, PerTopicPerSecond: 1, Capacity: 1, DeadLetterCapacity: -1},
	}
	invalid := 0
	for _, config := range cases {
		if _, err := NewEventLimiter(config); errors.Is(err, ErrRateLimitInvalid) {
			invalid++
		}
	}
	if invalid != 4 {
		t.Fatalf("invalid=%d", invalid)
	}
	fmt.Printf("m07-s14 config invalid=4\n")
}

func TestEventLimiterInvalidNames(t *testing.T) {
	clock := &rateClock{}
	limiter := newEventLimiter(t, RateLimitConfig{PerPluginPerSecond: 1, PerTopicPerSecond: 1, Capacity: 1, DeadLetterCapacity: 1}, clock)
	if err := limiter.Admit(" ", "t"); !errors.Is(err, ErrRateLimitInvalid) {
		t.Fatalf("plugin err=%v", err)
	}
	if err := limiter.Admit("p", ""); !errors.Is(err, ErrRateLimitInvalid) {
		t.Fatalf("topic err=%v", err)
	}
	fmt.Printf("m07-s14 names invalid=2\n")
}

func TestEventLimiterConcurrent(t *testing.T) {
	clock := &rateClock{}
	limiter := newEventLimiter(t, RateLimitConfig{PerPluginPerSecond: 5, PerTopicPerSecond: 100, Capacity: 100, DeadLetterCapacity: 8}, clock)
	var admitted atomic.Int64
	var wait sync.WaitGroup
	for i := 0; i < 32; i++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			if err := limiter.Admit("p", "t"); err == nil {
				admitted.Add(1)
			}
		}()
	}
	wait.Wait()
	if admitted.Load() != 5 || limiter.Pending() != 5 {
		t.Fatalf("admitted=%d pending=%d", admitted.Load(), limiter.Pending())
	}
	fmt.Printf("m07-s14 concurrent admitted=5\n")
}

func TestEventLimiterCompleteFreesCapacity(t *testing.T) {
	clock := &rateClock{}
	limiter := newEventLimiter(t, RateLimitConfig{PerPluginPerSecond: 10, PerTopicPerSecond: 10, Capacity: 1, DeadLetterCapacity: 8}, clock)
	if err := limiter.Admit("p", "t"); err != nil {
		t.Fatalf("first: %v", err)
	}
	if err := limiter.Admit("p", "t2"); err == nil {
		t.Fatalf("should be full")
	}
	limiter.Complete(1)
	if err := limiter.Admit("p", "t2"); err != nil {
		t.Fatalf("after complete: %v", err)
	}
	fmt.Printf("m07-s14 complete frees=true\n")
}
