package mindruntime

import (
	"context"
	"testing"
	"time"
)

func TestNewDeadlinePropagator(t *testing.T) {
	dp := NewDeadlinePropagator(DefaultDeadlineConfig)
	if dp == nil {
		t.Fatal("expected non-nil DeadlinePropagator")
	}
}

func TestNewDeadline(t *testing.T) {
	dp := NewDeadlinePropagator(DefaultDeadlineConfig)

	d := dp.NewDeadline("req-1")
	if d == nil {
		t.Fatal("expected non-nil Deadline")
	}
	if d.Total != DefaultDeadlineConfig.TotalTimeout {
		t.Fatalf("expected total %v, got %v", DefaultDeadlineConfig.TotalTimeout, d.Total)
	}
	if d.Stage != DeadlineStageQueue {
		t.Fatalf("expected stage queue, got %s", d.Stage)
	}
}

func TestPropagateDeadline(t *testing.T) {
	dp := NewDeadlinePropagator(DefaultDeadlineConfig)

	dp.NewDeadline("req-1")
	d := dp.Propagate("req-1", DeadlineStageGeneration)

	if d.Stage != DeadlineStageGeneration {
		t.Fatalf("expected stage generation, got %s", d.Stage)
	}
	if !d.Propagated {
		t.Fatal("expected propagated true")
	}
	if d.Remaining <= 0 {
		t.Fatal("expected positive remaining time")
	}
}

func TestContextWithDeadline(t *testing.T) {
	dp := NewDeadlinePropagator(DefaultDeadlineConfig)

	ctx, cancel := dp.ContextWithDeadline(context.Background(), "req-1", DeadlineStageGeneration)
	defer cancel()

	deadline, ok := ctx.Deadline()
	if !ok {
		t.Fatal("expected deadline set on context")
	}
	if time.Now().UTC().After(deadline) {
		t.Fatal("expected future deadline")
	}
}

func TestExpiredDeadlineContext(t *testing.T) {
	cfg := DeadlineConfig{
		TotalTimeout: 1 * time.Nanosecond,
	}
	dp := NewDeadlinePropagator(cfg)

	dp.NewDeadline("req-1")
	time.Sleep(5 * time.Millisecond)

	ctx, cancel := dp.ContextWithDeadline(context.Background(), "req-1", DeadlineStageQueue)
	defer cancel()

	select {
	case <-ctx.Done():
	default:
		t.Fatal("expected context cancelled immediately")
	}
}

func TestIsExpired(t *testing.T) {
	cfg := DeadlineConfig{
		TotalTimeout: 10 * time.Millisecond,
	}
	dp := NewDeadlinePropagator(cfg)

	dp.NewDeadline("req-1")

	if dp.IsExpired("req-1") {
		t.Fatal("should not be expired initially")
	}

	time.Sleep(15 * time.Millisecond)

	if !dp.IsExpired("req-1") {
		t.Fatal("should be expired after timeout")
	}
}

func TestCancelDeadline(t *testing.T) {
	dp := NewDeadlinePropagator(DefaultDeadlineConfig)

	dp.NewDeadline("req-1")
	dp.Cancel("req-1", "user_requested")

	if dp.IsExpired("req-1") {
		t.Log("cancel marks deadline effectively expired")
	}
}

func TestSuperseded(t *testing.T) {
	dp := NewDeadlinePropagator(DefaultDeadlineConfig)

	dp.NewDeadline("req-1")
	dp.Superseded("req-1", "req-2")

	if !dp.ValidateBeforePersist("req-1") {
		t.Log("SUPERSEDED interaction should not persist")
	}
}

func TestValidateBeforePersist(t *testing.T) {
	dp := NewDeadlinePropagator(DefaultDeadlineConfig)

	dp.NewDeadline("req-1")

	if !dp.ValidateBeforePersist("req-1") {
		t.Fatal("expected valid persist for active deadline")
	}

	dp.Cancel("req-1", "CANCELLED")

	if dp.ValidateBeforePersist("req-1") {
		t.Fatal("expected invalid persist for cancelled deadline")
	}
}

func TestValidateBeforePersistExpiredDuringPersist(t *testing.T) {
	cfg := DeadlineConfig{
		TotalTimeout: 10 * time.Millisecond,
	}
	dp := NewDeadlinePropagator(cfg)

	dp.NewDeadline("req-1")
	dp.Propagate("req-1", DeadlineStagePersist)
	time.Sleep(15 * time.Millisecond)

	if dp.ValidateBeforePersist("req-1") {
		t.Fatal("expected invalid persist when deadline passed during persist stage")
	}
}

func TestRemainingTime(t *testing.T) {
	cfg := DeadlineConfig{
		TotalTimeout: 1 * time.Second,
	}
	dp := NewDeadlinePropagator(cfg)

	dp.NewDeadline("req-1")
	remaining := dp.Remaining("req-1")

	if remaining <= 0 {
		t.Fatal("expected positive remaining time")
	}
	if remaining > 1*time.Second {
		t.Fatalf("expected remaining <= 1s, got %v", remaining)
	}
}

func TestRemoveDeadline(t *testing.T) {
	dp := NewDeadlinePropagator(DefaultDeadlineConfig)

	dp.NewDeadline("req-1")
	dp.Remove("req-1")

	if !dp.IsExpired("req-1") {
		t.Fatal("removed deadline should be expired")
	}
}

func TestMissingRequestID(t *testing.T) {
	dp := NewDeadlinePropagator(DefaultDeadlineConfig)

	if !dp.IsExpired("nonexistent") {
		t.Fatal("unknown request should be expired")
	}
	if dp.Remaining("nonexistent") != 0 {
		t.Fatal("unknown request should have 0 remaining")
	}
}

func TestAllDeadlineStages(t *testing.T) {
	stages := []DeadlineStage{
		DeadlineStageQueue,
		DeadlineStagePersonality,
		DeadlineStageAppraisal,
		DeadlineStageGeneration,
		DeadlineStageDelivery,
		DeadlineStagePersist,
	}

	dp := NewDeadlinePropagator(DefaultDeadlineConfig)
	dp.NewDeadline("req-1")

	for _, stage := range stages {
		d := dp.Propagate("req-1", stage)
		if d.Stage != stage {
			t.Fatalf("expected stage %s, got %s", stage, d.Stage)
		}
	}
}
