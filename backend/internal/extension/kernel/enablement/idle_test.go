//go:build rsi_lifecycle

package enablement

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/domain"
	"github.com/u-ai/backend/internal/extension/kernel/runtime_supervisor"
)

type idleFixture struct {
	installer  *Installer
	reaper     *IdleReaper
	spawner    *executeSpawner
	supervisor *runtime_supervisor.DefaultSupervisor
	runtime    *runtime_supervisor.FakeRuntime
	clock      *time.Time
	instanceID string
}

func idleDrainer(t *testing.T) (*runtime_supervisor.DefaultSupervisor, *runtime_supervisor.FakeRuntime, string) {
	t.Helper()
	supervisor := runtime_supervisor.NewDefaultSupervisor()
	runtime := runtime_supervisor.NewFakeRuntime()
	if err := supervisor.RegisterFactory(runtime_supervisor.NewFakeFactory(domain.RuntimeTypeBuiltin, runtime)); err != nil {
		t.Fatalf("RegisterFactory: %v", err)
	}
	spec := runtime_supervisor.InstanceSpec{
		DefinitionID: "rt-def-idle",
		ExtensionID:  "com.example/idle",
		ModuleID:     "main",
		RuntimeType:  domain.RuntimeTypeBuiltin,
		Generation:   1,
		Strategy:     runtime_supervisor.StrategySingletonPerModule,
		Limits: runtime_supervisor.ResourceLimits{
			MaxMemoryBytes:     64 * 1024 * 1024,
			MaxConcurrentCalls: 2,
			MaxQueueDepth:      8,
		},
		MaxRestarts:   3,
		RestartWindow: time.Minute,
	}
	result := supervisor.Reconcile(context.Background(), runtime_supervisor.ReconcileRequest{
		DefinitionID: "rt-def-idle",
		Desired:      runtime_supervisor.DesiredRunning,
		Spec:         spec,
	})
	if result.Error != nil {
		t.Fatalf("Reconcile: %v", result.Error)
	}
	if result.InstanceID == "" {
		t.Fatal("supervisor returned no instance id")
	}
	return supervisor, runtime, result.InstanceID
}

func newIdleFixture(t *testing.T) *idleFixture {
	t.Helper()
	supervisor, runtime, instanceID := idleDrainer(t)
	spawner := &executeSpawner{}
	installer := NewInstaller(spawner)
	clock := time.Date(2026, 11, 25, 12, 0, 0, 0, time.UTC)
	reaper, err := NewIdleReaperWithClock(installer, IdlePolicy{TTL: time.Minute, DrainTimeout: time.Second}, supervisor, func(string) (string, bool) {
		return instanceID, true
	}, func() time.Time { return clock })
	if err != nil {
		t.Fatalf("NewIdleReaperWithClock: %v", err)
	}
	installer.WithActivity(reaper.Observe)
	if _, err := installer.InstallBatch(context.Background(), metadataManifests(1)); err != nil {
		t.Fatalf("install: %v", err)
	}
	if err := installer.Enable(context.Background(), "plugin-0000", 1); err != nil {
		t.Fatalf("enable: %v", err)
	}
	return &idleFixture{installer: installer, reaper: reaper, spawner: spawner, supervisor: supervisor, runtime: runtime, clock: &clock, instanceID: instanceID}
}

func (f *idleFixture) advance(duration time.Duration) {
	*f.clock = f.clock.Add(duration)
}

func (f *idleFixture) warm(t *testing.T) {
	t.Helper()
	f.installer.mu.Lock()
	machine := f.installer.machines["plugin-0000"]
	f.installer.mu.Unlock()
	if _, err := machine.Apply(context.Background(), EventWarm, machine.Epoch()+1); err != nil {
		t.Fatalf("warm: %v", err)
	}
}

func idleLog(t *testing.T, format string, args ...any) {
	t.Helper()
	t.Logf("m06-s04 "+format, args...)
}

func TestPluginIdleReleasesAfterTTL(t *testing.T) {
	ctx := context.Background()
	fixture := newIdleFixture(t)
	if _, err := fixture.installer.Execute(ctx, "plugin-0000", ExecuteRequest{ID: "req-1"}); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if fixture.installer.Snapshot().Processes != 1 {
		t.Fatal("cold start did not create a runtime")
	}
	fixture.advance(time.Minute + time.Second)
	report, err := fixture.reaper.Sweep(ctx)
	if err != nil {
		t.Fatalf("sweep: %v", err)
	}
	if len(report.Released) != 1 || report.Released[0] != "plugin-0000" {
		t.Fatalf("released=%v", report.Released)
	}
	if state, _ := fixture.installer.State("plugin-0000"); state != LifecycleEnabled {
		t.Fatalf("state=%s want enabled", state)
	}
	if fixture.installer.Snapshot().Processes != 0 {
		t.Fatalf("release left a runtime")
	}
	if fixture.runtime.StopCount() != 1 {
		t.Fatalf("supervisor stop count=%d want 1", fixture.runtime.StopCount())
	}
	idleLog(t, "release after_ttl=true state=enabled processes=0 supervisor_stops=1")
}

func TestPluginIdleKeepsRecentRuntime(t *testing.T) {
	ctx := context.Background()
	fixture := newIdleFixture(t)
	if _, err := fixture.installer.Execute(ctx, "plugin-0000", ExecuteRequest{ID: "req-1"}); err != nil {
		t.Fatalf("execute: %v", err)
	}
	fixture.advance(time.Minute - time.Second)
	report, err := fixture.reaper.Sweep(ctx)
	if err != nil {
		t.Fatalf("sweep: %v", err)
	}
	if len(report.Released) != 0 || len(report.Skipped) != 1 {
		t.Fatalf("report=%+v", report)
	}
	if fixture.installer.Snapshot().Processes != 1 {
		t.Fatal("recent runtime was released")
	}
	if fixture.runtime.StopCount() != 0 {
		t.Fatalf("supervisor stopped %d times", fixture.runtime.StopCount())
	}
	idleLog(t, "keep before_ttl=true processes=1 supervisor_stops=0")
}

func TestPluginIdleTouchExtendsDeadline(t *testing.T) {
	ctx := context.Background()
	fixture := newIdleFixture(t)
	if _, err := fixture.installer.Execute(ctx, "plugin-0000", ExecuteRequest{ID: "req-1"}); err != nil {
		t.Fatalf("execute 1: %v", err)
	}
	fixture.advance(45 * time.Second)
	if _, err := fixture.installer.Execute(ctx, "plugin-0000", ExecuteRequest{ID: "req-2"}); err != nil {
		t.Fatalf("execute 2: %v", err)
	}
	fixture.advance(45 * time.Second)
	report, err := fixture.reaper.Sweep(ctx)
	if err != nil {
		t.Fatalf("sweep: %v", err)
	}
	if len(report.Released) != 0 {
		t.Fatalf("touch did not extend the deadline: %v", report.Released)
	}
	if fixture.installer.Snapshot().Processes != 1 {
		t.Fatal("runtime was released despite recent use")
	}
	idleLog(t, "touch extends=true total=90s ttl=60s released=0")
}

func TestPluginIdleReleasesWarmRuntime(t *testing.T) {
	ctx := context.Background()
	fixture := newIdleFixture(t)
	if _, err := fixture.installer.Execute(ctx, "plugin-0000", ExecuteRequest{ID: "req-1"}); err != nil {
		t.Fatalf("execute: %v", err)
	}
	fixture.warm(t)
	if state, _ := fixture.installer.State("plugin-0000"); state != LifecycleWarm {
		t.Fatalf("state=%s want warm", state)
	}
	fixture.advance(time.Minute + time.Second)
	report, err := fixture.reaper.Sweep(ctx)
	if err != nil {
		t.Fatalf("sweep: %v", err)
	}
	if len(report.Released) != 1 {
		t.Fatalf("released=%v", report.Released)
	}
	if state, _ := fixture.installer.State("plugin-0000"); state != LifecycleEnabled {
		t.Fatalf("warm release landed in %s want enabled", state)
	}
	idleLog(t, "warm_release warm_to_enabled=true stops=1")
}

func TestPluginIdleSkipsRunningRuntime(t *testing.T) {
	ctx := context.Background()
	fixture := newIdleFixture(t)
	if _, err := fixture.installer.Execute(ctx, "plugin-0000", ExecuteRequest{ID: "req-1"}); err != nil {
		t.Fatalf("execute: %v", err)
	}
	fixture.warm(t)
	fixture.installer.mu.Lock()
	machine := fixture.installer.machines["plugin-0000"]
	fixture.installer.mu.Unlock()
	if _, err := machine.Apply(ctx, EventStart, machine.Epoch()+1); err != nil {
		t.Fatalf("start: %v", err)
	}
	fixture.advance(time.Minute + time.Second)
	report, err := fixture.reaper.Sweep(ctx)
	if err != nil {
		t.Fatalf("sweep: %v", err)
	}
	if len(report.Released) != 0 {
		t.Fatalf("running runtime was released: %v", report.Released)
	}
	if fixture.installer.Snapshot().Processes != 1 {
		t.Fatal("running runtime disappeared")
	}
	idleLog(t, "running skipped=true processes=1")
}

func TestPluginIdleReactivateAfterRelease(t *testing.T) {
	ctx := context.Background()
	fixture := newIdleFixture(t)
	if _, err := fixture.installer.Execute(ctx, "plugin-0000", ExecuteRequest{ID: "req-1"}); err != nil {
		t.Fatalf("execute: %v", err)
	}
	fixture.advance(time.Minute + time.Second)
	if _, err := fixture.reaper.Sweep(ctx); err != nil {
		t.Fatalf("sweep: %v", err)
	}
	if _, err := fixture.installer.Execute(ctx, "plugin-0000", ExecuteRequest{ID: "req-2"}); err != nil {
		t.Fatalf("re-execute: %v", err)
	}
	if fixture.spawner.Calls() != 2 {
		t.Fatalf("spawn calls=%d want 2 (cold start after release)", fixture.spawner.Calls())
	}
	if state, _ := fixture.installer.State("plugin-0000"); state != LifecycleActivated {
		t.Fatalf("state=%s want activated", state)
	}
	idleLog(t, "reactivate spawns=2 state=activated")
}

type failingDrainer struct {
	err   error
	calls int
}

func (d *failingDrainer) Drain(ctx context.Context, instanceID string, timeout time.Duration) error {
	d.calls++
	return d.err
}

func TestPluginIdleDrainFailureKeepsRuntime(t *testing.T) {
	ctx := context.Background()
	fixture := newIdleFixture(t)
	if _, err := fixture.installer.Execute(ctx, "plugin-0000", ExecuteRequest{ID: "req-1"}); err != nil {
		t.Fatalf("execute: %v", err)
	}
	failure := errors.New("drain blocked")
	drainer := &failingDrainer{err: failure}
	reaper, err := NewIdleReaperWithClock(fixture.installer, IdlePolicy{TTL: time.Second, DrainTimeout: time.Second}, drainer, func(string) (string, bool) {
		return fixture.instanceID, true
	}, func() time.Time { return *fixture.clock })
	if err != nil {
		t.Fatalf("NewIdleReaperWithClock: %v", err)
	}
	reaper.Observe("plugin-0000")
	fixture.advance(2 * time.Second)
	if _, err := reaper.Sweep(ctx); err == nil {
		t.Fatal("sweep succeeded despite drain failure")
	}
	if state, _ := fixture.installer.State("plugin-0000"); state != LifecycleActivated {
		t.Fatalf("state=%s want activated after failed drain", state)
	}
	if fixture.installer.Snapshot().Processes != 1 {
		t.Fatal("failed drain released the runtime")
	}
	idleLog(t, "drain_failure state=activated processes=1 released=0")
}

func TestPluginIdleRejectsInvalidPolicy(t *testing.T) {
	installer := NewInstaller(&executeSpawner{})
	if _, err := NewIdleReaper(installer, IdlePolicy{}, nil, nil); !errors.Is(err, ErrIdleInvalid) {
		t.Fatalf("zero ttl err=%v want ErrIdleInvalid", err)
	}
	if _, err := NewIdleReaper(nil, IdlePolicy{TTL: time.Second}, nil, nil); !errors.Is(err, ErrIdleInvalid) {
		t.Fatalf("nil installer err=%v want ErrIdleInvalid", err)
	}
	idleLog(t, "invalid ttl=0 rejected=ErrIdleInvalid")
}

func TestPluginIdleCancelLeavesRuntime(t *testing.T) {
	ctx := context.Background()
	fixture := newIdleFixture(t)
	if _, err := fixture.installer.Execute(ctx, "plugin-0000", ExecuteRequest{ID: "req-1"}); err != nil {
		t.Fatalf("execute: %v", err)
	}
	fixture.advance(time.Minute + time.Second)
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	report, err := fixture.reaper.Sweep(cancelled)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err=%v want context.Canceled", err)
	}
	if len(report.Released) != 0 || fixture.installer.Snapshot().Processes != 1 {
		t.Fatalf("cancelled sweep released a runtime: %+v", report)
	}
	idleLog(t, "cancel released=0 err=context.Canceled")
}

func TestPluginIdleSkipsUntrackedRuntime(t *testing.T) {
	ctx := context.Background()
	fixture := newIdleFixture(t)
	if err := fixture.installer.Activate(ctx, "plugin-0000", 2); err != nil {
		t.Fatalf("activate: %v", err)
	}
	fixture.advance(time.Minute + time.Second)
	report, err := fixture.reaper.Sweep(ctx)
	if err != nil {
		t.Fatalf("sweep: %v", err)
	}
	if len(report.Released) != 0 {
		t.Fatalf("untracked runtime was released: %v", report.Released)
	}
	idleLog(t, "untracked skipped=true released=0")
}
