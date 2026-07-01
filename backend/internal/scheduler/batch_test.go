package scheduler

import (
	"testing"
	"time"
)

func TestNewBatchProcessor(t *testing.T) {
	flushed := false
	bp := NewBatchProcessor(5, func(entries []OutboxEntry) error {
		flushed = true
		return nil
	})
	if bp == nil {
		t.Fatal("expected non-nil BatchProcessor")
	}
	if bp.Size() != 0 {
		t.Fatalf("expected 0 size, got %d", bp.Size())
	}
	_ = flushed
}

func TestBatchProcessorSubmit(t *testing.T) {
	bp := NewBatchProcessor(5, func(entries []OutboxEntry) error {
		return nil
	})

	err := bp.Submit(BatchRequest{
		Operation: BatchEmbedding,
		Scope:     "test",
		Priority:  PriorityP1,
		Deadline:  time.Now().UTC().Add(1 * time.Hour),
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if bp.Size() != 1 {
		t.Fatalf("expected 1 size, got %d", bp.Size())
	}
}

func TestBatchProcessorFlush(t *testing.T) {
	flushed := false
	bp := NewBatchProcessor(3, func(entries []OutboxEntry) error {
		flushed = true
		return nil
	})

	bp.Submit(BatchRequest{Operation: BatchEmbedding, Scope: "test", Priority: PriorityP1})
	bp.Submit(BatchRequest{Operation: BatchEmbedding, Scope: "test", Priority: PriorityP1})

	err := bp.Flush()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !flushed {
		t.Fatal("expected flush to be called")
	}
	if bp.Size() != 0 {
		t.Fatalf("expected 0 size after flush, got %d", bp.Size())
	}
}

func TestBatchProcessorAutoFlush(t *testing.T) {
	flushed := false
	bp := NewBatchProcessor(2, func(entries []OutboxEntry) error {
		flushed = true
		return nil
	})

	bp.Submit(BatchRequest{Operation: BatchGraphSync, Scope: "test", Priority: PriorityP2})
	if flushed {
		t.Fatal("should not flush before batch size")
	}

	bp.Submit(BatchRequest{Operation: BatchGraphSync, Scope: "test", Priority: PriorityP2})
	if !flushed {
		t.Fatal("should auto flush when batch size reached")
	}
}

func TestBatchProcessorAggregate(t *testing.T) {
	bp := NewBatchProcessor(10, func(entries []OutboxEntry) error {
		return nil
	})

	bp.Submit(BatchRequest{Operation: BatchEmbedding, Scope: "s1", Priority: PriorityP1, BatchKey: "b1"})
	bp.Submit(BatchRequest{Operation: BatchEmbedding, Scope: "s2", Priority: PriorityP1, BatchKey: "b1"})
	bp.Submit(BatchRequest{Operation: BatchGraphSync, Scope: "s1", Priority: PriorityP2, BatchKey: "b2"})

	batches := bp.AggregateByBatchKey()
	if len(batches["b1"]) != 2 {
		t.Fatalf("expected 2 entries for b1, got %d", len(batches["b1"]))
	}
	if len(batches["b2"]) != 1 {
		t.Fatalf("expected 1 entry for b2, got %d", len(batches["b2"]))
	}
}

func TestBatchProcessorMultipleOperations(t *testing.T) {
	bp := NewBatchProcessor(10, func(entries []OutboxEntry) error {
		return nil
	})

	bp.Submit(BatchRequest{Operation: BatchEmbedding, Scope: "test", Priority: PriorityP1})
	bp.Submit(BatchRequest{Operation: BatchGraphSync, Scope: "test", Priority: PriorityP2})
	bp.Submit(BatchRequest{Operation: BatchDeleteClean, Scope: "test", Priority: PriorityP3})
	bp.Submit(BatchRequest{Operation: BatchReflection, Scope: "test", Priority: PriorityP4})
	bp.Submit(BatchRequest{Operation: BatchStats, Scope: "test", Priority: PriorityP5})

	if bp.Size() != 5 {
		t.Fatalf("expected 5 entries, got %d", bp.Size())
	}

	batches := bp.AggregateByBatchKey()
	for op := range batches {
		if len(batches[op]) != 1 {
			t.Fatalf("expected 1 entry per operation batch, got %d for %s", len(batches[op]), op)
		}
	}
}
