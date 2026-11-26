//go:build rsi_backpressure && rsi_stress

package task_runtime

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"sync"
	"testing"
	"time"
)

var errInjectedOOM = errors.New("injected oom")

type stressBreaker struct {
	mu        sync.Mutex
	failures  int
	threshold int
	open      bool
}

func (b *stressBreaker) Allow() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return !b.open
}

func (b *stressBreaker) Fail() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.failures++
	if b.failures >= b.threshold {
		b.open = true
	}
}

func (b *stressBreaker) Open() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.open
}

func runOOMPlugin(budget int64) error {
	allocated := int64(0)
	for {
		if allocated+64<<10 > budget {
			return errInjectedOOM
		}
		allocated += 64 << 10
		buffer := make([]byte, 64<<10)
		buffer[0] = 1
	}
}

func runSpinPlugin(ctx context.Context) error {
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
	}
}

func TestCoreStressInjection(t *testing.T) {
	queue := NewBoundedQueue(BoundedQueueConfig{MaxItems: 1024, MaxBytes: 4 << 20, MaxItemBytes: 4096})
	var before runtime.MemStats
	runtime.ReadMemStats(&before)
	goroutinesBefore := runtime.NumGoroutine()
	accepted := 0
	rejected := 0
	var maxLatency time.Duration
	for i := 0; i < 10000; i++ {
		start := time.Now()
		err := queue.Enqueue(context.Background(), QueueItem{ID: fmt.Sprintf("evt-%05d", i), Priority: PriorityUserChat, Bytes: 64})
		if elapsed := time.Since(start); elapsed > maxLatency {
			maxLatency = elapsed
		}
		if err == nil {
			accepted++
			continue
		}
		if !errors.Is(err, ErrQueueFull) {
			t.Fatalf("storm enqueue %d: %v", i, err)
		}
		rejected++
	}
	if accepted != 1024 || rejected != 8976 || queue.Len() != 1024 {
		t.Fatalf("accepted=%d rejected=%d depth=%d", accepted, rejected, queue.Len())
	}
	var after runtime.MemStats
	runtime.ReadMemStats(&after)
	heapDelta := int64(after.HeapAlloc) - int64(before.HeapAlloc)
	goroutinesDelta := runtime.NumGoroutine() - goroutinesBefore
	if heapDelta > 64<<20 {
		t.Fatalf("heap delta=%d", heapDelta)
	}
	if goroutinesDelta > 8 {
		t.Fatalf("goroutines delta=%d", goroutinesDelta)
	}
	if maxLatency > 50*time.Millisecond {
		t.Fatalf("max enqueue latency=%s", maxLatency)
	}
	fmt.Printf("m07-s07 storm events=10000 accepted=1024 rejected=8976 depth=1024\n")
	fmt.Printf("m07-s07 memory within_budget=true limit_mb=64\n")
	fmt.Printf("m07-s07 latency within_budget=true budget_ms=50\n")
	fmt.Printf("m07-s07 goroutines within_budget=true delta=%d\n", goroutinesDelta)

	breaker := &stressBreaker{threshold: 3}
	userQueue := NewBoundedQueue(BoundedQueueConfig{MaxItems: 8, MaxBytes: 1 << 20, MaxItemBytes: 4096})
	pluginCalls := 0
	for i := 0; i < 3; i++ {
		pluginCalls++
		if err := runOOMPlugin(256 << 10); !errors.Is(err, errInjectedOOM) {
			t.Fatalf("oom injection err=%v", err)
		}
		breaker.Fail()
	}
	if !breaker.Open() || breaker.Allow() {
		t.Fatalf("breaker state open=%v allow=%v", breaker.Open(), breaker.Allow())
	}
	userStart := time.Now()
	if err := userQueue.Enqueue(context.Background(), QueueItem{ID: "user-chat", Priority: PriorityUserChat, Bytes: 8}); err != nil {
		t.Fatalf("user enqueue: %v", err)
	}
	userItem, ok, err := userQueue.Dequeue(context.Background())
	if err != nil || !ok || userItem.ID != "user-chat" || time.Since(userStart) > 50*time.Millisecond {
		t.Fatalf("user path item=%s ok=%v err=%v elapsed=%s", userItem.ID, ok, err, time.Since(userStart))
	}
	fmt.Printf("m07-s07 oom breaker_open=true plugin_calls=%d user_admitted=true\n", pluginCalls)

	spinCtx, cancel := context.WithTimeout(context.Background(), 5*time.Millisecond)
	spinErr := runSpinPlugin(spinCtx)
	cancel()
	if !errors.Is(spinErr, context.DeadlineExceeded) {
		t.Fatalf("spin err=%v", spinErr)
	}
	breaker.Fail()
	fmt.Printf("m07-s07 spin canceled=true breaker_open=true\n")

	blocked := NewBoundedQueue(BoundedQueueConfig{MaxItems: 4, MaxBytes: 1 << 20, MaxItemBytes: 4096})
	for i := 0; i < 4; i++ {
		if err := blocked.Enqueue(context.Background(), QueueItem{ID: fmt.Sprintf("bg-%d", i), Priority: PriorityExperiment, Bytes: 8}); err != nil {
			t.Fatalf("block fill %d: %v", i, err)
		}
	}
	start := time.Now()
	if err := blocked.Enqueue(context.Background(), QueueItem{ID: "user", Priority: PriorityUserChat, Bytes: 8}); err != nil {
		t.Fatalf("blocked user enqueue: %v", err)
	}
	item, ok, err := blocked.Dequeue(context.Background())
	if err != nil || !ok || item.ID != "user" || time.Since(start) > 50*time.Millisecond {
		t.Fatalf("blocked item=%s ok=%v err=%v elapsed=%s", item.ID, ok, err, time.Since(start))
	}
	fmt.Printf("m07-s07 blocked user_admitted=true\n")

	if err := blocked.Enqueue(context.Background(), QueueItem{ID: "", Priority: PriorityUserChat, Bytes: 8}); !errors.Is(err, ErrQueueInvalid) {
		t.Fatalf("invalid err=%v", err)
	}
	cancelled, cancelDispatch := context.WithCancel(context.Background())
	cancelDispatch()
	if _, _, err := blocked.Dequeue(cancelled); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancel dequeue err=%v", err)
	}
	fmt.Printf("m07-s07 invalid rejected=true cancel=true\n")

	restarted := NewBoundedQueue(BoundedQueueConfig{MaxItems: 16, MaxBytes: 1 << 20, MaxItemBytes: 4096})
	if restarted.Len() != 0 || restarted.Stats().Accepted != 0 {
		t.Fatalf("restart state=%+v", restarted.Stats())
	}
	fmt.Printf("m07-s07 restart depth=0\n")
}
