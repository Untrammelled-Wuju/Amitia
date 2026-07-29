package update

import (
	"context"
	"testing"
)

func activateGeneration(t *testing.T, mgr *GenerationManager, ctx context.Context, extID, version, hash string) Generation {
	t.Helper()
	gen := mgr.Prepare(ctx, extID, version, hash)
	if err := mgr.Transition(ctx, extID, gen.GenerationID, GenerationStateValidated); err != nil {
		t.Fatalf("transition to validated: %v", err)
	}
	if err := mgr.Transition(ctx, extID, gen.GenerationID, GenerationStateRuntimeReady); err != nil {
		t.Fatalf("transition to runtime ready: %v", err)
	}
	if err := mgr.Transition(ctx, extID, gen.GenerationID, GenerationStateActive); err != nil {
		t.Fatalf("transition to active: %v", err)
	}
	return gen
}

func TestDrainGenerationActive(t *testing.T) {
	mgr := NewGenerationManager()
	ctx := context.Background()
	gen := activateGeneration(t, mgr, ctx, "ext-1", "1.0.0", "hash-1")

	if err := mgr.DrainGeneration(ctx, "ext-1", gen.GenerationID); err != nil {
		t.Fatalf("DrainGeneration: %v", err)
	}

	got, err := mgr.Get(ctx, "ext-1", gen.GenerationID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.State != GenerationStateDraining {
		t.Fatalf("expected draining, got %s", got.State)
	}
}

func TestDrainGenerationIdempotent(t *testing.T) {
	mgr := NewGenerationManager()
	ctx := context.Background()
	gen := activateGeneration(t, mgr, ctx, "ext-1", "1.0.0", "hash-1")

	_ = mgr.DrainGeneration(ctx, "ext-1", gen.GenerationID)
	if err := mgr.DrainGeneration(ctx, "ext-1", gen.GenerationID); err != nil {
		t.Fatalf("second DrainGeneration should be idempotent: %v", err)
	}
}

func TestDrainGenerationAlreadyStopped(t *testing.T) {
	mgr := NewGenerationManager()
	ctx := context.Background()
	gen := activateGeneration(t, mgr, ctx, "ext-1", "1.0.0", "hash-1")
	_ = mgr.DrainGeneration(ctx, "ext-1", gen.GenerationID)
	_ = mgr.StopGeneration(ctx, "ext-1", gen.GenerationID)

	if err := mgr.DrainGeneration(ctx, "ext-1", gen.GenerationID); err != nil {
		t.Fatalf("DrainGeneration on stopped should be idempotent: %v", err)
	}
}

func TestStopGenerationFromDraining(t *testing.T) {
	mgr := NewGenerationManager()
	ctx := context.Background()
	gen := activateGeneration(t, mgr, ctx, "ext-1", "1.0.0", "hash-1")
	_ = mgr.DrainGeneration(ctx, "ext-1", gen.GenerationID)

	if err := mgr.StopGeneration(ctx, "ext-1", gen.GenerationID); err != nil {
		t.Fatalf("StopGeneration: %v", err)
	}

	got, err := mgr.Get(ctx, "ext-1", gen.GenerationID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.State != GenerationStateStopped {
		t.Fatalf("expected stopped, got %s", got.State)
	}
	if got.StoppedAt == nil {
		t.Fatal("expected StoppedAt to be set")
	}
}

func TestStopGenerationIdempotent(t *testing.T) {
	mgr := NewGenerationManager()
	ctx := context.Background()
	gen := activateGeneration(t, mgr, ctx, "ext-1", "1.0.0", "hash-1")
	_ = mgr.DrainGeneration(ctx, "ext-1", gen.GenerationID)
	_ = mgr.StopGeneration(ctx, "ext-1", gen.GenerationID)

	if err := mgr.StopGeneration(ctx, "ext-1", gen.GenerationID); err != nil {
		t.Fatalf("second StopGeneration should be idempotent: %v", err)
	}
}

func TestDrainGenerationNotFound(t *testing.T) {
	mgr := NewGenerationManager()
	ctx := context.Background()
	if err := mgr.DrainGeneration(ctx, "ext-1", "nonexistent"); err == nil {
		t.Fatal("expected error for nonexistent generation")
	}
}

func TestStopGenerationNotFound(t *testing.T) {
	mgr := NewGenerationManager()
	ctx := context.Background()
	if err := mgr.StopGeneration(ctx, "ext-1", "nonexistent"); err == nil {
		t.Fatal("expected error for nonexistent generation")
	}
}

func TestDrainingGenerationsList(t *testing.T) {
	mgr := NewGenerationManager()
	ctx := context.Background()

	gen1 := activateGeneration(t, mgr, ctx, "ext-1", "1.0.0", "hash-1")
	gen2 := activateGeneration(t, mgr, ctx, "ext-1", "1.1.0", "hash-2")

	draining := mgr.DrainingGenerations(ctx, "ext-1")
	if len(draining) != 1 {
		t.Fatalf("expected 1 draining generation, got %d", len(draining))
	}
	if draining[0].GenerationID != gen1.GenerationID {
		t.Fatalf("expected gen1 to be draining, got %s", draining[0].GenerationID)
	}
	if mgr.Active(ctx, "ext-1").GenerationID != gen2.GenerationID {
		t.Fatal("expected gen2 to be active")
	}
}

func TestPromoteDrainsOldGeneration(t *testing.T) {
	mgr := NewGenerationManager()
	ctx := context.Background()

	gen1 := activateGeneration(t, mgr, ctx, "ext-1", "1.0.0", "hash-1")

	gen2 := mgr.Prepare(ctx, "ext-1", "1.1.0", "hash-2")
	mgr.Transition(ctx, "ext-1", gen2.GenerationID, GenerationStateValidated)
	mgr.Transition(ctx, "ext-1", gen2.GenerationID, GenerationStateRuntimeReady)
	mgr.Transition(ctx, "ext-1", gen2.GenerationID, GenerationStateActive)

	got1, _ := mgr.Get(ctx, "ext-1", gen1.GenerationID)
	if got1.State != GenerationStateDraining {
		t.Fatalf("expected gen1 draining after promote, got %s", got1.State)
	}

	draining := mgr.DrainingGenerations(ctx, "ext-1")
	if len(draining) != 1 {
		t.Fatalf("expected 1 draining, got %d", len(draining))
	}
}

func TestDrainGenerationDoesNotAffectNewActive(t *testing.T) {
	mgr := NewGenerationManager()
	ctx := context.Background()

	gen1 := activateGeneration(t, mgr, ctx, "ext-1", "1.0.0", "hash-1")
	gen2 := activateGeneration(t, mgr, ctx, "ext-1", "1.1.0", "hash-2")

	if err := mgr.DrainGeneration(ctx, "ext-1", gen1.GenerationID); err != nil {
		t.Fatalf("DrainGeneration gen1: %v", err)
	}

	active := mgr.Active(ctx, "ext-1")
	if active == nil || active.GenerationID != gen2.GenerationID {
		t.Fatal("expected gen2 to remain active")
	}
}

func TestStopGenerationClearsActive(t *testing.T) {
	mgr := NewGenerationManager()
	ctx := context.Background()
	gen := activateGeneration(t, mgr, ctx, "ext-1", "1.0.0", "hash-1")

	if mgr.Active(ctx, "ext-1") == nil {
		t.Fatal("expected active generation before drain")
	}

	_ = mgr.DrainGeneration(ctx, "ext-1", gen.GenerationID)
	_ = mgr.StopGeneration(ctx, "ext-1", gen.GenerationID)

	if mgr.Active(ctx, "ext-1") != nil {
		t.Fatal("expected no active generation after stop")
	}
}

func TestActiveToFailedAllowed(t *testing.T) {
	mgr := NewGenerationManager()
	ctx := context.Background()
	gen := activateGeneration(t, mgr, ctx, "ext-1", "1.0.0", "hash-1")

	if err := mgr.Transition(ctx, "ext-1", gen.GenerationID, GenerationStateFailed); err != nil {
		t.Fatalf("transition active -> failed should be allowed: %v", err)
	}

	got, _ := mgr.Get(ctx, "ext-1", gen.GenerationID)
	if got.State != GenerationStateFailed {
		t.Fatalf("expected failed, got %s", got.State)
	}
}

func TestDrainingToFailedAllowed(t *testing.T) {
	mgr := NewGenerationManager()
	ctx := context.Background()
	gen := activateGeneration(t, mgr, ctx, "ext-1", "1.0.0", "hash-1")
	_ = mgr.DrainGeneration(ctx, "ext-1", gen.GenerationID)

	if err := mgr.Transition(ctx, "ext-1", gen.GenerationID, GenerationStateFailed); err != nil {
		t.Fatalf("transition draining -> failed should be allowed: %v", err)
	}

	got, _ := mgr.Get(ctx, "ext-1", gen.GenerationID)
	if got.State != GenerationStateFailed {
		t.Fatalf("expected failed, got %s", got.State)
	}
}
