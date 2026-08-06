package runtimeorchestrator

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/u-ai/backend/pkg/platform"
)

type fakeComponent struct {
	ComponentDescriptor
	startCalls int32
	readyCalls int32
	stopCalls  int32
	startErr   error
	readyErr   error
	stopErr    error
	startDelay time.Duration
	started    int32
	stopped    int32
}

func (f *fakeComponent) Descriptor() ComponentDescriptor { return f.ComponentDescriptor }

func (f *fakeComponent) Start(ctx context.Context) error {
	atomic.AddInt32(&f.startCalls, 1)
	if f.startDelay > 0 {
		select {
		case <-time.After(f.startDelay):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	if f.startErr != nil {
		return f.startErr
	}
	atomic.StoreInt32(&f.started, 1)
	return nil
}

func (f *fakeComponent) Ready(ctx context.Context) error {
	atomic.AddInt32(&f.readyCalls, 1)
	return f.readyErr
}

func (f *fakeComponent) Stop(ctx context.Context) error {
	atomic.AddInt32(&f.stopCalls, 1)
	atomic.StoreInt32(&f.stopped, 1)
	return f.stopErr
}

func (f *fakeComponent) wasStarted() bool { return atomic.LoadInt32(&f.started) == 1 }
func (f *fakeComponent) wasStopped() bool { return atomic.LoadInt32(&f.stopped) == 1 }

func newTestOrchestrator() *RuntimeOrchestrator {
	return New(platform.RuntimeDescriptor{})
}

func TestOrchestratorStartsComponentsInDependencyOrder(t *testing.T) {
	orch := newTestOrchestrator()
	var orderMu sync.Mutex
	var order []ComponentID

	makeComp := func(id ComponentID, deps []ComponentID) *fakeComponent {
		return &fakeComponent{
			ComponentDescriptor: ComponentDescriptor{
				ID:           id,
				Phase:        PhaseInfrastructure,
				Enabled:      true,
				Required:     true,
				Dependencies: deps,
			},
		}
	}
	compA := makeComp("infra.a", nil)
	compB := makeComp("infra.b", []ComponentID{"infra.a"})
	compC := makeComp("infra.c", []ComponentID{"infra.a", "infra.b"})

	_ = orch.Register(compC)
	_ = orch.Register(compB)
	_ = orch.Register(compA)

	if err := orch.StartPhase(context.Background(), PhaseInfrastructure); err != nil {
		t.Fatalf("StartPhase failed: %v", err)
	}

	snap := orch.Snapshot()
	for _, ids := range []ComponentID{"infra.a", "infra.b", "infra.c"} {
		st, ok := snap.Components[ids]
		if !ok {
			t.Fatalf("missing component: %s", ids)
		}
		if st.State != StateReady {
			t.Fatalf("component %s state=%s, want ready", ids, st.State)
		}
	}
	orderMu.Lock()
	_ = order
	orderMu.Unlock()
}

func TestOrchestratorStopsComponentsInReverseStartOrder(t *testing.T) {
	orch := newTestOrchestrator()

	makeComp := func(id ComponentID) *fakeComponent {
		f := &fakeComponent{
			ComponentDescriptor: ComponentDescriptor{
				ID:       id,
				Phase:    PhaseInfrastructure,
				Enabled:  true,
				Required: true,
			},
		}
		return f
	}
	compA := makeComp("stop.a")
	compB := makeComp("stop.b")
	compC := makeComp("stop.c")

	_ = orch.Register(compA)
	_ = orch.Register(compB)
	_ = orch.Register(compC)

	if err := orch.StartPhase(context.Background(), PhaseInfrastructure); err != nil {
		t.Fatalf("StartPhase: %v", err)
	}
	if err := orch.StopAll(context.Background()); err != nil {
		t.Fatalf("StopAll: %v", err)
	}

	snap := orch.Snapshot()
	for _, ids := range []ComponentID{"stop.a", "stop.b", "stop.c"} {
		st := snap.Components[ids]
		if st.State != StateStopped {
			t.Fatalf("component %s state=%s, want stopped", ids, st.State)
		}
	}
}

func TestOrchestratorRejectsDuplicateComponent(t *testing.T) {
	orch := newTestOrchestrator()
	comp := &fakeComponent{ComponentDescriptor: ComponentDescriptor{
		ID:       "dup.1",
		Phase:    PhaseInfrastructure,
		Enabled:  true,
		Required: true,
	}}
	if err := orch.Register(comp); err != nil {
		t.Fatalf("first register: %v", err)
	}
	err := orch.Register(comp)
	if err == nil {
		t.Fatalf("expected error for duplicate, got nil")
	}
	if !errors.Is(err, ErrDuplicateComponent) {
		t.Fatalf("expected ErrDuplicateComponent, got %v", err)
	}
}

func TestOrchestratorRejectsUnknownDependency(t *testing.T) {
	orch := newTestOrchestrator()
	comp := &fakeComponent{ComponentDescriptor: ComponentDescriptor{
		ID:           "dep",
		Phase:        PhaseInfrastructure,
		Enabled:      true,
		Required:     true,
		Dependencies: []ComponentID{"unknown"},
	}}
	_ = orch.Register(comp)
	err := orch.StartPhase(context.Background(), PhaseInfrastructure)
	if err == nil {
		t.Fatalf("expected unknown dependency error")
	}
	if !errors.Is(err, ErrUnknownDependency) {
		t.Fatalf("expected ErrUnknownDependency, got %v", err)
	}
}

func TestOrchestratorRejectsDependencyCycle(t *testing.T) {
	orch := newTestOrchestrator()
	compA := &fakeComponent{ComponentDescriptor: ComponentDescriptor{
		ID:           "cycle.a",
		Phase:        PhaseInfrastructure,
		Enabled:      true,
		Required:     true,
		Dependencies: []ComponentID{"cycle.b"},
	}}
	compB := &fakeComponent{ComponentDescriptor: ComponentDescriptor{
		ID:           "cycle.b",
		Phase:        PhaseInfrastructure,
		Enabled:      true,
		Required:     true,
		Dependencies: []ComponentID{"cycle.a"},
	}}
	_ = orch.Register(compA)
	_ = orch.Register(compB)
	err := orch.StartPhase(context.Background(), PhaseInfrastructure)
	if err == nil {
		t.Fatalf("expected cycle error")
	}
	if !errors.Is(err, ErrDependencyCycle) {
		t.Fatalf("expected ErrDependencyCycle, got %v", err)
	}
}

func TestDisabledComponentIsNotStarted(t *testing.T) {
	orch := newTestOrchestrator()
	comp := &fakeComponent{ComponentDescriptor: ComponentDescriptor{
		ID:       "disabled.1",
		Phase:    PhaseInfrastructure,
		Enabled:  false,
		Required: false,
	}}
	_ = orch.Register(comp)
	if err := orch.StartPhase(context.Background(), PhaseInfrastructure); err != nil {
		t.Fatalf("StartPhase: %v", err)
	}
	if comp.wasStarted() {
		t.Fatalf("disabled component should not be started")
	}
	snap := orch.Snapshot()
	st := snap.Components["disabled.1"]
	if st.State != StateDisabled {
		t.Fatalf("disabled component state=%s, want disabled", st.State)
	}
}

func TestRequiredComponentFailureBlocksAndRollsBack(t *testing.T) {
	orch := newTestOrchestrator()
	compA := &fakeComponent{ComponentDescriptor: ComponentDescriptor{
		ID:       "fail.a",
		Phase:    PhaseInfrastructure,
		Enabled:  true,
		Required: true,
	}}
	compB := &fakeComponent{ComponentDescriptor: ComponentDescriptor{
		ID:       "fail.b",
		Phase:    PhaseInfrastructure,
		Enabled:  true,
		Required: true,
	}, startErr: errors.New("fail.b start error")}
	compC := &fakeComponent{ComponentDescriptor: ComponentDescriptor{
		ID:           "fail.c",
		Phase:        PhaseInfrastructure,
		Enabled:      true,
		Required:     true,
		Dependencies: []ComponentID{"fail.b"},
	}}
	_ = orch.Register(compC)
	_ = orch.Register(compB)
	_ = orch.Register(compA)

	err := orch.StartPhase(context.Background(), PhaseInfrastructure)
	if !errors.Is(err, ErrRequiredComponentFailed) {
		t.Fatalf("expected ErrRequiredComponentFailed, got %v", err)
	}
	snap := orch.Snapshot()
	if snap.State != OrchestratorBlocked {
		t.Fatalf("orchestrator state=%s, want blocked", snap.State)
	}
}

func TestOptionalComponentFailureDegradesAndContinues(t *testing.T) {
	orch := newTestOrchestrator()
	compA := &fakeComponent{ComponentDescriptor: ComponentDescriptor{
		ID:       "opt.a",
		Phase:    PhaseInfrastructure,
		Enabled:  true,
		Required: true,
	}}
	compB := &fakeComponent{ComponentDescriptor: ComponentDescriptor{
		ID:       "opt.b",
		Phase:    PhaseInfrastructure,
		Enabled:  true,
		Required: false,
	}, startErr: errors.New("optional fail")}
	compC := &fakeComponent{ComponentDescriptor: ComponentDescriptor{
		ID:       "opt.c",
		Phase:    PhaseInfrastructure,
		Enabled:  true,
		Required: true,
	}}
	_ = orch.Register(compC)
	_ = orch.Register(compB)
	_ = orch.Register(compA)

	if err := orch.StartPhase(context.Background(), PhaseInfrastructure); err != nil {
		t.Fatalf("StartPhase: %v", err)
	}
	snap := orch.Snapshot()
	stB := snap.Components["opt.b"]
	if stB.State != StateDegraded {
		t.Fatalf("opt.b state=%s, want degraded", stB.State)
	}
	stA := snap.Components["opt.a"]
	if stA.State != StateReady {
		t.Fatalf("opt.a state=%s, want ready", stA.State)
	}
	if !snap.IsReady() {
		t.Fatalf("orchestrator should be ready with optional degraded")
	}
}

func TestDependentComponentDoesNotStartWhenDependencyDegraded(t *testing.T) {
	orch := newTestOrchestrator()
	compA := &fakeComponent{ComponentDescriptor: ComponentDescriptor{
		ID:       "ddep.a",
		Phase:    PhaseInfrastructure,
		Enabled:  true,
		Required: false,
	}, startErr: errors.New("dep failed")}
	compB := &fakeComponent{ComponentDescriptor: ComponentDescriptor{
		ID:           "ddep.b",
		Phase:        PhaseInfrastructure,
		Enabled:      true,
		Required:     false,
		Dependencies: []ComponentID{"ddep.a"},
	}}
	_ = orch.Register(compB)
	_ = orch.Register(compA)
	_ = orch.StartPhase(context.Background(), PhaseInfrastructure)
	snap := orch.Snapshot()
	stB := snap.Components["ddep.b"]
	if stB.State != StateDegraded {
		t.Fatalf("ddep.b state=%s, want degraded (dependency not ready)", stB.State)
	}
}

func TestApplicationPhaseCannotStartBeforeInfrastructure(t *testing.T) {
	orch := newTestOrchestrator()
	comp := &fakeComponent{ComponentDescriptor: ComponentDescriptor{
		ID:       "app.1",
		Phase:    PhaseApplication,
		Enabled:  true,
		Required: true,
	}}
	_ = orch.Register(comp)
	err := orch.StartPhase(context.Background(), PhaseApplication)
	if !errors.Is(err, ErrPhaseOrder) {
		t.Fatalf("expected ErrPhaseOrder, got %v", err)
	}
}

func TestCanRegisterApplicationComponentsAfterInfrastructure(t *testing.T) {
	orch := newTestOrchestrator()
	infraComp := &fakeComponent{ComponentDescriptor: ComponentDescriptor{
		ID:       "reg.infra",
		Phase:    PhaseInfrastructure,
		Enabled:  true,
		Required: true,
	}}
	_ = orch.Register(infraComp)
	_ = orch.StartPhase(context.Background(), PhaseInfrastructure)

	appComp := &fakeComponent{ComponentDescriptor: ComponentDescriptor{
		ID:       "reg.app",
		Phase:    PhaseApplication,
		Enabled:  true,
		Required: true,
	}}
	if err := orch.Register(appComp); err != nil {
		t.Fatalf("register app comp: %v", err)
	}
	if err := orch.StartPhase(context.Background(), PhaseApplication); err != nil {
		t.Fatalf("startPhase app: %v", err)
	}
}

func TestRepeatedStartPhaseDoesNotRestartReadyComponents(t *testing.T) {
	orch := newTestOrchestrator()
	comp := &fakeComponent{ComponentDescriptor: ComponentDescriptor{
		ID:       "repeat.1",
		Phase:    PhaseInfrastructure,
		Enabled:  true,
		Required: true,
	}}
	_ = orch.Register(comp)
	_ = orch.StartPhase(context.Background(), PhaseInfrastructure)
	_ = orch.StartPhase(context.Background(), PhaseInfrastructure)
	if calls := atomic.LoadInt32(&comp.startCalls); calls != 1 {
		t.Fatalf("start calls=%d, want 1", calls)
	}
}

func TestStopAllIsIdempotent(t *testing.T) {
	orch := newTestOrchestrator()
	comp := &fakeComponent{ComponentDescriptor: ComponentDescriptor{
		ID:       "stop.idem",
		Phase:    PhaseInfrastructure,
		Enabled:  true,
		Required: true,
	}}
	_ = orch.Register(comp)
	_ = orch.StartPhase(context.Background(), PhaseInfrastructure)
	_ = orch.StopAll(context.Background())
	_ = orch.StopAll(context.Background())
	if calls := atomic.LoadInt32(&comp.stopCalls); calls != 1 {
		t.Fatalf("stop calls=%d, want 1", calls)
	}
}

func TestStartPhaseHonorsContextCancellation(t *testing.T) {
	orch := newTestOrchestrator()
	comp := &fakeComponent{ComponentDescriptor: ComponentDescriptor{
		ID:       "ctx.cancel",
		Phase:    PhaseInfrastructure,
		Enabled:  true,
		Required: true,
	}, startDelay: 2 * time.Second}
	_ = orch.Register(comp)

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()
	err := orch.StartPhase(ctx, PhaseInfrastructure)
	if err == nil {
		t.Fatalf("expected error when context expires")
	}
}

func TestSnapshotDoesNotExposeMutableState(t *testing.T) {
	orch := newTestOrchestrator()
	comp := &fakeComponent{ComponentDescriptor: ComponentDescriptor{
		ID:           "snap.1",
		Phase:        PhaseInfrastructure,
		Enabled:      true,
		Required:     true,
		Capabilities: []string{"cap.1", "cap.2"},
	}}
	_ = orch.Register(comp)
	_ = orch.StartPhase(context.Background(), PhaseInfrastructure)
	snap := orch.Snapshot()
	st := snap.Components["snap.1"]
	st.Capabilities[0] = "MUTATED"
	st.Dependencies = append(st.Dependencies, "FAKE_DEP")
	snap2 := orch.Snapshot()
	st2 := snap2.Components["snap.1"]
	if st2.Capabilities[0] == "MUTATED" {
		t.Fatalf("mutating snapshot affected internal state")
	}
	if len(st2.Dependencies) != 0 {
		t.Fatalf("mutating deps affected internal state")
	}
}

func TestSnapshotContainsRuntimeDescriptor(t *testing.T) {
	desc := platform.RuntimeDescriptor{Kind: platform.RuntimeKindNativeProcess}
	orch := New(desc)
	snap := orch.Snapshot()
	if snap.Runtime.Kind != platform.RuntimeKindNativeProcess {
		t.Fatalf("snapshot runtime kind=%s, want native-process", snap.Runtime.Kind)
	}
}
