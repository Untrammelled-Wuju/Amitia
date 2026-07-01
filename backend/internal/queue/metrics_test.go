package queue

import (
	"testing"
	"time"
)

func TestNewQueueMetricsRecord(t *testing.T) {
	qm := NewQueueMetricsRecord()
	if qm == nil {
		t.Fatal("expected non-nil QueueMetricsRecord")
	}
}

func TestRecordEnqueueDequeueComplete(t *testing.T) {
	qm := NewQueueMetricsRecord()

	qm.RecordEnqueue()
	qm.RecordEnqueue()
	qm.RecordDequeue()
	qm.RecordComplete()

	snap := qm.Snapshot(time.Time{})
	if snap.TotalEnqueued != 2 {
		t.Fatalf("expected 2 enqueued, got %d", snap.TotalEnqueued)
	}
	if snap.TotalDequeued != 1 {
		t.Fatalf("expected 1 dequeued, got %d", snap.TotalDequeued)
	}
	if snap.TotalCompleted != 1 {
		t.Fatalf("expected 1 completed, got %d", snap.TotalCompleted)
	}
}

func TestRecordDrop(t *testing.T) {
	qm := NewQueueMetricsRecord()

	qm.RecordDrop("queue_full")
	qm.RecordDrop("queue_full")
	qm.RecordDrop("expired")

	snap := qm.Snapshot(time.Time{})
	if snap.TotalDropped != 3 {
		t.Fatalf("expected 3 dropped, got %d", snap.TotalDropped)
	}
	if snap.DropReasons["queue_full"] != 2 {
		t.Fatalf("expected 2 queue_full drops, got %d", snap.DropReasons["queue_full"])
	}
	if snap.DropReasons["expired"] != 1 {
		t.Fatalf("expected 1 expired drop, got %d", snap.DropReasons["expired"])
	}
}

func TestRecordMerge(t *testing.T) {
	qm := NewQueueMetricsRecord()

	qm.RecordMerge("batch_embedding")
	qm.RecordMerge("batch_embedding")
	qm.RecordMerge("batch_graph")

	snap := qm.Snapshot(time.Time{})
	if snap.MergeReasons["batch_embedding"] != 2 {
		t.Fatalf("expected 2 batch_embedding merges, got %d", snap.MergeReasons["batch_embedding"])
	}
	if snap.MergeReasons["batch_graph"] != 1 {
		t.Fatalf("expected 1 batch_graph merge, got %d", snap.MergeReasons["batch_graph"])
	}
}

func TestRecordCacheInvalidation(t *testing.T) {
	qm := NewQueueMetricsRecord()

	qm.RecordCacheInvalidation()
	qm.RecordCacheInvalidation()

	snap := qm.Snapshot(time.Time{})
	if snap.CacheInvalidations != 2 {
		t.Fatalf("expected 2 cache invalidations, got %d", snap.CacheInvalidations)
	}
}

func TestRecordQueueDepth(t *testing.T) {
	qm := NewQueueMetricsRecord()

	qm.RecordQueueDepth(10)
	qm.RecordQueueDepth(50)
	qm.RecordQueueDepth(25)

	snap := qm.Snapshot(time.Time{})
	if snap.QueueDepth != 25 {
		t.Fatalf("expected current depth 25, got %d", snap.QueueDepth)
	}
	if snap.MaxQueueDepth != 50 {
		t.Fatalf("expected max depth 50, got %d", snap.MaxQueueDepth)
	}
}

func TestRecordTaskAge(t *testing.T) {
	qm := NewQueueMetricsRecord()

	qm.RecordTaskAge(100 * time.Millisecond)
	qm.RecordTaskAge(300 * time.Millisecond)

	snap := qm.Snapshot(time.Time{})
	if snap.MaxTaskAgeMs < 300 {
		t.Fatalf("expected max age >= 300ms, got %f", snap.MaxTaskAgeMs)
	}
	if snap.AvgTaskAgeMs <= 0 {
		t.Fatalf("expected positive avg age, got %f", snap.AvgTaskAgeMs)
	}
}

func TestRecordCancel(t *testing.T) {
	qm := NewQueueMetricsRecord()

	qm.RecordCancel()
	qm.RecordCancel()
	qm.RecordCancel()

	snap := qm.Snapshot(time.Time{})
	if snap.TotalCancelled != 3 {
		t.Fatalf("expected 3 cancelled, got %d", snap.TotalCancelled)
	}
}

func TestSnapshotThroughput(t *testing.T) {
	qm := NewQueueMetricsRecord()

	qm.RecordEnqueue()
	qm.RecordDequeue()
	qm.RecordComplete()

	lastCollected := time.Now().UTC().Add(-1 * time.Second)

	snap := qm.Snapshot(lastCollected)
	if snap.ThroughputPerSec <= 0 {
		t.Fatalf("expected positive throughput, got %f", snap.ThroughputPerSec)
	}
}

func TestEmptySnapshot(t *testing.T) {
	qm := NewQueueMetricsRecord()

	snap := qm.Snapshot(time.Time{})
	if snap.TotalEnqueued != 0 {
		t.Fatalf("expected 0 enqueued, got %d", snap.TotalEnqueued)
	}
	if snap.AvgTaskAgeMs != 0 {
		t.Fatalf("expected 0 avg age, got %f", snap.AvgTaskAgeMs)
	}
}
