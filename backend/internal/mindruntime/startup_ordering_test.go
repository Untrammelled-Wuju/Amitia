package mindruntime

import (
	"context"
	"errors"
	"testing"
	"time"
)

type testStartupComponent struct {
	phaseName    StartupPhase
	startupErr   error
	healthResult HealthCheckResult
	startupCalled bool
}

func (c *testStartupComponent) PhaseName() StartupPhase {
	return c.phaseName
}

func (c *testStartupComponent) Startup(ctx context.Context) error {
	c.startupCalled = true
	return c.startupErr
}

func (c *testStartupComponent) HealthCheck() HealthCheckResult {
	return c.healthResult
}

func makeHealthyComponent(phase StartupPhase) *testStartupComponent {
	return &testStartupComponent{
		phaseName: phase,
		healthResult: HealthCheckResult{
			Target:  HealthCheckAffect,
			Healthy: true,
			Checks:  []ComponentCheck{{Name: "check", Passed: true}},
		},
	}
}

func makeUnhealthyComponent(phase StartupPhase) *testStartupComponent {
	return &testStartupComponent{
		phaseName: phase,
		healthResult: HealthCheckResult{
			Target:  HealthCheckAffect,
			Healthy: false,
			Checks:  []ComponentCheck{{Name: "check", Passed: false, Message: "failed"}},
			Summary: "health check failed",
		},
	}
}

func TestStartupSequenceExecuteSuccess(t *testing.T) {
	seq := NewStartupSequence()
	seq.Register(makeHealthyComponent(StartupPhaseDatabase))
	seq.Register(makeHealthyComponent(StartupPhaseConfig))
	seq.Register(makeHealthyComponent(StartupPhaseRuntime))

	results := seq.Execute(context.Background())

	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}
	if results[0].Phase != StartupPhaseDatabase {
		t.Fatalf("expected database first, got %s", results[0].Phase)
	}
	if results[len(results)-1].Phase != StartupPhaseRuntime {
		t.Fatalf("expected runtime last, got %s", results[len(results)-1].Phase)
	}
	for _, r := range results {
		if r.Status != StartupStatusComplete {
			t.Fatalf("phase %s should be complete, got %s", r.Phase, r.Status)
		}
	}
}

func TestStartupSequenceFailAndSkip(t *testing.T) {
	seq := NewStartupSequence()
	seq.Register(makeHealthyComponent(StartupPhaseDatabase))
	seq.Register(makeUnhealthyComponent(StartupPhaseConfig))
	seq.Register(makeHealthyComponent(StartupPhaseRuntime))

	results := seq.Execute(context.Background())

	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}
	if results[0].Status != StartupStatusComplete {
		t.Fatalf("database should be complete, got %s", results[0].Status)
	}
	if results[1].Status != StartupStatusFailed {
		t.Fatalf("config should be failed, got %s", results[1].Status)
	}
	if results[2].Status != StartupStatusSkipped {
		t.Fatalf("runtime should be skipped, got %s", results[2].Status)
	}
}

func TestStartupSequenceStartupError(t *testing.T) {
	comp := &testStartupComponent{
		phaseName:  StartupPhaseDatabase,
		startupErr: errors.New("startup failed"),
		healthResult: HealthCheckResult{
			Target:  HealthCheckAffect,
			Healthy: true,
			Checks:  []ComponentCheck{{Name: "check", Passed: true}},
		},
	}

	seq := NewStartupSequence()
	seq.Register(comp)

	results := seq.Execute(context.Background())
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Status != StartupStatusFailed {
		t.Fatalf("expected failed status, got %s", results[0].Status)
	}
	if results[0].Error == "" {
		t.Fatal("expected error message")
	}
}

func TestStartupSequenceAllReady(t *testing.T) {
	seq := NewStartupSequence()
	seq.Register(makeHealthyComponent(StartupPhaseDatabase))
	seq.Register(makeHealthyComponent(StartupPhaseReady))

	results := seq.Execute(context.Background())
	if !seq.AllReady(results) {
		t.Fatal("should be all ready")
	}
}

func TestStartupSequenceNotAllReady(t *testing.T) {
	seq := NewStartupSequence()
	seq.Register(makeHealthyComponent(StartupPhaseDatabase))
	seq.Register(makeHealthyComponent(StartupPhaseRuntime))

	results := seq.Execute(context.Background())
	if seq.AllReady(results) {
		t.Fatal("should not be ready without ready phase")
	}
}

func TestReadyGateSignalAndWait(t *testing.T) {
	deps := []string{"db", "config", "model"}
	gate := NewReadyGate(deps)

	if gate.IsReady() {
		t.Fatal("should not be ready initially")
	}

	gate.SignalReady("db")
	if gate.IsReady() {
		t.Fatal("should not be ready with only one signal")
	}

	gate.SignalReady("config")
	gate.SignalReady("model")

	if !gate.IsReady() {
		t.Fatal("should be ready after all signals")
	}

	ctx := context.Background()
	if err := gate.Wait(ctx); err != nil {
		t.Fatalf("wait should not error, got %v", err)
	}
}

func TestReadyGateWaitWithContext(t *testing.T) {
	gate := NewReadyGate([]string{"db"})
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	if err := gate.Wait(ctx); err == nil {
		t.Fatal("wait should timeout")
	}
}

func TestReadyGateSignalUnknownDependency(t *testing.T) {
	gate := NewReadyGate([]string{"db"})
	gate.SignalReady("unknown")
	if gate.IsReady() {
		t.Fatal("unknown signal should not make gate ready")
	}
}

func TestReadyGateDoubleSignal(t *testing.T) {
	gate := NewReadyGate([]string{"db"})
	gate.SignalReady("db")
	gate.SignalReady("db")

	if !gate.IsReady() {
		t.Fatal("should be ready after first signal")
	}

	if err := gate.Wait(context.Background()); err != nil {
		t.Fatalf("wait error: %v", err)
	}
}

func TestShutdownSequenceExecute(t *testing.T) {
	seq := NewShutdownSequence()
	seq.Register(makeHealthyComponent(StartupPhaseDatabase))
	seq.Register(makeHealthyComponent(StartupPhaseConfig))
	seq.Register(makeHealthyComponent(StartupPhaseRuntime))

	results := seq.Execute(context.Background())

	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}
	if results[0].Phase != StartupPhaseRuntime {
		t.Fatalf("shutdown should reverse, expected runtime first, got %s", results[0].Phase)
	}
	if results[2].Phase != StartupPhaseDatabase {
		t.Fatalf("shutdown should reverse, expected database last, got %s", results[2].Phase)
	}
	for _, r := range results {
		if r.Status != StartupStatusComplete {
			t.Fatalf("phase %s should be complete, got %s", r.Phase, r.Status)
		}
	}
}

func TestShutdownSequenceFailPhase(t *testing.T) {
	comp := makeUnhealthyComponent(StartupPhaseRuntime)
	seq := NewShutdownSequence()
	seq.Register(comp)
	seq.Register(makeHealthyComponent(StartupPhaseDatabase))

	results := seq.Execute(context.Background())
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if results[0].Phase != StartupPhaseRuntime {
		t.Fatal("shutdown should start from highest priority")
	}
	if results[0].Status != StartupStatusFailed {
		t.Fatalf("expected failed, got %s", results[0].Status)
	}
}

func TestStartupPhasePriority(t *testing.T) {
	if p := StartupPhasePriority(StartupPhaseDatabase); p != 1 {
		t.Fatalf("database priority should be 1, got %d", p)
	}
	if p := StartupPhasePriority(StartupPhaseReady); p != 6 {
		t.Fatalf("ready priority should be 6, got %d", p)
	}
	if p := StartupPhasePriority("unknown"); p != 99 {
		t.Fatalf("unknown priority should be 99, got %d", p)
	}
}

func TestDefaultStartupOrder(t *testing.T) {
	if len(DefaultStartupOrder) != 6 {
		t.Fatalf("expected 6 phases, got %d", len(DefaultStartupOrder))
	}
	if DefaultStartupOrder[0] != StartupPhaseDatabase {
		t.Fatal("first phase should be database")
	}
	if DefaultStartupOrder[5] != StartupPhaseReady {
		t.Fatal("last phase should be ready")
	}
}