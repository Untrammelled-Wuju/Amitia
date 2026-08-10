package control

import (
	"sync"
	"testing"
	"time"
)

type BoundedOutputQueue struct {
	mu         sync.Mutex
	items      []QueuedOutput
	maxSize    int
	dropped    int
	maxEpoch   uint64
	windowSize int
}

type QueuedOutput struct {
	OutputID  string
	RuntimeID string
	Epoch     uint64
	Enqueued  time.Time
}

func NewBoundedOutputQueue(maxSize int) *BoundedOutputQueue {
	return &BoundedOutputQueue{
		items:   make([]QueuedOutput, 0, maxSize),
		maxSize: maxSize,
	}
}

func (q *BoundedOutputQueue) Enqueue(output QueuedOutput) bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	if len(q.items) >= q.maxSize {
		q.dropped++
		return false
	}
	q.items = append(q.items, output)
	return true
}

func (q *BoundedOutputQueue) PurgeStale(currentEpoch uint64) int {
	q.mu.Lock()
	defer q.mu.Unlock()
	writeIdx := 0
	for _, item := range q.items {
		if item.Epoch == currentEpoch {
			q.items[writeIdx] = item
			writeIdx++
		}
	}
	removed := len(q.items) - writeIdx
	q.items = q.items[:writeIdx]
	return removed
}

func (q *BoundedOutputQueue) Len() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.items)
}

func TestBoundedOutputQueue_RespectsMaxSize(t *testing.T) {
	q := NewBoundedOutputQueue(10)
	for i := 0; i < 20; i++ {
		_ = q.Enqueue(QueuedOutput{
			OutputID: "out-" + string(rune('a'+i%26)),
			Epoch:    1,
			Enqueued: time.Now().UTC(),
		})
	}
	if q.Len() != 10 {
		t.Fatalf("expected queue bounded to 10, got %d", q.Len())
	}
	if q.dropped != 10 {
		t.Fatalf("expected 10 dropped, got %d", q.dropped)
	}
}

func TestBoundedOutputQueue_PurgeStale(t *testing.T) {
	q := NewBoundedOutputQueue(100)
	for i := 0; i < 50; i++ {
		_ = q.Enqueue(QueuedOutput{OutputID: "old", Epoch: 1, Enqueued: time.Now().UTC()})
	}
	for i := 0; i < 30; i++ {
		_ = q.Enqueue(QueuedOutput{OutputID: "new", Epoch: 2, Enqueued: time.Now().UTC()})
	}
	if q.Len() != 80 {
		t.Fatalf("expected 80, got %d", q.Len())
	}

	removed := q.PurgeStale(2)
	if removed != 50 {
		t.Fatalf("expected 50 removed, got %d", removed)
	}
	if q.Len() != 30 {
		t.Fatalf("expected 30 remaining, got %d", q.Len())
	}
}

func TestBoundedOutputQueue_Concurrency(t *testing.T) {
	q := NewBoundedOutputQueue(1000)
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				_ = q.Enqueue(QueuedOutput{
					OutputID: "-" + string(rune('a'+id)) + "-" + string(rune('0'+j%10)),
					Epoch:    uint64(id%3 + 1),
				})
			}
		}(i)
	}
	wg.Wait()
	if q.Len() == 0 {
		t.Fatal("expected non-empty queue after concurrent enqueues")
	}
}
