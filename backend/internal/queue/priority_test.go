package queue

import (
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
