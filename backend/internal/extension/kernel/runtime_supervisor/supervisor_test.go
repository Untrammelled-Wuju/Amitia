package runtime_supervisor

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/domain"
)

func makeSpec(rt domain.RuntimeType, gen int64) InstanceSpec {
	return InstanceSpec{
		DefinitionID: "rt-def-1",
		ExtensionID:  "com.example/test",
		ModuleID:     "main",
		RuntimeType:  rt,
		Generation:   gen,
		Strategy:     StrategySingletonPerModule,
		Limits: ResourceLimits{
			MaxMemoryBytes:     128 * 1024 * 1024,
			MaxConcurrentCalls: 4,
			MaxQueueDepth:      16,
		},
		MaxRestarts:   3,
		RestartWindow: 5 * time.Minute,
	}
}

func TestRegisterFactory(t *testing.T) {
	s := NewDefaultSupervisor()
	f := NewFakeFactory(domain.RuntimeTypeBuiltin, NewFakeRuntime())
	if err := s.RegisterFactory(f); err != nil {
		t.Fatalf("RegisterFactory: %v", err)
	}
	if err := s.RegisterFactory(f); err == nil {
		t.Errorf("expected duplicate error")
	}
}

func TestReconcileStart(t *testing.T) {
	s := NewDefaultSupervisor()
	rt := NewFakeRuntime()
	if err := s.RegisterFactory(NewFakeFactory(domain.RuntimeTypeBuiltin, rt)); err != nil {
		t.Fatal(err)
	}
	spec := makeSpec(domain.RuntimeTypeBuiltin, 1)
	result := s.Reconcile(context.Background(), ReconcileRequest{
		DefinitionID: "rt-def-1",
		Desired:      DesiredRunning,
		Spec:         spec,
	})
	if result.Error != nil {
		t.Fatalf("Reconcile: %v", result.Error)
	}
	if result.Actual != ActualReady {
		t.Errorf("expected ready, got %s", result.Actual)
	}
	if result.InstanceID == "" {
		t.Errorf("expected instance id")
	}
}

func TestReconcileStop(t *testing.T) {
	s := NewDefaultSupervisor()
	rt := NewFakeRuntime()
	_ = s.RegisterFactory(NewFakeFactory(domain.RuntimeTypeBuiltin, rt))
	spec := makeSpec(domain.RuntimeTypeBuiltin, 1)
	startResult := s.Reconcile(context.Background(), ReconcileRequest{
		DefinitionID: "rt-def-1", Desired: DesiredRunning, Spec: spec,
	})
	if startResult.Error != nil {
		t.Fatal(startResult.Error)
	}
	stopResult := s.Reconcile(context.Background(), ReconcileRequest{
		DefinitionID: "rt-def-1", Desired: DesiredStopped, Spec: spec,
	})
	if stopResult.Error != nil {
		t.Fatalf("stop: %v", stopResult.Error)
	}
	if stopResult.Actual != ActualStopped {
		t.Errorf("expected stopped, got %s", stopResult.Actual)
	}
}

func TestReconcileNoFactory(t *testing.T) {
	s := NewDefaultSupervisor()
	result := s.Reconcile(context.Background(), ReconcileRequest{
		DefinitionID: "rt-def-1",
		Desired:      DesiredRunning,
		Spec:         makeSpec(domain.RuntimeTypeJavaScript, 1),
	})
	if result.Error == nil {
		t.Errorf("expected error")
	}
	if result.Actual != ActualFailed {
		t.Errorf("expected failed, got %s", result.Actual)
	}
}

func TestInvoke(t *testing.T) {
	s := NewDefaultSupervisor()
	rt := NewFakeRuntime()
	_ = s.RegisterFactory(NewFakeFactory(domain.RuntimeTypeBuiltin, rt))
	spec := makeSpec(domain.RuntimeTypeBuiltin, 1)
	startResult := s.Reconcile(context.Background(), ReconcileRequest{
		DefinitionID: "rt-def-1", Desired: DesiredRunning, Spec: spec,
	})
	res := s.Invoke(context.Background(), InvocationRequest{
		InstanceID:   startResult.InstanceID,
		InvocationID: "inv-1",
		Operation:    "ping",
		Generation:   1,
	})
	if res.Error != nil {
		t.Fatalf("Invoke: %v", res.Error)
	}
	if res.Status != "success" {
		t.Errorf("expected success, got %s", res.Status)
	}
}

func TestInvokeNotFound(t *testing.T) {
	s := NewDefaultSupervisor()
	res := s.Invoke(context.Background(), InvocationRequest{
		InstanceID:   "nonexistent",
		InvocationID: "inv-1",
	})
	if !errors.Is(res.Error, ErrInstanceNotFound) {
		t.Errorf("expected ErrInstanceNotFound, got %v", res.Error)
	}
}

func TestInvokeGenerationMismatch(t *testing.T) {
	s := NewDefaultSupervisor()
	rt := NewFakeRuntime()
	_ = s.RegisterFactory(NewFakeFactory(domain.RuntimeTypeBuiltin, rt))
	spec := makeSpec(domain.RuntimeTypeBuiltin, 1)
	startResult := s.Reconcile(context.Background(), ReconcileRequest{
		DefinitionID: "rt-def-1", Desired: DesiredRunning, Spec: spec,
	})
	res := s.Invoke(context.Background(), InvocationRequest{
		InstanceID:   startResult.InstanceID,
		InvocationID: "inv-1",
		Generation:   999,
	})
	if !errors.Is(res.Error, ErrGenerationMismatch) {
		t.Errorf("expected generation mismatch, got %v", res.Error)
	}
}

func TestCircuitBreaker(t *testing.T) {
	s := NewDefaultSupervisor()
	s.SetCircuitConfig(CircuitConfig{FailureThreshold: 2, RecoveryAfter: time.Second, HalfOpenAttempts: 1})
	rt := NewFakeRuntime()
	rt.SetInvokeErr(ErrFakeInvoke)
	_ = s.RegisterFactory(NewFakeFactory(domain.RuntimeTypeBuiltin, rt))
	spec := makeSpec(domain.RuntimeTypeBuiltin, 1)
	startResult := s.Reconcile(context.Background(), ReconcileRequest{
		DefinitionID: "rt-def-1", Desired: DesiredRunning, Spec: spec,
	})
	for i := 0; i < 2; i++ {
		s.Invoke(context.Background(), InvocationRequest{
			InstanceID:   startResult.InstanceID,
			InvocationID: "inv",
			Generation:   1,
		})
	}
	snap, _ := s.GetInstance(context.Background(), startResult.InstanceID)
	if snap.Circuit != CircuitOpen {
		t.Errorf("expected circuit open, got %s", snap.Circuit)
	}
	res := s.Invoke(context.Background(), InvocationRequest{
		InstanceID:   startResult.InstanceID,
		InvocationID: "inv",
		Generation:   1,
	})
	if !errors.Is(res.Error, ErrCircuitOpen) {
		t.Errorf("expected circuit open error, got %v", res.Error)
	}
}

func TestRestartOnCrash(t *testing.T) {
	s := NewDefaultSupervisor()
	rt := NewFakeRuntime()
	_ = s.RegisterFactory(NewFakeFactory(domain.RuntimeTypeBuiltin, rt))
	spec := makeSpec(domain.RuntimeTypeBuiltin, 1)
	startResult := s.Reconcile(context.Background(), ReconcileRequest{
		DefinitionID: "rt-def-1", Desired: DesiredRunning, Spec: spec,
	})
	s.mu.Lock()
	entry := s.instances[startResult.InstanceID]
	entry.actual = ActualCrashed
	s.mu.Unlock()
	result := s.Reconcile(context.Background(), ReconcileRequest{
		DefinitionID: "rt-def-1", Desired: DesiredRunning, Spec: spec,
	})
	if result.Error != nil {
		t.Fatalf("restart: %v", result.Error)
	}
	if result.Actual != ActualReady {
		t.Errorf("expected ready after restart, got %s", result.Actual)
	}
}

func TestMaxRestartsExceeded(t *testing.T) {
	s := NewDefaultSupervisor()
	rt := NewFakeRuntime()
	rt.SetStartErr(ErrFakeStart)
	_ = s.RegisterFactory(NewFakeFactory(domain.RuntimeTypeBuiltin, rt))
	spec := makeSpec(domain.RuntimeTypeBuiltin, 1)
	spec.MaxRestarts = 2
	startResult := s.Reconcile(context.Background(), ReconcileRequest{
		DefinitionID: "rt-def-1", Desired: DesiredRunning, Spec: spec,
	})
	if startResult.Error == nil {
		t.Fatalf("expected start error")
	}
	s.mu.Lock()
	entry := s.instances[startResult.InstanceID]
	if entry == nil {
		s.mu.Unlock()
		t.Skip("no instance created")
	}
	s.mu.Unlock()
}

func TestStopAndSnapshot(t *testing.T) {
	s := NewDefaultSupervisor()
	rt := NewFakeRuntime()
	_ = s.RegisterFactory(NewFakeFactory(domain.RuntimeTypeBuiltin, rt))
	spec := makeSpec(domain.RuntimeTypeBuiltin, 1)
	startResult := s.Reconcile(context.Background(), ReconcileRequest{
		DefinitionID: "rt-def-1", Desired: DesiredRunning, Spec: spec,
	})
	if err := s.Stop(context.Background(), startResult.InstanceID, StopReasonManual); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	snap := s.Snapshot(context.Background(), "rt-def-1")
	if len(snap.Instances) != 1 {
		t.Fatalf("expected 1 instance, got %d", len(snap.Instances))
	}
	if snap.Instances[0].Actual != ActualStopped {
		t.Errorf("expected stopped, got %s", snap.Instances[0].Actual)
	}
}

func TestManualRestart(t *testing.T) {
	s := NewDefaultSupervisor()
	rt := NewFakeRuntime()
	_ = s.RegisterFactory(NewFakeFactory(domain.RuntimeTypeBuiltin, rt))
	spec := makeSpec(domain.RuntimeTypeBuiltin, 1)
	startResult := s.Reconcile(context.Background(), ReconcileRequest{
		DefinitionID: "rt-def-1", Desired: DesiredRunning, Spec: spec,
	})
	if err := s.Restart(context.Background(), startResult.InstanceID); err != nil {
		t.Fatalf("Restart: %v", err)
	}
	snap, _ := s.GetInstance(context.Background(), startResult.InstanceID)
	if snap.Actual != ActualReady {
		t.Errorf("expected ready after restart, got %s", snap.Actual)
	}
}

func TestGenerationUpgrade(t *testing.T) {
	s := NewDefaultSupervisor()
	rt := NewFakeRuntime()
	_ = s.RegisterFactory(NewFakeFactory(domain.RuntimeTypeBuiltin, rt))
	spec1 := makeSpec(domain.RuntimeTypeBuiltin, 1)
	start1 := s.Reconcile(context.Background(), ReconcileRequest{
		DefinitionID: "rt-def-1", Desired: DesiredRunning, Spec: spec1,
	})
	firstInstance := start1.InstanceID
	spec2 := makeSpec(domain.RuntimeTypeBuiltin, 2)
	start2 := s.Reconcile(context.Background(), ReconcileRequest{
		DefinitionID: "rt-def-1", Desired: DesiredRunning, Spec: spec2,
	})
	if start2.InstanceID == firstInstance {
		t.Errorf("expected new instance for new generation")
	}
	if start2.Error != nil {
		t.Fatalf("expected no error, got %v", start2.Error)
	}
}

func TestConcurrentInvoke(t *testing.T) {
	s := NewDefaultSupervisor()
	rt := NewFakeRuntime()
	_ = s.RegisterFactory(NewFakeFactory(domain.RuntimeTypeBuiltin, rt))
	spec := makeSpec(domain.RuntimeTypeBuiltin, 1)
	startResult := s.Reconcile(context.Background(), ReconcileRequest{
		DefinitionID: "rt-def-1", Desired: DesiredRunning, Spec: spec,
	})
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			s.Invoke(context.Background(), InvocationRequest{
				InstanceID:   startResult.InstanceID,
				InvocationID: "inv",
				Operation:    "ping",
				Generation:   1,
			})
		}(i)
	}
	wg.Wait()
	if rt.invokeCount != 10 {
		t.Errorf("expected 10 invokes, got %d", rt.invokeCount)
	}
}

func TestStopPropagatesRuntimeError(t *testing.T) {
	s := NewDefaultSupervisor()
	rt := NewFakeRuntime()
	rt.SetStopErr(ErrFakeStop)
	_ = s.RegisterFactory(NewFakeFactory(domain.RuntimeTypeBuiltin, rt))
	spec := makeSpec(domain.RuntimeTypeBuiltin, 1)
	startResult := s.Reconcile(context.Background(), ReconcileRequest{
		DefinitionID: "rt-def-1", Desired: DesiredRunning, Spec: spec,
	})
	if startResult.Error != nil {
		t.Fatalf("start: %v", startResult.Error)
	}
	err := s.Stop(context.Background(), startResult.InstanceID, StopReasonUninstall)
	if !errors.Is(err, ErrFakeStop) {
		t.Fatalf("expected ErrFakeStop from Stop, got %v", err)
	}
	snap := s.Snapshot(context.Background(), "rt-def-1")
	if len(snap.Instances) != 1 {
		t.Fatalf("expected 1 instance, got %d", len(snap.Instances))
	}
	if snap.Instances[0].Actual != ActualFailed {
		t.Errorf("expected ActualFailed after stop error, got %s", snap.Instances[0].Actual)
	}
}

func TestStopReconcilePropagatesRuntimeError(t *testing.T) {
	s := NewDefaultSupervisor()
	rt := NewFakeRuntime()
	_ = s.RegisterFactory(NewFakeFactory(domain.RuntimeTypeBuiltin, rt))
	spec := makeSpec(domain.RuntimeTypeBuiltin, 1)
	startResult := s.Reconcile(context.Background(), ReconcileRequest{
		DefinitionID: "rt-def-1", Desired: DesiredRunning, Spec: spec,
	})
	if startResult.Error != nil {
		t.Fatalf("start: %v", startResult.Error)
	}
	rt.SetStopErr(ErrFakeStop)
	stopResult := s.Reconcile(context.Background(), ReconcileRequest{
		DefinitionID: "rt-def-1", Desired: DesiredStopped, Spec: spec,
	})
	if stopResult.Error == nil {
		t.Fatalf("expected error from stop reconcile")
	}
	if !errors.Is(stopResult.Error, ErrFakeStop) {
		t.Errorf("expected ErrFakeStop, got %v", stopResult.Error)
	}
	if stopResult.Actual != ActualFailed {
		t.Errorf("expected ActualFailed, got %s", stopResult.Actual)
	}
}

func TestSnapshotByExtension(t *testing.T) {
	s := NewDefaultSupervisor()
	rt := NewFakeRuntime()
	_ = s.RegisterFactory(NewFakeFactory(domain.RuntimeTypeBuiltin, rt))
	spec := makeSpec(domain.RuntimeTypeBuiltin, 1)
	startResult := s.Reconcile(context.Background(), ReconcileRequest{
		DefinitionID: "rt-def-1", Desired: DesiredRunning, Spec: spec,
	})
	if startResult.Error != nil {
		t.Fatalf("start: %v", startResult.Error)
	}
	snapshots := s.SnapshotByExtension(context.Background(), "com.example/test", "main")
	if len(snapshots) != 1 {
		t.Fatalf("expected 1 snapshot, got %d", len(snapshots))
	}
	snap := snapshots[0]
	if snap.InstanceID != startResult.InstanceID {
		t.Errorf("expected instanceID %s, got %s", startResult.InstanceID, snap.InstanceID)
	}
	if snap.Health != HealthHealthy {
		t.Errorf("expected healthy, got %s", snap.Health)
	}
	if snap.Circuit != CircuitClosed {
		t.Errorf("expected circuit closed, got %s", snap.Circuit)
	}
	if snap.Actual != ActualReady {
		t.Errorf("expected ready, got %s", snap.Actual)
	}
	if snap.Quarantined {
		t.Errorf("expected not quarantined")
	}
	if snap.Generation != 1 {
		t.Errorf("expected generation 1, got %d", snap.Generation)
	}
}

func TestSnapshotByExtensionWrongExtension(t *testing.T) {
	s := NewDefaultSupervisor()
	rt := NewFakeRuntime()
	_ = s.RegisterFactory(NewFakeFactory(domain.RuntimeTypeBuiltin, rt))
	spec := makeSpec(domain.RuntimeTypeBuiltin, 1)
	_ = s.Reconcile(context.Background(), ReconcileRequest{
		DefinitionID: "rt-def-1", Desired: DesiredRunning, Spec: spec,
	})
	snapshots := s.SnapshotByExtension(context.Background(), "com.other/extension", "main")
	if len(snapshots) != 0 {
		t.Errorf("expected 0 snapshots for wrong extension, got %d", len(snapshots))
	}
}

func TestSnapshotByExtensionQuarantined(t *testing.T) {
	s := NewDefaultSupervisor()
	rt := NewFakeRuntime()
	_ = s.RegisterFactory(NewFakeFactory(domain.RuntimeTypeBuiltin, rt))
	spec := makeSpec(domain.RuntimeTypeBuiltin, 1)
	spec.MaxRestarts = 1
	startResult := s.Reconcile(context.Background(), ReconcileRequest{
		DefinitionID: "rt-def-1", Desired: DesiredRunning, Spec: spec,
	})
	s.mu.Lock()
	entry := s.instances[startResult.InstanceID]
	if entry != nil {
		entry.actual = ActualQuarantined
		entry.health = HealthUnhealthy
	}
	s.mu.Unlock()
	snapshots := s.SnapshotByExtension(context.Background(), "com.example/test", "main")
	if len(snapshots) != 1 {
		t.Fatalf("expected 1 snapshot, got %d", len(snapshots))
	}
	if !snapshots[0].Quarantined {
		t.Errorf("expected quarantined=true")
	}
	if snapshots[0].Health != HealthUnhealthy {
		t.Errorf("expected unhealthy, got %s", snapshots[0].Health)
	}
}

func TestServiceRuntimeResolver(t *testing.T) {
	s := NewDefaultSupervisor()
	rt := NewFakeRuntime()
	if err := s.RegisterFactory(NewFakeFactory(domain.RuntimeTypeService, rt)); err != nil {
		t.Fatalf("RegisterFactory: %v", err)
	}
	spec := makeSpec(domain.RuntimeTypeService, 1)
	result := s.Reconcile(context.Background(), ReconcileRequest{
		DefinitionID: "rt-def-1",
		Desired:      DesiredRunning,
		Spec:         spec,
	})
	if result.Error != nil {
		t.Fatalf("Reconcile with service runtime type should find factory, got error: %v", result.Error)
	}
	if result.InstanceID == "" {
		t.Errorf("expected instance id for service runtime")
	}
}

func TestUnknownRuntimeTypeNotFound(t *testing.T) {
	s := NewDefaultSupervisor()
	rt := NewFakeRuntime()
	_ = s.RegisterFactory(NewFakeFactory(domain.RuntimeTypeService, rt))
	spec := makeSpec("foobar", 1)
	result := s.Reconcile(context.Background(), ReconcileRequest{
		DefinitionID: "rt-def-1",
		Desired:      DesiredRunning,
		Spec:         spec,
	})
	if result.Error == nil {
		t.Errorf("expected error for unknown runtime type")
	}
}
