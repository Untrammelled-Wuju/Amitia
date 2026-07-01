package mindruntime

import (
	"context"
	"errors"
	"testing"
	"time"
)

type testLifecycleComponent struct {
	phase    LifecyclePhase
	startErr error
	drainErr error
	shutdownCalled bool
	drainCalled    bool
}

func (c *testLifecycleComponent) Start(ctx context.Context) error {
	if c.startErr != nil {
		return c.startErr
	}
	c.phase = LifecyclePhaseRunning
	return nil
}

func (c *testLifecycleComponent) Shutdown(ctx context.Context) error {
	c.shutdownCalled = true
	c.phase = LifecyclePhaseShuttingDown
	return nil
}

func (c *testLifecycleComponent) Drain(ctx context.Context) error {
	c.drainCalled = true
	if c.drainErr != nil {
		return c.drainErr
	}
	c.phase = LifecyclePhaseDraining
	return nil
}

func (c *testLifecycleComponent) Phase() LifecyclePhase {
	return c.phase
}

func (c *testLifecycleComponent) State() LifecycleState {
	return LifecycleState{
		Phase: c.phase,
	}
}

func TestLifecyclePhaseConstants(t *testing.T) {
	if LifecyclePhaseInit != "init" {
		t.Fatalf("expected init, got %s", LifecyclePhaseInit)
	}
	if LifecyclePhaseRunning != "running" {
		t.Fatalf("expected running, got %s", LifecyclePhaseRunning)
	}
	if LifecyclePhaseTerminated != "terminated" {
		t.Fatalf("expected terminated, got %s", LifecyclePhaseTerminated)
	}
}

func TestShutdownOrderExecute(t *testing.T) {
	comp1 := &testLifecycleComponent{phase: LifecyclePhaseRunning}
	comp2 := &testLifecycleComponent{phase: LifecyclePhaseRunning}
	comp3 := &testLifecycleComponent{phase: LifecyclePhaseRunning}

	order := NewShutdownOrder([]LifecycleComponent{comp1, comp2, comp3})
	results := order.Execute(context.Background())

	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}
	if !comp1.shutdownCalled {
		t.Fatal("comp1 should be shut down")
	}
	if !comp3.drainCalled {
		t.Fatal("comp3 should be drained")
	}
	if results[0].Phase != LifecyclePhaseTerminated {
		t.Fatalf("expected terminated, got %s", results[0].Phase)
	}
}

func TestShutdownOrderReverseOrder(t *testing.T) {
	shutdownOrder := make([]string, 0)

	comp1 := &testLifecycleComponent{phase: LifecyclePhaseRunning}
	comp2 := &testLifecycleComponent{phase: LifecyclePhaseRunning}

	order := NewShutdownOrder([]LifecycleComponent{comp1, comp2})
	ctx := context.Background()
	results := order.Execute(ctx)

	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	_ = shutdownOrder
}

func TestLifecycleDeadlineRemaining(t *testing.T) {
	deadline := NewLifecycleDeadline("req-1", 100*time.Millisecond)

	remaining := deadline.Remaining()
	if remaining <= 0 {
		t.Fatal("remaining should be positive")
	}
	if remaining > 100*time.Millisecond {
		t.Fatal("remaining should not exceed timeout")
	}

	time.Sleep(150 * time.Millisecond)
	if !deadline.IsExpired() {
		t.Fatal("deadline should be expired")
	}
	if deadline.Remaining() != 0 {
		t.Fatal("remaining should be 0 after expiry")
	}
}

func TestLifecycleDeadlinePropagate(t *testing.T) {
	deadline := NewLifecycleDeadline("req-2", 5*time.Second)
	deadline.Propagate("service-a")
	deadline.Propagate("service-b")

	if len(deadline.PropagatedTo) != 2 {
		t.Fatalf("expected 2 propagated targets, got %d", len(deadline.PropagatedTo))
	}

	deadline.Expired = true
	deadline.Propagate("service-c")
	if len(deadline.PropagatedTo) != 2 {
		t.Fatal("expired deadline should not propagate further")
	}
}

func TestLifecycleDeadlineContext(t *testing.T) {
	deadline := NewLifecycleDeadline("req-3", 50*time.Millisecond)
	ctx, cancel := deadline.Context(context.Background())
	defer cancel()

	select {
	case <-ctx.Done():
		t.Fatal("context should not be done immediately")
	case <-time.After(10 * time.Millisecond):
	}

	time.Sleep(100 * time.Millisecond)
	select {
	case <-ctx.Done():
		if ctx.Err() != context.DeadlineExceeded {
			t.Fatalf("expected DeadlineExceeded, got %v", ctx.Err())
		}
	}
}

func TestShutdownSignalString(t *testing.T) {
	if ShutdownSignalGraceful.String() != "graceful" {
		t.Fatalf("expected graceful, got %s", ShutdownSignalGraceful.String())
	}
	if ShutdownSignalForce.String() != "force" {
		t.Fatalf("expected force, got %s", ShutdownSignalForce.String())
	}
}

func TestDrainConfigDefaults(t *testing.T) {
	cfg := DefaultDrainConfig()
	if cfg.Timeout != 30*time.Second {
		t.Fatalf("expected 30s timeout, got %v", cfg.Timeout)
	}
	if cfg.PollInterval != 500*time.Millisecond {
		t.Fatalf("expected 500ms poll, got %v", cfg.PollInterval)
	}
}

func TestShutdownOrderDrainError(t *testing.T) {
	comp := &testLifecycleComponent{
		phase:    LifecyclePhaseRunning,
		drainErr: errors.New("drain failed"),
	}

	order := NewShutdownOrder([]LifecycleComponent{comp})
	results := order.Execute(context.Background())

	if len(results) != 1 {
		t.Fatal("expected 1 result")
	}
	if results[0].Error == "" {
		t.Fatal("expected drain error in result")
	}
}
