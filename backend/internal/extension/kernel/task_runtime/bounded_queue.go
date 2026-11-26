//go:build rsi_backpressure

package task_runtime

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

type QueuePriority int

const (
	PriorityUserChat QueuePriority = iota
	PriorityUserForeground
	PriorityExperiment
	PriorityBackground
)

func (p QueuePriority) String() string {
	switch p {
	case PriorityUserChat:
		return "user_chat"
	case PriorityUserForeground:
		return "user_foreground"
	case PriorityExperiment:
		return "experiment"
	case PriorityBackground:
		return "background"
	default:
		return "unknown"
	}
}

func (p QueuePriority) valid() bool {
	return p >= PriorityUserChat && p <= PriorityBackground
}

var (
	ErrQueueInvalid      = errors.New("task_runtime: queue item invalid")
	ErrQueueItemTooLarge = errors.New("task_runtime: queue item exceeds byte budget")
	ErrQueueFull         = errors.New("task_runtime: queue full")
)

type QueueFullError struct {
	Priority   QueuePriority
	Depth      int
	MaxItems   int
	Bytes      int64
	MaxBytes   int64
	RetryAfter time.Duration
	task       *TaskError
}

func (e *QueueFullError) Error() string {
	return fmt.Sprintf("%v: priority=%s depth=%d/%d bytes=%d/%d retry_after=%s", ErrQueueFull, e.Priority, e.Depth, e.MaxItems, e.Bytes, e.MaxBytes, e.RetryAfter)
}

func (e *QueueFullError) Is(target error) bool {
	return target == ErrQueueFull
}

func (e *QueueFullError) Retryable() bool {
	return e.RetryAfter > 0
}

func (e *QueueFullError) Unwrap() error {
	return e.task
}

type QueueItem struct {
	ID         string
	Priority   QueuePriority
	Bytes      int64
	Scope      string
	EnqueuedAt time.Time
	Attempt    int
}

type BoundedQueueConfig struct {
	MaxItems          int
	MaxBytes          int64
	MaxItemBytes      int64
	AgingAfter        time.Duration
	ReservedUserSlots int
	RetryAfter        time.Duration
}

func (c BoundedQueueConfig) withDefaults() BoundedQueueConfig {
	if c.MaxItems <= 0 {
		c.MaxItems = 1024
	}
	if c.MaxBytes <= 0 {
		c.MaxBytes = 64 << 20
	}
	if c.MaxItemBytes <= 0 {
		c.MaxItemBytes = c.MaxBytes
	}
	if c.ReservedUserSlots < 0 {
		c.ReservedUserSlots = 0
	}
	if c.ReservedUserSlots >= c.MaxItems {
		c.ReservedUserSlots = c.MaxItems - 1
	}
	if c.RetryAfter <= 0 {
		c.RetryAfter = 25 * time.Millisecond
	}
	return c
}

type PressureLevel int

const (
	PressureLow PressureLevel = iota
	PressureMedium
	PressureHigh
	PressureCritical
)

func (p PressureLevel) String() string {
	switch p {
	case PressureLow:
		return "low"
	case PressureMedium:
		return "medium"
	case PressureHigh:
		return "high"
	case PressureCritical:
		return "critical"
	default:
		return "unknown"
	}
}

type BoundedQueueStats struct {
	Depth     int
	Bytes     int64
	Accepted  int64
	Rejected  int64
	Preempted int64
	Served    int64
	LastEvict string
}

type boundedEntry struct {
	item       QueueItem
	enqueuedAt time.Time
}

type BoundedQueue struct {
	mu      sync.Mutex
	config  BoundedQueueConfig
	entries []boundedEntry
	bytes   int64
	now     func() time.Time
	stats   BoundedQueueStats
}

func NewBoundedQueue(config BoundedQueueConfig) *BoundedQueue {
	return &BoundedQueue{config: config.withDefaults(), now: time.Now}
}

func (q *BoundedQueue) WithClock(now func() time.Time) *BoundedQueue {
	q.mu.Lock()
	defer q.mu.Unlock()
	if now != nil {
		q.now = now
	}
	return q
}

func (q *BoundedQueue) Enqueue(ctx context.Context, item QueueItem) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	item.ID = strings.TrimSpace(item.ID)
	if item.ID == "" || !item.Priority.valid() || item.Bytes <= 0 {
		return fmt.Errorf("%w: id=%q priority=%d bytes=%d", ErrQueueInvalid, item.ID, item.Priority, item.Bytes)
	}
	if item.Bytes > q.config.MaxItemBytes {
		return fmt.Errorf("%w: id=%s bytes=%d limit=%d", ErrQueueItemTooLarge, item.ID, item.Bytes, q.config.MaxItemBytes)
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	if item.Priority > PriorityUserForeground {
		if q.stats.Depth >= q.config.MaxItems-q.config.ReservedUserSlots {
			q.stats.Rejected++
			return q.fullLocked(item.Priority)
		}
	}
	if q.stats.Depth < q.config.MaxItems && q.bytes+item.Bytes <= q.config.MaxBytes {
		q.appendLocked(item)
		return nil
	}
	if item.Priority <= PriorityUserForeground {
		if victim := q.evictLowerLocked(item.Priority); victim != "" {
			q.stats.LastEvict = victim
			q.stats.Preempted++
			q.appendLocked(item)
			return nil
		}
	}
	q.stats.Rejected++
	return q.fullLocked(item.Priority)
}

func (q *BoundedQueue) appendLocked(item QueueItem) {
	now := q.now()
	q.entries = append(q.entries, boundedEntry{item: item, enqueuedAt: now})
	q.bytes += item.Bytes
	q.stats.Depth = len(q.entries)
	q.stats.Bytes = q.bytes
	q.stats.Accepted++
}

func (q *BoundedQueue) evictLowerLocked(priority QueuePriority) string {
	best := -1
	for index, entry := range q.entries {
		if entry.item.Priority <= priority {
			continue
		}
		if best < 0 || entry.item.Priority > q.entries[best].item.Priority {
			best = index
		}
	}
	if best < 0 {
		return ""
	}
	victim := q.entries[best]
	q.entries = append(q.entries[:best], q.entries[best+1:]...)
	q.bytes -= victim.item.Bytes
	q.stats.Depth = len(q.entries)
	q.stats.Bytes = q.bytes
	return victim.item.ID
}

func (q *BoundedQueue) fullLocked(priority QueuePriority) error {
	return &QueueFullError{
		Priority:   priority,
		Depth:      q.stats.Depth,
		MaxItems:   q.config.MaxItems,
		Bytes:      q.bytes,
		MaxBytes:   q.config.MaxBytes,
		RetryAfter: q.config.RetryAfter,
		task:       WrapTaskError(ErrTaskQueueFull, "queue full", ErrQueueFull),
	}
}

func (q *BoundedQueue) Dequeue(ctx context.Context) (QueueItem, bool, error) {
	if err := ctx.Err(); err != nil {
		return QueueItem{}, false, err
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	if len(q.entries) == 0 {
		return QueueItem{}, false, nil
	}
	now := q.now()
	best := 0
	for index := 1; index < len(q.entries); index++ {
		if q.beforeLocked(index, best, now) {
			best = index
		}
	}
	entry := q.entries[best]
	q.entries = append(q.entries[:best], q.entries[best+1:]...)
	q.bytes -= entry.item.Bytes
	q.stats.Depth = len(q.entries)
	q.stats.Bytes = q.bytes
	q.stats.Served++
	entry.item.EnqueuedAt = entry.enqueuedAt
	return entry.item, true, nil
}

func (q *BoundedQueue) effectivePriority(entry boundedEntry, now time.Time) QueuePriority {
	if q.config.AgingAfter <= 0 {
		return entry.item.Priority
	}
	steps := int(now.Sub(entry.enqueuedAt) / q.config.AgingAfter)
	rank := int(entry.item.Priority) - steps
	if entry.item.Priority > PriorityUserChat && rank < int(PriorityUserForeground) {
		rank = int(PriorityUserForeground)
	}
	return QueuePriority(rank)
}

func (q *BoundedQueue) beforeLocked(left, right int, now time.Time) bool {
	leftPriority := q.effectivePriority(q.entries[left], now)
	rightPriority := q.effectivePriority(q.entries[right], now)
	if leftPriority != rightPriority {
		return leftPriority < rightPriority
	}
	if !q.entries[left].enqueuedAt.Equal(q.entries[right].enqueuedAt) {
		return q.entries[left].enqueuedAt.Before(q.entries[right].enqueuedAt)
	}
	return q.entries[left].item.ID < q.entries[right].item.ID
}

func (q *BoundedQueue) Pressure() PressureLevel {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.pressureLocked()
}

func (q *BoundedQueue) pressureLocked() PressureLevel {
	itemRatio := float64(q.stats.Depth) / float64(q.config.MaxItems)
	byteRatio := float64(q.bytes) / float64(q.config.MaxBytes)
	ratio := itemRatio
	if byteRatio > ratio {
		ratio = byteRatio
	}
	switch {
	case ratio >= 0.9:
		return PressureCritical
	case ratio >= 0.75:
		return PressureHigh
	case ratio >= 0.5:
		return PressureMedium
	default:
		return PressureLow
	}
}

func (q *BoundedQueue) Admit(priority QueuePriority) error {
	if !priority.valid() {
		return fmt.Errorf("%w: priority=%d", ErrQueueInvalid, priority)
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	level := q.pressureLocked()
	if level == PressureHigh && priority >= PriorityExperiment {
		return q.fullLocked(priority)
	}
	if level == PressureCritical && priority >= PriorityUserForeground {
		return q.fullLocked(priority)
	}
	return nil
}

func (q *BoundedQueue) Stats() BoundedQueueStats {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.stats
}

func (q *BoundedQueue) Pending(priority QueuePriority) []string {
	q.mu.Lock()
	defer q.mu.Unlock()
	ids := make([]string, 0, len(q.entries))
	for _, entry := range q.entries {
		if entry.item.Priority == priority {
			ids = append(ids, entry.item.ID)
		}
	}
	sort.Strings(ids)
	return ids
}

func (q *BoundedQueue) Len() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.entries)
}

func (q *BoundedQueue) Bytes() int64 {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.bytes
}
