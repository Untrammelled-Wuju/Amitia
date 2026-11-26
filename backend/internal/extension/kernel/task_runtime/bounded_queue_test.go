//go:build rsi_backpressure

package task_runtime

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	schedqueue "github.com/u-ai/backend/internal/queue"
)

type backpressureClock struct {
	mu sync.Mutex
	at time.Time
}

func (c *backpressureClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.at
}

func (c *backpressureClock) Advance(delta time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.at = c.at.Add(delta)
}

func boundedItem(id string, priority QueuePriority, bytes int64) QueueItem {
	return QueueItem{ID: id, Priority: priority, Bytes: bytes}
}

func TestBoundedQueueExistingPriorityQueueOverflows(t *testing.T) {
	legacy := schedqueue.NewPriorityQueue(schedqueue.PriorityQueueConfig{MaxQueueSize: 16})
	for i := 0; i < 100; i++ {
		accepted, reason := legacy.Enqueue(&schedqueue.Task{ID: fmt.Sprintf("p0-%03d", i), Priority: schedqueue.PriorityP0})
		if !accepted || reason != "" {
			t.Fatalf("legacy enqueue %d accepted=%v reason=%s", i, accepted, reason)
		}
	}
	if legacy.Depth() != 100 {
		t.Fatalf("legacy depth=%d", legacy.Depth())
	}
	fmt.Printf("m07-s03 existing max=16 depth=100 overflow=true\n")
}

func TestBoundedQueueCapacityBound(t *testing.T) {
	q := NewBoundedQueue(BoundedQueueConfig{MaxItems: 256, MaxBytes: 1 << 20, MaxItemBytes: 4096})
	accepted := 0
	rejected := 0
	for i := 0; i < 10000; i++ {
		err := q.Enqueue(context.Background(), boundedItem(fmt.Sprintf("evt-%05d", i), PriorityUserChat, 64))
		if err == nil {
			accepted++
			continue
		}
		if !errors.Is(err, ErrQueueFull) {
			t.Fatalf("enqueue %d err=%v", i, err)
		}
		rejected++
	}
	if accepted != 256 || rejected != 9744 || q.Len() != 256 {
		t.Fatalf("accepted=%d rejected=%d depth=%d", accepted, rejected, q.Len())
	}
	fmt.Printf("m07-s03 capacity accepted=256 rejected=9744 depth=256\n")
}

func TestBoundedQueueByteBudget(t *testing.T) {
	q := NewBoundedQueue(BoundedQueueConfig{MaxItems: 100, MaxBytes: 4096, MaxItemBytes: 512})
	accepted := 0
	for i := 0; i < 8; i++ {
		if err := q.Enqueue(context.Background(), boundedItem(fmt.Sprintf("exp-%d", i), PriorityBackground, 512)); err != nil {
			t.Fatalf("enqueue %d: %v", i, err)
		}
		accepted++
	}
	err := q.Enqueue(context.Background(), boundedItem("exp-8", PriorityBackground, 512))
	if !errors.Is(err, ErrQueueFull) {
		t.Fatalf("byte budget err=%v", err)
	}
	if accepted != 8 || q.Bytes() != 4096 {
		t.Fatalf("accepted=%d bytes=%d", accepted, q.Bytes())
	}
	fmt.Printf("m07-s03 bytes accepted=8 bytes=4096 limit=4096\n")
}

func TestBoundedQueueOversizeRejected(t *testing.T) {
	q := NewBoundedQueue(BoundedQueueConfig{MaxItems: 10, MaxBytes: 1 << 20, MaxItemBytes: 64 << 10})
	err := q.Enqueue(context.Background(), boundedItem("huge", PriorityUserChat, 1<<20))
	if !errors.Is(err, ErrQueueItemTooLarge) || errors.Is(err, ErrQueueFull) {
		t.Fatalf("oversize err=%v", err)
	}
	var full *QueueFullError
	if errors.As(err, &full) {
		t.Fatalf("oversize reported retryable")
	}
	if q.Len() != 0 {
		t.Fatalf("depth=%d", q.Len())
	}
	fmt.Printf("m07-s03 oversize rejected=true retryable=false\n")
}

func TestBoundedQueueRetryableRejection(t *testing.T) {
	q := NewBoundedQueue(BoundedQueueConfig{MaxItems: 4, MaxBytes: 1 << 20, MaxItemBytes: 4096, RetryAfter: 25 * time.Millisecond})
	for i := 0; i < 4; i++ {
		if err := q.Enqueue(context.Background(), boundedItem(fmt.Sprintf("bg-%d", i), PriorityBackground, 8)); err != nil {
			t.Fatalf("enqueue %d: %v", i, err)
		}
	}
	err := q.Enqueue(context.Background(), boundedItem("exp", PriorityExperiment, 8))
	if !errors.Is(err, ErrQueueFull) {
		t.Fatalf("full err=%v", err)
	}
	var full *QueueFullError
	if !errors.As(err, &full) || !full.Retryable() || full.RetryAfter != 25*time.Millisecond {
		t.Fatalf("retryable error=%+v", full)
	}
	if !IsTaskErrorCode(err, ErrTaskQueueFull) || HTTPStatusForErrorCode(ErrTaskQueueFull) != 429 {
		t.Fatalf("task error code not wired: %v", err)
	}
	if q.Len() != 4 {
		t.Fatalf("depth=%d", q.Len())
	}
	fmt.Printf("m07-s03 retryable=true retry_after=25ms http=429 depth=4\n")
}

func TestBoundedQueueUserPreemptsExperiment(t *testing.T) {
	q := NewBoundedQueue(BoundedQueueConfig{MaxItems: 4, MaxBytes: 1 << 20, MaxItemBytes: 4096})
	for i := 0; i < 4; i++ {
		if err := q.Enqueue(context.Background(), boundedItem(fmt.Sprintf("exp-%d", i), PriorityExperiment, 8)); err != nil {
			t.Fatalf("enqueue %d: %v", i, err)
		}
	}
	if err := q.Enqueue(context.Background(), boundedItem("chat", PriorityUserChat, 8)); err != nil {
		t.Fatalf("preempt enqueue: %v", err)
	}
	stats := q.Stats()
	if q.Len() != 4 || stats.Preempted != 1 || stats.LastEvict != "exp-0" || len(q.Pending(PriorityExperiment)) != 3 {
		t.Fatalf("depth=%d stats=%+v experiments=%v", q.Len(), stats, q.Pending(PriorityExperiment))
	}
	fmt.Printf("m07-s03 preempt user=true evicted=1 depth=4\n")
}

func TestBoundedQueuePeerNotPreempted(t *testing.T) {
	q := NewBoundedQueue(BoundedQueueConfig{MaxItems: 4, MaxBytes: 1 << 20, MaxItemBytes: 4096})
	for i := 0; i < 4; i++ {
		if err := q.Enqueue(context.Background(), boundedItem(fmt.Sprintf("chat-%d", i), PriorityUserChat, 8)); err != nil {
			t.Fatalf("enqueue %d: %v", i, err)
		}
	}
	err := q.Enqueue(context.Background(), boundedItem("exp", PriorityExperiment, 8))
	if !errors.Is(err, ErrQueueFull) {
		t.Fatalf("peer err=%v", err)
	}
	if q.Len() != 4 || q.Stats().Preempted != 0 {
		t.Fatalf("depth=%d stats=%+v", q.Len(), q.Stats())
	}
	fmt.Printf("m07-s03 no_preempt peer=true depth=4\n")
}

func TestBoundedQueuePriorityOrder(t *testing.T) {
	q := NewBoundedQueue(BoundedQueueConfig{MaxItems: 16, MaxBytes: 1 << 20, MaxItemBytes: 4096})
	order := []struct {
		id       string
		priority QueuePriority
	}{
		{"bg", PriorityBackground},
		{"exp", PriorityExperiment},
		{"fg", PriorityUserForeground},
		{"chat", PriorityUserChat},
	}
	for _, entry := range order {
		if err := q.Enqueue(context.Background(), boundedItem(entry.id, entry.priority, 8)); err != nil {
			t.Fatalf("enqueue %s: %v", entry.id, err)
		}
	}
	got := make([]string, 0, len(order))
	for i := 0; i < len(order); i++ {
		item, ok, err := q.Dequeue(context.Background())
		if err != nil || !ok {
			t.Fatalf("dequeue %d ok=%v err=%v", i, ok, err)
		}
		got = append(got, item.ID)
	}
	if fmt.Sprintf("%v", got) != "[chat fg exp bg]" {
		t.Fatalf("order=%v", got)
	}
	fmt.Printf("m07-s03 order=chat,foreground,experiment,background\n")
}

func TestBoundedQueueAgingFairness(t *testing.T) {
	clock := &backpressureClock{at: time.Unix(1700000000, 0)}
	q := NewBoundedQueue(BoundedQueueConfig{MaxItems: 100, MaxBytes: 1 << 20, MaxItemBytes: 4096, AgingAfter: time.Second}).WithClock(clock.Now)
	if err := q.Enqueue(context.Background(), boundedItem("bg", PriorityBackground, 8)); err != nil {
		t.Fatalf("enqueue bg: %v", err)
	}
	clock.Advance(2 * time.Second)
	if err := q.Enqueue(context.Background(), boundedItem("fg", PriorityUserForeground, 8)); err != nil {
		t.Fatalf("enqueue fg: %v", err)
	}
	item, ok, err := q.Dequeue(context.Background())
	if err != nil || !ok || item.ID != "bg" {
		t.Fatalf("first=%s ok=%v err=%v", item.ID, ok, err)
	}
	fmt.Printf("m07-s03 aging promoted=true wait=1\n")
}

func TestBoundedQueueAgedDoesNotOutrankChat(t *testing.T) {
	clock := &backpressureClock{at: time.Unix(1700000000, 0)}
	q := NewBoundedQueue(BoundedQueueConfig{MaxItems: 100, MaxBytes: 1 << 20, MaxItemBytes: 4096, AgingAfter: time.Second}).WithClock(clock.Now)
	if err := q.Enqueue(context.Background(), boundedItem("bg", PriorityBackground, 8)); err != nil {
		t.Fatalf("enqueue bg: %v", err)
	}
	clock.Advance(10 * time.Second)
	if err := q.Enqueue(context.Background(), boundedItem("chat", PriorityUserChat, 8)); err != nil {
		t.Fatalf("enqueue chat: %v", err)
	}
	item, ok, err := q.Dequeue(context.Background())
	if err != nil || !ok || item.ID != "chat" {
		t.Fatalf("first=%s ok=%v err=%v", item.ID, ok, err)
	}
	fmt.Printf("m07-s03 aging user_guard=true\n")
}

func TestBoundedQueueReservedUserSlots(t *testing.T) {
	q := NewBoundedQueue(BoundedQueueConfig{MaxItems: 8, MaxBytes: 1 << 20, MaxItemBytes: 4096, ReservedUserSlots: 2})
	accepted := 0
	for i := 0; i < 6; i++ {
		if err := q.Enqueue(context.Background(), boundedItem(fmt.Sprintf("exp-%d", i), PriorityExperiment, 8)); err != nil {
			t.Fatalf("enqueue %d: %v", i, err)
		}
		accepted++
	}
	err := q.Enqueue(context.Background(), boundedItem("exp-6", PriorityExperiment, 8))
	if !errors.Is(err, ErrQueueFull) {
		t.Fatalf("reserved err=%v", err)
	}
	for i := 0; i < 2; i++ {
		if err := q.Enqueue(context.Background(), boundedItem(fmt.Sprintf("chat-%d", i), PriorityUserChat, 8)); err != nil {
			t.Fatalf("chat %d: %v", i, err)
		}
	}
	if accepted != 6 || q.Len() != 8 {
		t.Fatalf("accepted=%d depth=%d", accepted, q.Len())
	}
	fmt.Printf("m07-s03 reserve experiments=6 rejected=1 chat=2\n")
}

func TestBoundedQueuePressureAndShed(t *testing.T) {
	q := NewBoundedQueue(BoundedQueueConfig{MaxItems: 10, MaxBytes: 1 << 20, MaxItemBytes: 4096})
	for i := 0; i < 5; i++ {
		if err := q.Enqueue(context.Background(), boundedItem(fmt.Sprintf("bg-%d", i), PriorityBackground, 8)); err != nil {
			t.Fatalf("enqueue %d: %v", i, err)
		}
	}
	medium := q.Pressure()
	for i := 5; i < 8; i++ {
		if err := q.Enqueue(context.Background(), boundedItem(fmt.Sprintf("bg-%d", i), PriorityBackground, 8)); err != nil {
			t.Fatalf("enqueue %d: %v", i, err)
		}
	}
	high := q.Pressure()
	highShed := q.Admit(PriorityBackground)
	highChat := q.Admit(PriorityUserChat)
	for i := 8; i < 9; i++ {
		if err := q.Enqueue(context.Background(), boundedItem(fmt.Sprintf("bg-%d", i), PriorityBackground, 8)); err != nil {
			t.Fatalf("enqueue %d: %v", i, err)
		}
	}
	critical := q.Pressure()
	criticalShed := q.Admit(PriorityExperiment)
	criticalChat := q.Admit(PriorityUserChat)
	if medium != PressureMedium || high != PressureHigh || critical != PressureCritical {
		t.Fatalf("levels=%s,%s,%s", medium, high, critical)
	}
	if !errors.Is(highShed, ErrQueueFull) || highChat != nil || !errors.Is(criticalShed, ErrQueueFull) || criticalChat != nil {
		t.Fatalf("shed high=%v highChat=%v critical=%v criticalChat=%v", highShed, highChat, criticalShed, criticalChat)
	}
	fmt.Printf("m07-s03 pressure medium=true high=true critical=true shed=true\n")
}

func TestBoundedQueueConcurrentEnqueue(t *testing.T) {
	q := NewBoundedQueue(BoundedQueueConfig{MaxItems: 128, MaxBytes: 1 << 20, MaxItemBytes: 4096})
	var wait sync.WaitGroup
	for worker := 0; worker < 32; worker++ {
		wait.Add(1)
		go func(worker int) {
			defer wait.Done()
			for i := 0; i < 50; i++ {
				_ = q.Enqueue(context.Background(), boundedItem(fmt.Sprintf("w%02d-%03d", worker, i), PriorityUserChat, 8))
			}
		}(worker)
	}
	wait.Wait()
	stats := q.Stats()
	if stats.Depth != 128 || stats.Accepted != 128 || stats.Rejected != 1472 {
		t.Fatalf("stats=%+v", stats)
	}
	fmt.Printf("m07-s03 concurrent goroutines=32 accepted=128 depth=128\n")
}

func TestBoundedQueueCancel(t *testing.T) {
	q := NewBoundedQueue(BoundedQueueConfig{MaxItems: 4, MaxBytes: 1 << 20, MaxItemBytes: 4096})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := q.Enqueue(ctx, boundedItem("chat", PriorityUserChat, 8)); !errors.Is(err, context.Canceled) {
		t.Fatalf("enqueue err=%v", err)
	}
	if q.Len() != 0 {
		t.Fatalf("depth=%d", q.Len())
	}
	if _, ok, err := q.Dequeue(ctx); !errors.Is(err, context.Canceled) || ok {
		t.Fatalf("dequeue ok=%v err=%v", ok, err)
	}
	fmt.Printf("m07-s03 cancel enqueue=context.Canceled dequeue=context.Canceled depth=0\n")
}
