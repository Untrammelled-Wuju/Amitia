//go:build rsi_ratelimit

package task_runtime

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

var (
	ErrRateLimitInvalid = errors.New("task_runtime: rate limit invalid")
	ErrRateLimited      = errors.New("task_runtime: rate limited")
)

type RateLimitConfig struct {
	PerPluginPerSecond int
	PerTopicPerSecond  int
	Capacity           int
	DeadLetterCapacity int
}

func (c RateLimitConfig) validate() error {
	if c.PerPluginPerSecond <= 0 || c.PerTopicPerSecond <= 0 || c.Capacity <= 0 {
		return fmt.Errorf("%w: plugin=%d topic=%d capacity=%d", ErrRateLimitInvalid, c.PerPluginPerSecond, c.PerTopicPerSecond, c.Capacity)
	}
	if c.DeadLetterCapacity < 0 {
		return fmt.Errorf("%w: deadLetterCapacity=%d", ErrRateLimitInvalid, c.DeadLetterCapacity)
	}
	return nil
}

type RateLimitError struct {
	Scope      string
	RetryAfter time.Duration
}

func (e *RateLimitError) Error() string {
	return fmt.Sprintf("%v: scope=%s retry_after=%s", ErrRateLimited, e.Scope, e.RetryAfter)
}

func (e *RateLimitError) Is(target error) bool {
	return target == ErrRateLimited
}

func (e *RateLimitError) Unwrap() error {
	return WrapTaskError(ErrTaskQueueFull, "event rate limited", ErrRateLimited)
}

func (e *RateLimitError) Retryable() bool {
	return e.RetryAfter > 0
}

type DeadLetter struct {
	Seq     int64
	Plugin  string
	Topic   string
	Reason  string
	Count   int
	FirstAt time.Time
	LastAt  time.Time
}

type rateWindow struct {
	start time.Time
	count int
}

type EventLimiter struct {
	mu              sync.Mutex
	config          RateLimitConfig
	now             func() time.Time
	pluginWindows   map[string]*rateWindow
	topicWindows    map[string]*rateWindow
	pending         int
	deadLetters     []DeadLetter
	deadSeq         int64
	deadDropped     int64
}

func NewEventLimiter(config RateLimitConfig) (*EventLimiter, error) {
	if err := config.validate(); err != nil {
		return nil, err
	}
	return &EventLimiter{
		config:        config,
		now:           time.Now,
		pluginWindows: map[string]*rateWindow{},
		topicWindows:  map[string]*rateWindow{},
	}, nil
}

func (l *EventLimiter) WithClock(now func() time.Time) *EventLimiter {
	l.mu.Lock()
	defer l.mu.Unlock()
	if now != nil {
		l.now = now
	}
	return l
}

func (l *EventLimiter) windowLocked(windows map[string]*rateWindow, key string, limit int, now time.Time) (bool, time.Duration) {
	window, ok := windows[key]
	if !ok || now.Sub(window.start) >= time.Second {
		windows[key] = &rateWindow{start: now}
		return true, 0
	}
	if window.count >= limit {
		return false, window.start.Add(time.Second).Sub(now)
	}
	return true, 0
}

func (l *EventLimiter) Admit(plugin, topic string) error {
	plugin = strings.TrimSpace(plugin)
	topic = strings.TrimSpace(topic)
	if plugin == "" || topic == "" {
		return fmt.Errorf("%w: plugin=%q topic=%q", ErrRateLimitInvalid, plugin, topic)
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	if l.pending >= l.config.Capacity {
		l.recordDeadLocked(plugin, topic, "queue_full", now)
		return &RateLimitError{Scope: "queue", RetryAfter: time.Second}
	}
	if ok, wait := l.windowLocked(l.pluginWindows, plugin, l.config.PerPluginPerSecond, now); !ok {
		l.recordDeadLocked(plugin, topic, "plugin_rate", now)
		return &RateLimitError{Scope: "plugin", RetryAfter: wait}
	}
	if ok, wait := l.windowLocked(l.topicWindows, topic, l.config.PerTopicPerSecond, now); !ok {
		l.recordDeadLocked(plugin, topic, "topic_rate", now)
		return &RateLimitError{Scope: "topic", RetryAfter: wait}
	}
	l.pluginWindows[plugin].count++
	l.topicWindows[topic].count++
	l.pending++
	return nil
}

func (l *EventLimiter) Complete(count int) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.pending -= count
	if l.pending < 0 {
		l.pending = 0
	}
}

func (l *EventLimiter) recordDeadLocked(plugin, topic, reason string, now time.Time) {
	if l.config.DeadLetterCapacity == 0 {
		return
	}
	for index := range l.deadLetters {
		record := &l.deadLetters[index]
		if record.Plugin == plugin && record.Topic == topic && record.Reason == reason {
			record.Count++
			record.LastAt = now
			return
		}
	}
	l.deadSeq++
	record := DeadLetter{Seq: l.deadSeq, Plugin: plugin, Topic: topic, Reason: reason, Count: 1, FirstAt: now, LastAt: now}
	if len(l.deadLetters) >= l.config.DeadLetterCapacity {
		l.deadLetters = l.deadLetters[1:]
		l.deadDropped++
	}
	l.deadLetters = append(l.deadLetters, record)
}

func (l *EventLimiter) Trace(seq int64) (DeadLetter, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, record := range l.deadLetters {
		if record.Seq == seq {
			return record, true
		}
	}
	return DeadLetter{}, false
}

func (l *EventLimiter) DeadLetters() []DeadLetter {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]DeadLetter(nil), l.deadLetters...)
}

func (l *EventLimiter) DeadDropped() int64 {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.deadDropped
}

func (l *EventLimiter) Pending() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.pending
}
