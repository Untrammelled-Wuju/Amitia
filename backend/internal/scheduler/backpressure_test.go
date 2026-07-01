package scheduler

import (
	"testing"
)

func TestNewBackpressureController(t *testing.T) {
	bc := NewBackpressureController(DefaultBackpressureConfig)
	if bc.State() != BackpressureNormal {
		t.Fatalf("expected normal state, got %s", bc.State())
	}
}

func TestBackpressureNormalToWarning(t *testing.T) {
	cfg := DefaultBackpressureConfig
	cfg.CooldownDuration = 0
	bc := NewBackpressureController(cfg)

	for i := 0; i < cfg.WindowSize; i++ {
		bc.RecordLoad(0.7)
	}

	if bc.State() != BackpressureWarning {
		t.Fatalf("expected warning state, got %s", bc.State())
	}
}

func TestBackpressureCritical(t *testing.T) {
	cfg := DefaultBackpressureConfig
	cfg.CooldownDuration = 0
	bc := NewBackpressureController(cfg)

	for i := 0; i < cfg.WindowSize; i++ {
		bc.RecordLoad(0.85)
	}

	if bc.State() != BackpressureCritical {
		t.Fatalf("expected critical state, got %s", bc.State())
	}
}

func TestBackpressureShedding(t *testing.T) {
	cfg := DefaultBackpressureConfig
	cfg.CooldownDuration = 0
	bc := NewBackpressureController(cfg)

	for i := 0; i < cfg.WindowSize; i++ {
		bc.RecordLoad(0.96)
	}

	if bc.State() != BackpressureShedding {
		t.Fatalf("expected shedding state, got %s", bc.State())
	}
}

func TestShouldDefer(t *testing.T) {
	cfg := DefaultBackpressureConfig
	cfg.CooldownDuration = 0
	bc := NewBackpressureController(cfg)

	if bc.ShouldDefer(DeferredEmbedding) {
		t.Fatal("should not defer in normal state")
	}

	for i := 0; i < cfg.WindowSize; i++ {
		bc.RecordLoad(0.9)
	}

	if !bc.ShouldDefer(DeferredEmbedding) {
		t.Fatal("should defer embedding in critical state")
	}
	if !bc.ShouldDefer(DeferredGraph) {
		t.Fatal("should defer graph in critical state")
	}
	if !bc.ShouldDefer(DeferredReflection) {
		t.Fatal("should defer reflection in critical state")
	}
}

func TestShouldCancelProactive(t *testing.T) {
	cfg := DefaultBackpressureConfig
	cfg.CooldownDuration = 0
	bc := NewBackpressureController(cfg)

	if bc.ShouldCancelProactive() {
		t.Fatal("should not cancel proactive in normal state")
	}

	for i := 0; i < cfg.WindowSize; i++ {
		bc.RecordLoad(0.96)
	}

	if !bc.ShouldCancelProactive() {
		t.Fatal("should cancel proactive in shedding state")
	}
}

func TestOutboxAggregation(t *testing.T) {
	flushed := false
	var flushedEntries []OutboxEntry

	o := NewOutbox(3, func(entries []OutboxEntry) error {
		flushed = true
		flushedEntries = entries
		return nil
	})

	o.Add(OutboxEntry{ID: "a", BatchKey: "key1"})
	o.Add(OutboxEntry{ID: "b", BatchKey: "key1"})

	if flushed {
		t.Fatal("should not flush before batch size")
	}

	o.Add(OutboxEntry{ID: "c", BatchKey: "key2"})

	if !flushed {
		t.Fatal("should flush when batch size reached")
	}
	if len(flushedEntries) != 3 {
		t.Fatalf("expected 3 flushed entries, got %d", len(flushedEntries))
	}
}

func TestOutboxFlushEmpty(t *testing.T) {
	o := NewOutbox(10, func(entries []OutboxEntry) error {
		return nil
	})

	err := o.Flush()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestOutboxAggregateByBatchKey(t *testing.T) {
	o := NewOutbox(10, func(entries []OutboxEntry) error {
		return nil
	})

	o.Add(OutboxEntry{ID: "a", BatchKey: "key1"})
	o.Add(OutboxEntry{ID: "b", BatchKey: "key1"})
	o.Add(OutboxEntry{ID: "c", BatchKey: "key2"})

	batches := o.AggregateByBatchKey()
	if len(batches["key1"]) != 2 {
		t.Fatalf("expected 2 entries for key1, got %d", len(batches["key1"]))
	}
	if len(batches["key2"]) != 1 {
		t.Fatalf("expected 1 entry for key2, got %d", len(batches["key2"]))
	}
}

func TestResolveDeferredStrategy(t *testing.T) {
	cfg := DefaultBackpressureConfig
	cfg.CooldownDuration = 0
	bc := NewBackpressureController(cfg)

	if bc.ResolveDeferredStrategy() != "" {
		t.Fatal("expected empty strategy in normal state")
	}

	for i := 0; i < cfg.WindowSize; i++ {
		bc.RecordLoad(0.7)
	}

	if bc.ResolveDeferredStrategy() != DeferredGraph {
		t.Fatalf("expected graph defer in warning, got %s", bc.ResolveDeferredStrategy())
	}
}

func TestBackpressureCooldown(t *testing.T) {
	cfg := BackpressureConfig{
		WarningThreshold:  0.6,
		CriticalThreshold: 0.8,
		SheddingThreshold: 0.95,
		WindowSize:        20,
		CooldownDuration:  0,
	}
	bc := NewBackpressureController(cfg)

	for i := 0; i < cfg.WindowSize; i++ {
		bc.RecordLoad(0.9)
	}

	if bc.State() != BackpressureCritical {
		t.Fatalf("expected critical state on first hit, got %s", bc.State())
	}

	for i := 0; i < cfg.WindowSize; i++ {
		bc.RecordLoad(0.1)
	}

	if bc.State() != BackpressureNormal {
		t.Fatalf("expected normal state after load drops with no cooldown, got %s", bc.State())
	}
}



