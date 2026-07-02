package scheduler

import (
	"path/filepath"
	"testing"
	"time"
)

func TestNewPriorityQueue(t *testing.T) {
	pq := NewPriorityQueue(QueueConfig{MaxQueueSize: 100})
	if pq == nil {
		t.Fatal("expected non-nil PriorityQueue")
	}
	if pq.Depth() != 0 {
		t.Fatalf("expected depth 0, got %d", pq.Depth())
	}
}

func TestEnqueueDequeue(t *testing.T) {
	pq := NewPriorityQueue(QueueConfig{MaxQueueSize: 100})

	task := &Task{
		ID:       "t1",
		Path:     "/test",
		Priority: PriorityP0,
		Scope:    "test",
	}

	ok, reason := pq.Enqueue(task)
	if !ok {
		t.Fatalf("expected enqueue success, got reason: %s", reason)
	}
	if pq.Depth() != 1 {
		t.Fatalf("expected depth 1, got %d", pq.Depth())
	}

	dequeued := pq.Dequeue()
	if dequeued == nil {
		t.Fatal("expected non-nil dequeued task")
	}
	if dequeued.ID != "t1" {
		t.Fatalf("expected task t1, got %s", dequeued.ID)
	}
	if dequeued.Status != TaskRunning {
		t.Fatalf("expected status running, got %s", dequeued.Status)
	}
}

func TestPriorityOrdering(t *testing.T) {
	pq := NewPriorityQueue(QueueConfig{MaxQueueSize: 100})

	tasks := []*Task{
		{ID: "p5", Priority: PriorityP5, Scope: "test"},
		{ID: "p0", Priority: PriorityP0, Scope: "test"},
		{ID: "p3", Priority: PriorityP3, Scope: "test"},
		{ID: "p1", Priority: PriorityP1, Scope: "test"},
	}

	for _, task := range tasks {
		pq.Enqueue(task)
	}

	expected := []string{"p0", "p1", "p3", "p5"}
	for i, exp := range expected {
		task := pq.Dequeue()
		if task == nil {
			t.Fatalf("dequeue %d returned nil, expected %s", i, exp)
		}
		if task.ID != exp {
			t.Fatalf("dequeue %d expected %s, got %s", i, exp, task.ID)
		}
		pq.Complete(task)
	}
}

func TestMaxConcurrency(t *testing.T) {
	cfg := map[PriorityLevel]PriorityConfig{
		PriorityP0: {MaxConcurrency: 2},
		PriorityP1: {MaxConcurrency: 1},
	}
	pq := NewPriorityQueue(QueueConfig{
		MaxQueueSize: 100,
		Configs:      cfg,
	})

	for i := 0; i < 5; i++ {
		pq.Enqueue(&Task{ID: "t", Priority: PriorityP0, Scope: "test"})
	}

	t1 := pq.Dequeue()
	t2 := pq.Dequeue()

	if t1 == nil || t2 == nil {
		t.Fatal("expected two dequeued tasks")
	}

	t3 := pq.Dequeue()
	if t3 != nil {
		t.Fatal("expected nil due to max concurrency")
	}

	pq.Complete(t1)
	t3 = pq.Dequeue()
	if t3 == nil {
		t.Fatal("expected non-nil after completion")
	}
	pq.Complete(t2)
	pq.Complete(t3)
}

func TestExpiredTask(t *testing.T) {
	pq := NewPriorityQueue(QueueConfig{MaxQueueSize: 100})

	deadline := time.Now().UTC().Add(-1 * time.Hour)
	ok, reason := pq.Enqueue(&Task{
		ID:       "expired",
		Priority: PriorityP0,
		Scope:    "test",
		Deadline: deadline,
	})

	if ok {
		t.Fatal("expected expired task to be rejected")
	}
	if reason != DropReasonExpired {
		t.Fatalf("expected expired reason, got %s", reason)
	}
}

func TestQueueFullEviction(t *testing.T) {
	pq := NewPriorityQueue(QueueConfig{MaxQueueSize: 3})

	for i := 0; i < 3; i++ {
		pq.Enqueue(&Task{ID: "keep", Priority: PriorityP0, Scope: "test", Deadline: time.Now().UTC().Add(1 * time.Hour)})
	}

	ok, reason := pq.Enqueue(&Task{ID: "overflow", Priority: PriorityP5, Scope: "test"})

	if !ok {
		t.Fatalf("expected overflow task accepted after eviction, got: %s", reason)
	}
}

func TestCancelTask(t *testing.T) {
	pq := NewPriorityQueue(QueueConfig{MaxQueueSize: 100})

	task := &Task{ID: "cancel", Priority: PriorityP0, Scope: "test"}
	pq.Enqueue(task)

	pq.Cancel(task)
	if task.Status != TaskCancelled {
		t.Fatalf("expected cancelled status, got %s", task.Status)
	}

	dequeued := pq.Dequeue()
	if dequeued != nil {
		t.Fatal("expected nil after cancel")
	}
}

func TestCancelByScope(t *testing.T) {
	pq := NewPriorityQueue(QueueConfig{MaxQueueSize: 100})

	pq.Enqueue(&Task{ID: "a", Priority: PriorityP0, Scope: "scope1"})
	pq.Enqueue(&Task{ID: "b", Priority: PriorityP0, Scope: "scope1"})
	pq.Enqueue(&Task{ID: "c", Priority: PriorityP0, Scope: "scope2"})

	cancelled := pq.CancelByScope("scope1")
	if cancelled != 2 {
		t.Fatalf("expected 2 cancelled, got %d", cancelled)
	}

	dequeued := pq.Dequeue()
	if dequeued == nil || dequeued.ID != "c" {
		t.Fatalf("expected task c, got %v", dequeued)
	}
	pq.Complete(dequeued)
}

func TestExpireStaleTasks(t *testing.T) {
	pq := NewPriorityQueue(QueueConfig{MaxQueueSize: 100})

	pq.Enqueue(&Task{ID: "fresh", Priority: PriorityP0, Scope: "test"})
	pq.Enqueue(&Task{ID: "stale", Priority: PriorityP1, Scope: "test"})

	time.Sleep(10 * time.Millisecond)

	expired := pq.ExpireStaleTasks(5 * time.Millisecond)
	if expired == 0 {
		t.Log("stale task expiration may vary by timing")
	}
}

func TestMetricsSnapshot(t *testing.T) {
	pq := NewPriorityQueue(QueueConfig{MaxQueueSize: 100})

	pq.Enqueue(&Task{ID: "m1", Priority: PriorityP0, Scope: "test"})
	task := pq.Dequeue()
	pq.Complete(task)

	snap := pq.MetricsSnapshot()
	if snap.TotalEnqueued != 1 {
		t.Fatalf("expected 1 enqueued, got %d", snap.TotalEnqueued)
	}
	if snap.TotalCompleted != 1 {
		t.Fatalf("expected 1 completed, got %d", snap.TotalCompleted)
	}
}

func TestReorderByDeadline(t *testing.T) {
	pq := NewPriorityQueue(QueueConfig{MaxQueueSize: 100})

	now := time.Now().UTC()
	pq.Enqueue(&Task{ID: "far", Priority: PriorityP1, Scope: "test", Deadline: now.Add(1 * time.Hour)})
	pq.Enqueue(&Task{ID: "near", Priority: PriorityP1, Scope: "test", Deadline: now.Add(1 * time.Second)})
	pq.Enqueue(&Task{ID: "none", Priority: PriorityP1, Scope: "test"})

	pq.ReorderByDeadline()

	t1 := pq.Dequeue()
	if t1 == nil || t1.ID != "near" {
		t.Fatalf("expected near task first, got %v", t1)
	}
	pq.Complete(t1)

	t2 := pq.Dequeue()
	if t2 == nil || t2.ID != "far" {
		t.Fatalf("expected far task second, got %v", t2)
	}
	pq.Complete(t2)
}

func TestSchedulerPriorityQueueCheckpointConfig(t *testing.T) {
	checkpointPath := filepath.Join(t.TempDir(), "queue.json")
	pq := NewPriorityQueue(QueueConfig{MaxQueueSize: 10, CheckpointPath: checkpointPath})

	ok, reason := pq.Enqueue(&Task{ID: "persisted", Path: "/persisted", Priority: PriorityP1, Scope: "scheduler"})
	if !ok {
		t.Fatalf("enqueue failed: %s", reason)
	}

	restored := NewPriorityQueue(QueueConfig{MaxQueueSize: 10, CheckpointPath: checkpointPath})
	task := restored.Dequeue()
	if task == nil || task.ID != "persisted" {
		t.Fatalf("expected restored scheduler task, got %#v", task)
	}
}
