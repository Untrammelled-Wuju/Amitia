package queue

import (
	"path/filepath"
	"testing"
	"time"
)

func TestPriorityQueueOrdersP0ToP5(t *testing.T) {
	pq := NewPriorityQueue(PriorityQueueConfig{MaxQueueSize: 10})

	for _, task := range []*Task{
		{ID: "p5", Priority: PriorityP5, Scope: "test"},
		{ID: "p3", Priority: PriorityP3, Scope: "test"},
		{ID: "p0", Priority: PriorityP0, Scope: "test"},
		{ID: "p2", Priority: PriorityP2, Scope: "test"},
		{ID: "p1", Priority: PriorityP1, Scope: "test"},
		{ID: "p4", Priority: PriorityP4, Scope: "test"},
	} {
		ok, reason := pq.Enqueue(task)
		if !ok {
			t.Fatalf("enqueue %s failed: %s", task.ID, reason)
		}
	}

	for _, expected := range []string{"p0", "p1", "p2", "p3", "p4", "p5"} {
		task := pq.Dequeue()
		if task == nil {
			t.Fatalf("expected %s, got nil", expected)
		}
		if task.ID != expected {
			t.Fatalf("expected %s, got %s", expected, task.ID)
		}
		pq.Complete(task)
	}
}

func TestPriorityQueueUsesDefaultConfigsWithPartialOverrides(t *testing.T) {
	pq := NewPriorityQueue(PriorityQueueConfig{
		MaxQueueSize: 10,
		Configs: map[PriorityLevel]PriorityConfig{
			PriorityP0: {MaxConcurrency: 1},
		},
	})

	pq.Enqueue(&Task{ID: "p0-a", Priority: PriorityP0, Scope: "test"})
	pq.Enqueue(&Task{ID: "p0-b", Priority: PriorityP0, Scope: "test"})
	pq.Enqueue(&Task{ID: "p2", Priority: PriorityP2, Scope: "test"})

	first := pq.Dequeue()
	if first == nil || first.ID != "p0-a" {
		t.Fatalf("expected p0-a, got %v", first)
	}

	second := pq.Dequeue()
	if second == nil || second.ID != "p2" {
		t.Fatalf("expected p2 default config to remain active, got %v", second)
	}

	pq.Complete(first)
	pq.Complete(second)
	third := pq.Dequeue()
	if third == nil || third.ID != "p0-b" {
		t.Fatalf("expected p0-b after p0 capacity is released, got %v", third)
	}
}

func TestPriorityQueueRecordsDropAndDepthMetrics(t *testing.T) {
	pq := NewPriorityQueue(PriorityQueueConfig{MaxQueueSize: 1})

	ok, reason := pq.Enqueue(&Task{ID: "expired", Priority: PriorityP0, Scope: "test", Deadline: time.Now().UTC().Add(-time.Second)})
	if ok {
		t.Fatalf("expected expired task to be rejected, got reason %s", reason)
	}

	ok, reason = pq.Enqueue(&Task{ID: "live", Priority: PriorityP0, Scope: "test"})
	if !ok {
		t.Fatalf("expected live task enqueue success, got %s", reason)
	}

	snap := pq.MetricsSnapshot()
	if snap.TotalEnqueued != 1 {
		t.Fatalf("expected 1 enqueued, got %d", snap.TotalEnqueued)
	}
	if snap.TotalDropped != 1 {
		t.Fatalf("expected 1 dropped, got %d", snap.TotalDropped)
	}
	if snap.DropReasons[string(DropReasonExpired)] != 1 {
		t.Fatalf("expected expired drop reason, got %d", snap.DropReasons[string(DropReasonExpired)])
	}
	if snap.QueueDepth != 1 {
		t.Fatalf("expected depth metric 1, got %d", snap.QueueDepth)
	}
}

func TestPriorityQueueCheckpointRestoresPendingTasks(t *testing.T) {
	checkpointPath := filepath.Join(t.TempDir(), "queue.json")
	pq := NewPriorityQueue(PriorityQueueConfig{MaxQueueSize: 10, CheckpointPath: checkpointPath})

	for _, task := range []*Task{
		{ID: "p2", Path: "/p2", Priority: PriorityP2, Scope: "test"},
		{ID: "p0", Path: "/p0", Priority: PriorityP0, Scope: "test"},
	} {
		ok, reason := pq.Enqueue(task)
		if !ok {
			t.Fatalf("enqueue %s failed: %s", task.ID, reason)
		}
	}
	if err := pq.LastPersistenceError(); err != nil {
		t.Fatalf("unexpected persistence error: %v", err)
	}

	restored := NewPriorityQueue(PriorityQueueConfig{MaxQueueSize: 10, CheckpointPath: checkpointPath})
	if restored.Depth() != 2 {
		t.Fatalf("expected restored depth 2, got %d", restored.Depth())
	}

	first := restored.Dequeue()
	if first == nil || first.ID != "p0" || first.Status != TaskRunning {
		t.Fatalf("expected restored p0 running first, got %#v", first)
	}
	restored.Complete(first)

	second := restored.Dequeue()
	if second == nil || second.ID != "p2" {
		t.Fatalf("expected restored p2 second, got %#v", second)
	}
}

func TestPriorityQueueCheckpointRestoresRunningTasksAsPending(t *testing.T) {
	checkpointPath := filepath.Join(t.TempDir(), "queue.json")
	pq := NewPriorityQueue(PriorityQueueConfig{
		MaxQueueSize:   10,
		CheckpointPath: checkpointPath,
		Configs: map[PriorityLevel]PriorityConfig{
			PriorityP0: {MaxConcurrency: 2},
		},
	})

	pq.Enqueue(&Task{ID: "same", Path: "/a", Priority: PriorityP0, Scope: "test"})
	pq.Enqueue(&Task{ID: "same", Path: "/b", Priority: PriorityP0, Scope: "test"})
	first := pq.Dequeue()
	second := pq.Dequeue()
	if first == nil || second == nil {
		t.Fatalf("expected two running tasks, got first=%#v second=%#v", first, second)
	}
	if pq.ActiveCount() != 2 {
		t.Fatalf("expected active count 2, got %d", pq.ActiveCount())
	}

	restored := NewPriorityQueue(PriorityQueueConfig{
		MaxQueueSize:   10,
		CheckpointPath: checkpointPath,
		Configs: map[PriorityLevel]PriorityConfig{
			PriorityP0: {MaxConcurrency: 2},
		},
	})
	if restored.ActiveCount() != 0 {
		t.Fatalf("expected no active tasks after restore, got %d", restored.ActiveCount())
	}
	if restored.Depth() != 2 {
		t.Fatalf("expected running tasks restored as pending, got depth %d", restored.Depth())
	}

	restoredFirst := restored.Dequeue()
	restoredSecond := restored.Dequeue()
	if restoredFirst == nil || restoredSecond == nil {
		t.Fatalf("expected two restored tasks, got first=%#v second=%#v", restoredFirst, restoredSecond)
	}
}

func TestPriorityQueueCheckpointRemovesCompletedTasks(t *testing.T) {
	checkpointPath := filepath.Join(t.TempDir(), "queue.json")
	pq := NewPriorityQueue(PriorityQueueConfig{MaxQueueSize: 10, CheckpointPath: checkpointPath})

	ok, reason := pq.Enqueue(&Task{ID: "done", Path: "/done", Priority: PriorityP0, Scope: "test"})
	if !ok {
		t.Fatalf("enqueue failed: %s", reason)
	}
	task := pq.Dequeue()
	if task == nil {
		t.Fatal("expected dequeued task")
	}
	pq.Complete(task)

	restored := NewPriorityQueue(PriorityQueueConfig{MaxQueueSize: 10, CheckpointPath: checkpointPath})
	if restored.Depth() != 0 {
		t.Fatalf("expected empty queue after completed task checkpoint, got %d", restored.Depth())
	}
}
