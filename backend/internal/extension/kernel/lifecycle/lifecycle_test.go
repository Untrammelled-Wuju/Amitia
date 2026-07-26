package lifecycle

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

type fakeComponent struct {
	id          string
	deps        []string
	startErr    error
	readyErr    error
	stopErr     error
	state       ComponentState
	startCount  int32
	readyCount  int32
	stopCount   int32
	healthErr   error
}

func (f *fakeComponent) ID() string { return f.id }
func (f *fakeComponent) Dependencies() []string { return f.deps }
func (f *fakeComponent) Start(ctx context.Context) error {
	atomic.AddInt32(&f.startCount, 1)
	if f.startErr != nil {
		return f.startErr
	}
	f.state = ComponentStateStarted
	return nil
}
func (f *fakeComponent) Ready(ctx context.Context) error {
	atomic.AddInt32(&f.readyCount, 1)
	if f.readyErr != nil {
		return f.readyErr
	}
	f.state = ComponentStateReady
	return nil
}
func (f *fakeComponent) Stop(ctx context.Context, reason ShutdownReason) error {
	atomic.AddInt32(&f.stopCount, 1)
	f.state = ComponentStateStopped
	return f.stopErr
}
func (f *fakeComponent) Health(ctx context.Context) ComponentHealth {
	return ComponentHealth{
		ComponentID: f.id,
		State:       f.state,
		Healthy:     f.state == ComponentStateReady,
		CheckedAt:   now(),
	}
}

func TestCoordinatorBasicStartup(t *testing.T) {
	registry := NewComponentRegistry()
	c1 := &fakeComponent{id: "c1"}
	c2 := &fakeComponent{id: "c2", deps: []string{"c1"}}
	if err := registry.Register(c1, BootstrapComponent{ID: "c1", Phase: StartupPhaseCore, Required: true}); err != nil {
		t.Fatalf("register c1: %v", err)
	}
	if err := registry.Register(c2, BootstrapComponent{ID: "c2", Phase: StartupPhaseStorage, Required: true, Dependencies: []string{"c1"}}); err != nil {
		t.Fatalf("register c2: %v", err)
	}
	journal := NewJournal()
	store := NewInMemoryJournalStore()
	audit := NewInMemoryAuditWriter()
	reconciler := NewDefaultReconciler()
	readiness := NewReadinessService(registry, audit)
	readiness.MarkCore("c1")
	readiness.MarkCore("c2")
	drain := NewDrainController()
	shutdown := NewShutdownCoordinator(registry, journal, drain, audit, store)
	recovery := NewRecoveryScanner(journal)
	coord := NewCoordinator(registry, journal, store, reconciler, readiness, shutdown, drain, audit, recovery)

	ctx := context.Background()
	if err := coord.Startup(ctx, 30*time.Second); err != nil {
		t.Fatalf("startup: %v", err)
	}
	if !coord.IsReady() {
		t.Fatalf("expected ready")
	}
	if atomic.LoadInt32(&c1.startCount) != 1 {
		t.Errorf("c1 start count: %d", c1.startCount)
	}
	if atomic.LoadInt32(&c2.startCount) != 1 {
		t.Errorf("c2 start count: %d", c2.startCount)
	}
}

func TestCoordinatorCircularDependency(t *testing.T) {
	registry := NewComponentRegistry()
	c1 := &fakeComponent{id: "c1", deps: []string{"c2"}}
	c2 := &fakeComponent{id: "c2", deps: []string{"c1"}}
	_ = registry.Register(c1, BootstrapComponent{ID: "c1", Phase: StartupPhaseCore, Dependencies: []string{"c2"}})
	_ = registry.Register(c2, BootstrapComponent{ID: "c2", Phase: StartupPhaseCore, Dependencies: []string{"c1"}})
	planner := NewPlanner(registry)
	_, err := planner.BuildPlan("test")
	if !errors.Is(err, ErrCircularDependency) {
		t.Fatalf("expected circular dependency, got %v", err)
	}
}

func TestCoordinatorMissingDependency(t *testing.T) {
	registry := NewComponentRegistry()
	c1 := &fakeComponent{id: "c1", deps: []string{"missing"}}
	_ = registry.Register(c1, BootstrapComponent{ID: "c1", Phase: StartupPhaseCore, Dependencies: []string{"missing"}})
	planner := NewPlanner(registry)
	_, err := planner.BuildPlan("test")
	if !errors.Is(err, ErrMissingDependency) {
		t.Fatalf("expected missing dependency, got %v", err)
	}
}

func TestCoordinatorIllegalCrossPhase(t *testing.T) {
	registry := NewComponentRegistry()
	c1 := &fakeComponent{id: "c1"}
	c2 := &fakeComponent{id: "c2", deps: []string{"c1"}}
	_ = registry.Register(c1, BootstrapComponent{ID: "c1", Phase: StartupPhaseRegistries})
	_ = registry.Register(c2, BootstrapComponent{ID: "c2", Phase: StartupPhaseCore, Dependencies: []string{"c1"}})
	planner := NewPlanner(registry)
	_, err := planner.BuildPlan("test")
	if !errors.Is(err, ErrIllegalCrossPhase) {
		t.Fatalf("expected illegal cross phase, got %v", err)
	}
}

func TestCoordinatorShutdown(t *testing.T) {
	registry := NewComponentRegistry()
	c1 := &fakeComponent{id: "c1"}
	_ = registry.Register(c1, BootstrapComponent{ID: "c1", Phase: StartupPhaseCore, Required: true})
	journal := NewJournal()
	store := NewInMemoryJournalStore()
	audit := NewInMemoryAuditWriter()
	reconciler := NewDefaultReconciler()
	readiness := NewReadinessService(registry, audit)
	drain := NewDrainController()
	shutdown := NewShutdownCoordinator(registry, journal, drain, audit, store)
	recovery := NewRecoveryScanner(journal)
	coord := NewCoordinator(registry, journal, store, reconciler, readiness, shutdown, drain, audit, recovery)

	ctx := context.Background()
	if err := coord.Startup(ctx, 30*time.Second); err != nil {
		t.Fatalf("startup: %v", err)
	}
	if err := coord.Shutdown(ctx, ShutdownReasonNormal, 30*time.Second); err != nil {
		t.Fatalf("shutdown: %v", err)
	}
	if atomic.LoadInt32(&c1.stopCount) != 1 {
		t.Errorf("c1 stop count: %d", c1.stopCount)
	}
	if !journal.IsCleanShutdown() {
		t.Errorf("expected clean shutdown")
	}
}

func TestCoordinatorFailedComponent(t *testing.T) {
	registry := NewComponentRegistry()
	c1 := &fakeComponent{id: "c1", startErr: errors.New("boom")}
	_ = registry.Register(c1, BootstrapComponent{ID: "c1", Phase: StartupPhaseCore, Required: true})
	journal := NewJournal()
	store := NewInMemoryJournalStore()
	audit := NewInMemoryAuditWriter()
	reconciler := NewDefaultReconciler()
	readiness := NewReadinessService(registry, audit)
	drain := NewDrainController()
	shutdown := NewShutdownCoordinator(registry, journal, drain, audit, store)
	recovery := NewRecoveryScanner(journal)
	coord := NewCoordinator(registry, journal, store, reconciler, readiness, shutdown, drain, audit, recovery)

	ctx := context.Background()
	err := coord.Startup(ctx, 30*time.Second)
	if err == nil {
		t.Fatalf("expected error from required failed component")
	}
}

func TestCoordinatorOptionalFailure(t *testing.T) {
	registry := NewComponentRegistry()
	c1 := &fakeComponent{id: "c1", startErr: errors.New("boom")}
	_ = registry.Register(c1, BootstrapComponent{
		ID: "c1", Phase: StartupPhaseCore, Required: false,
		FailureMode: FailureModeDegrade,
	})
	journal := NewJournal()
	store := NewInMemoryJournalStore()
	audit := NewInMemoryAuditWriter()
	reconciler := NewDefaultReconciler()
	readiness := NewReadinessService(registry, audit)
	drain := NewDrainController()
	shutdown := NewShutdownCoordinator(registry, journal, drain, audit, store)
	recovery := NewRecoveryScanner(journal)
	coord := NewCoordinator(registry, journal, store, reconciler, readiness, shutdown, drain, audit, recovery)

	ctx := context.Background()
	if err := coord.Startup(ctx, 30*time.Second); err != nil {
		t.Fatalf("optional failure should not abort startup: %v", err)
	}
}

func TestDrainControllerWait(t *testing.T) {
	d := NewDrainController()
	d.Begin(ShutdownReasonNormal)
	if !d.RejectNew() {
		t.Fatalf("expected reject new")
	}
	op := ActiveOperation{ID: "op1", ComponentID: "c1", Kind: "tool"}
	d.Register(op, func() {})
	result := d.Wait(context.Background(), 100*time.Millisecond)
	if !result.TimedOut {
		t.Fatalf("expected timeout with active op")
	}
	d.Complete("op1")
	result = d.Wait(context.Background(), 100*time.Millisecond)
	if result.TimedOut {
		t.Fatalf("expected no timeout after complete")
	}
}

func TestInstanceLock(t *testing.T) {
	tmp := t.TempDir()
	path := tmp + "/instance.lock"
	lock1 := NewInstanceLock(path)
	id1, err := lock1.Acquire(context.Background(), tmp, "test")
	if err != nil {
		t.Fatalf("acquire: %v", err)
	}
	if id1 == "" {
		t.Fatalf("expected id")
	}
	lock2 := NewInstanceLock(path)
	if _, err := lock2.Acquire(context.Background(), tmp, "test"); err == nil {
		t.Fatalf("expected lock held")
	}
	if err := lock1.Release(); err != nil {
		t.Fatalf("release: %v", err)
	}
	lock3 := NewInstanceLock(path)
	if _, err := lock3.Acquire(context.Background(), tmp, "test"); err != nil {
		t.Fatalf("acquire after release: %v", err)
	}
}

func TestJournalRecovery(t *testing.T) {
	journal := NewJournal()
	journal.RecordStartup(StartupJournalEntry{
		StartupID:   "s1",
		ComponentID: "c1",
		Status:      StartupStatusFailed,
		StartedAt:   now(),
	})
	interrupted := journal.InterruptedComponents()
	if len(interrupted) != 1 || interrupted[0] != "c1" {
		t.Fatalf("expected c1 interrupted, got %v", interrupted)
	}
}

func TestReadinessCheck(t *testing.T) {
	registry := NewComponentRegistry()
	c1 := &fakeComponent{id: "c1", state: ComponentStateReady}
	_ = registry.Register(c1, BootstrapComponent{ID: "c1", Phase: StartupPhaseCore, Required: true})
	audit := NewInMemoryAuditWriter()
	readiness := NewReadinessService(registry, audit)
	readiness.MarkCore("c1")
	report := readiness.Check(context.Background())
	if !report.Ready {
		t.Fatalf("expected ready, got %v", report)
	}
}

func TestRecoveryScanner(t *testing.T) {
	journal := NewJournal()
	scanner := NewRecoveryScanner(journal)
	scanner.RegisterScanHook(func(ctx context.Context, startupID string) ([]RecoveryItem, error) {
		return []RecoveryItem{
			{Category: "package", Subject: "op1", Severity: "high"},
			{Category: "runtime", Subject: "r1", Severity: "info"},
		}, nil
	})
	report, err := scanner.Scan(context.Background(), "s1")
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	if len(report.Items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(report.Items))
	}
	if len(report.PendingPackageOps) != 1 {
		t.Errorf("expected 1 package op")
	}
	if len(report.HighRiskItems) != 1 {
		t.Errorf("expected 1 high risk item")
	}
}
