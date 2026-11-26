//go:build rsi_lifecycle

package enablement

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"sync"
	"testing"
	"time"
)

type leakSpawner struct {
	mu       sync.Mutex
	runtimes []*executeRuntime
	fail     error
	starts   int
	next     *executeRuntime
}

func (s *leakSpawner) Spawn(ctx context.Context, metadata Metadata) (RuntimeHandle, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.fail != nil {
		return nil, s.fail
	}
	created := s.next
	if created == nil {
		created = &executeRuntime{id: metadata.ID, workers: 1}
	}
	s.runtimes = append(s.runtimes, created)
	s.starts++
	return created, nil
}

func (s *leakSpawner) Starts() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.starts
}

func (s *leakSpawner) Stops() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	stops := 0
	for _, created := range s.runtimes {
		if created.Closed() {
			stops++
		}
	}
	return stops
}

func (s *leakSpawner) Last() *executeRuntime {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.runtimes) == 0 {
		return nil
	}
	return s.runtimes[len(s.runtimes)-1]
}

func releaseLog(t *testing.T, format string, args ...any) {
	t.Helper()
	t.Logf("m06-s06 "+format, args...)
}

func releaseInstaller(t *testing.T, spawner RuntimeSpawner, count int) *Installer {
	t.Helper()
	installer := NewInstaller(spawner)
	ids := make([]string, 0, count)
	for index := 0; index < count; index++ {
		ids = append(ids, fmt.Sprintf("plugin-%04d", index))
	}
	manifests := make([]Manifest, 0, count)
	for _, id := range ids {
		manifests = append(manifests, Manifest{ID: id, Version: "1.0.0"})
	}
	if _, err := installer.InstallBatch(context.Background(), manifests); err != nil {
		t.Fatalf("install: %v", err)
	}
	for _, id := range ids {
		if err := installer.Enable(context.Background(), id, 1); err != nil {
			t.Fatalf("enable %s: %v", id, err)
		}
	}
	return installer
}

func TestPluginReleaseDeactivateClosesOnce(t *testing.T) {
	ctx := context.Background()
	spawner := &leakSpawner{}
	installer := releaseInstaller(t, spawner, 1)
	if _, err := installer.Execute(ctx, "plugin-0000", ExecuteRequest{ID: "req-1"}); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if err := installer.Deactivate(ctx, "plugin-0000", 3); err != nil {
		t.Fatalf("deactivate: %v", err)
	}
	if state, _ := installer.State("plugin-0000"); state != LifecycleEnabled {
		t.Fatalf("state=%s want enabled", state)
	}
	if spawner.Starts() != 1 || spawner.Stops() != 1 {
		t.Fatalf("starts=%d stops=%d want 1/1", spawner.Starts(), spawner.Stops())
	}
	if snapshot := installer.Snapshot(); snapshot.Processes != 0 || snapshot.Workers != 0 {
		t.Fatalf("snapshot=%+v", snapshot)
	}
	releaseLog(t, "deactivate starts=1 stops=1 processes=0")
}

func TestPluginReleaseDeactivateWarm(t *testing.T) {
	ctx := context.Background()
	spawner := &leakSpawner{}
	installer := releaseInstaller(t, spawner, 1)
	if _, err := installer.Execute(ctx, "plugin-0000", ExecuteRequest{ID: "req-1"}); err != nil {
		t.Fatalf("execute: %v", err)
	}
	installer.mu.Lock()
	machine := installer.machines["plugin-0000"]
	installer.mu.Unlock()
	if _, err := machine.Apply(ctx, EventWarm, machine.Epoch()+1); err != nil {
		t.Fatalf("warm: %v", err)
	}
	if err := installer.Deactivate(ctx, "plugin-0000", machine.Epoch()+1); err != nil {
		t.Fatalf("deactivate warm: %v", err)
	}
	if state, _ := installer.State("plugin-0000"); state != LifecycleEnabled {
		t.Fatalf("state=%s want enabled", state)
	}
	if spawner.Stops() != 1 {
		t.Fatalf("stops=%d", spawner.Stops())
	}
	releaseLog(t, "deactivate_warm stops=1 state=enabled")
}

func TestPluginReleaseBusyRefused(t *testing.T) {
	ctx := context.Background()
	block := make(chan struct{})
	runtimeHandle := &executeRuntime{id: "plugin-0000", workers: 1, block: block}
	spawner := &leakSpawner{next: runtimeHandle}
	installer := releaseInstaller(t, spawner, 1)
	done := make(chan error, 1)
	go func() {
		_, err := installer.Execute(ctx, "plugin-0000", ExecuteRequest{ID: "req-busy"})
		done <- err
	}()
	deadline := time.Now().Add(2 * time.Second)
	for runtimeHandle.Invocations() == 0 {
		if time.Now().After(deadline) {
			t.Fatal("request never reached the runtime")
		}
		time.Sleep(time.Millisecond)
	}
	if err := installer.Deactivate(ctx, "plugin-0000", 5); !errors.Is(err, ErrPluginBusy) {
		t.Fatalf("deactivate during request err=%v want ErrPluginBusy", err)
	}
	if spawner.Stops() != 0 || installer.Snapshot().Processes != 1 {
		t.Fatal("busy deactivate released the runtime")
	}
	close(block)
	if err := <-done; err != nil {
		t.Fatalf("active request failed: %v", err)
	}
	releaseLog(t, "busy refused=ErrPluginBusy stops=0")
}

func TestPluginReleaseDoubleDeactivateRejected(t *testing.T) {
	ctx := context.Background()
	spawner := &leakSpawner{}
	installer := releaseInstaller(t, spawner, 1)
	if _, err := installer.Execute(ctx, "plugin-0000", ExecuteRequest{ID: "req-1"}); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if err := installer.Deactivate(ctx, "plugin-0000", 3); err != nil {
		t.Fatalf("first deactivate: %v", err)
	}
	if err := installer.Deactivate(ctx, "plugin-0000", 4); !errors.Is(err, ErrInstallerState) {
		t.Fatalf("second deactivate err=%v want ErrInstallerState", err)
	}
	if spawner.Stops() != 1 {
		t.Fatalf("double deactivate stopped %d runtimes", spawner.Stops())
	}
	releaseLog(t, "double_deactivate rejected=true stops=1")
}

func TestPluginReleaseSuspendResume(t *testing.T) {
	ctx := context.Background()
	spawner := &leakSpawner{}
	installer := releaseInstaller(t, spawner, 1)
	if _, err := installer.Execute(ctx, "plugin-0000", ExecuteRequest{ID: "req-1"}); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if err := installer.Suspend(ctx, "plugin-0000", 3); err != nil {
		t.Fatalf("suspend: %v", err)
	}
	if state, _ := installer.State("plugin-0000"); state != LifecycleSuspended {
		t.Fatalf("state=%s want suspended", state)
	}
	if spawner.Stops() != 1 || installer.Snapshot().Processes != 0 {
		t.Fatalf("suspend leaked a runtime")
	}
	if err := installer.Resume(ctx, "plugin-0000", 4); err != nil {
		t.Fatalf("resume: %v", err)
	}
	if state, _ := installer.State("plugin-0000"); state != LifecycleActivated {
		t.Fatalf("state=%s want activated", state)
	}
	if spawner.Starts() != 2 || installer.Snapshot().Processes != 1 {
		t.Fatalf("resume starts=%d processes=%d", spawner.Starts(), installer.Snapshot().Processes)
	}
	releaseLog(t, "suspend_resume stops=1 starts=2")
}

func TestPluginReleaseUninstallCloses(t *testing.T) {
	ctx := context.Background()
	spawner := &leakSpawner{}
	installer := releaseInstaller(t, spawner, 1)
	if _, err := installer.Execute(ctx, "plugin-0000", ExecuteRequest{ID: "req-1"}); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if err := installer.Deactivate(ctx, "plugin-0000", 3); err != nil {
		t.Fatalf("deactivate: %v", err)
	}
	if err := installer.Uninstall(ctx, "plugin-0000", 4); err != nil {
		t.Fatalf("uninstall: %v", err)
	}
	if installer.Snapshot().Plugins != 0 || spawner.Stops() != 1 {
		t.Fatalf("plugins=%d stops=%d", installer.Snapshot().Plugins, spawner.Stops())
	}
	releaseLog(t, "uninstall entries=0 stops=1")
}

func TestPluginReleaseThousandCyclesStable(t *testing.T) {
	ctx := context.Background()
	spawner := &leakSpawner{}
	installer := releaseInstaller(t, spawner, 1)
	baseline := runtime.NumGoroutine()
	for cycle := 0; cycle < 1000; cycle++ {
		requestID := fmt.Sprintf("cycle-%04d", cycle)
		if _, err := installer.Execute(ctx, "plugin-0000", ExecuteRequest{ID: requestID}); err != nil {
			t.Fatalf("cycle %d execute: %v", cycle, err)
		}
		if err := installer.Deactivate(ctx, "plugin-0000", uint64(2*cycle+3)); err != nil {
			t.Fatalf("cycle %d deactivate: %v", cycle, err)
		}
	}
	runtime.GC()
	after := runtime.NumGoroutine()
	if after > baseline+2 {
		t.Fatalf("goroutines grew from %d to %d", baseline, after)
	}
	if spawner.Starts() != 1000 || spawner.Stops() != 1000 {
		t.Fatalf("starts=%d stops=%d want 1000/1000", spawner.Starts(), spawner.Stops())
	}
	snapshot := installer.Snapshot()
	if snapshot.Processes != 0 || snapshot.Workers != 0 {
		t.Fatalf("snapshot=%+v", snapshot)
	}
	installer.mu.Lock()
	handles := len(installer.handles)
	results := len(installer.results)
	machine := installer.machines["plugin-0000"]
	installer.mu.Unlock()
	if handles != 0 || results != 0 {
		t.Fatalf("handles=%d results=%d want 0/0", handles, results)
	}
	if len(machine.History()) > 128 {
		t.Fatalf("history grew to %d", len(machine.History()))
	}
	releaseLog(t, "thousand_cycles starts=1000 stops=1000 handles=0 results=0 history<=128 goroutines_delta=%d", after-baseline)
}

func TestPluginReleaseFailedActivationNoHandle(t *testing.T) {
	ctx := context.Background()
	spawner := &leakSpawner{fail: errors.New("spawn offline")}
	installer := releaseInstaller(t, spawner, 1)
	if _, err := installer.Execute(ctx, "plugin-0000", ExecuteRequest{ID: "req-1"}); !errors.Is(err, ErrExecuteFailed) {
		t.Fatalf("err=%v want ErrExecuteFailed", err)
	}
	if len(installer.handles) != 0 || spawner.Stops() != 0 {
		t.Fatalf("failed activation leaked: handles=%d stops=%d", len(installer.handles), spawner.Stops())
	}
	spawner.mu.Lock()
	spawner.fail = nil
	spawner.mu.Unlock()
	if _, err := installer.Execute(ctx, "plugin-0000", ExecuteRequest{ID: "req-2"}); err != nil {
		t.Fatalf("retry: %v", err)
	}
	if err := installer.Deactivate(ctx, "plugin-0000", 5); err != nil {
		t.Fatalf("deactivate retry: %v", err)
	}
	if spawner.Starts() != 1 || spawner.Stops() != 1 {
		t.Fatalf("starts=%d stops=%d", spawner.Starts(), spawner.Stops())
	}
	releaseLog(t, "failed_activation no_handle=true retry_balanced=true")
}

func TestPluginReleaseCancelAndInvalid(t *testing.T) {
	ctx := context.Background()
	spawner := &leakSpawner{}
	installer := releaseInstaller(t, spawner, 1)
	if _, err := installer.Execute(ctx, "plugin-0000", ExecuteRequest{ID: "req-1"}); err != nil {
		t.Fatalf("execute: %v", err)
	}
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	if err := installer.Deactivate(cancelled, "plugin-0000", 3); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancel err=%v", err)
	}
	if spawner.Stops() != 0 {
		t.Fatal("cancelled deactivate stopped the runtime")
	}
	if err := installer.Deactivate(ctx, "missing", 3); !errors.Is(err, ErrInstallerState) {
		t.Fatalf("unknown plugin err=%v", err)
	}
	if err := installer.Suspend(ctx, "plugin-0000", 0); err == nil {
		t.Fatal("suspend with zero epoch succeeded")
	}
	releaseLog(t, "cancel stops=0 unknown=ErrInstallerState")
}

func TestPluginReleaseConcurrentExecuteDeactivate(t *testing.T) {
	ctx := context.Background()
	spawner := &leakSpawner{}
	installer := releaseInstaller(t, spawner, 1)
	var wg sync.WaitGroup
	for index := 0; index < 4; index++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			_, _ = installer.Execute(ctx, "plugin-0000", ExecuteRequest{ID: fmt.Sprintf("concurrent-%d", index)})
		}(index)
	}
	wg.Wait()
	if err := installer.Deactivate(ctx, "plugin-0000", 100); err != nil {
		t.Fatalf("deactivate: %v", err)
	}
	if spawner.Stops() != 1 || installer.Snapshot().Processes != 0 {
		t.Fatalf("stops=%d processes=%d", spawner.Stops(), installer.Snapshot().Processes)
	}
	if spawner.Starts() != 1 {
		t.Fatalf("concurrent executes spawned %d runtimes", spawner.Starts())
	}
	releaseLog(t, "concurrent starts=1 stops=1")
}
