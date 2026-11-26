//go:build rsi_lifecycle

package enablement

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"
)

type executeRuntime struct {
	mu      sync.Mutex
	id      string
	workers int
	invokes int
	fail    error
	block   chan struct{}
	closed  bool
}

func (r *executeRuntime) ID() string   { return r.id }
func (r *executeRuntime) Workers() int { return r.workers }

func (r *executeRuntime) Invoke(ctx context.Context, payload []byte) ([]byte, error) {
	r.mu.Lock()
	r.invokes++
	block := r.block
	fail := r.fail
	r.mu.Unlock()
	if block != nil {
		select {
		case <-block:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	if fail != nil {
		return nil, fail
	}
	return append([]byte("ok:"), payload...), nil
}

func (r *executeRuntime) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.closed = true
	return nil
}

func (r *executeRuntime) Invocations() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.invokes
}

func (r *executeRuntime) Closed() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.closed
}

type executeSpawner struct {
	mu     sync.Mutex
	calls  int
	fail   error
	block  bool
	last   *executeRuntime
	handle RuntimeHandle
}

func (s *executeSpawner) Spawn(ctx context.Context, metadata Metadata) (RuntimeHandle, error) {
	s.mu.Lock()
	s.calls++
	block := s.block
	fail := s.fail
	handle := s.handle
	s.mu.Unlock()
	if block {
		<-ctx.Done()
		return nil, ctx.Err()
	}
	if fail != nil {
		return nil, fail
	}
	if handle != nil {
		return handle, nil
	}
	runtime := &executeRuntime{id: metadata.ID, workers: 1}
	s.mu.Lock()
	s.last = runtime
	s.mu.Unlock()
	return runtime, nil
}

func (s *executeSpawner) Calls() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.calls
}

func (s *executeSpawner) Last() *executeRuntime {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.last
}

func executeLog(t *testing.T, format string, args ...any) {
	t.Helper()
	t.Logf("m06-s03 "+format, args...)
}

func executeInstaller(t *testing.T, spawner RuntimeSpawner, enabled bool) *Installer {
	t.Helper()
	installer := NewInstaller(spawner)
	if _, err := installer.InstallBatch(context.Background(), metadataManifests(1)); err != nil {
		t.Fatalf("install: %v", err)
	}
	if enabled {
		if err := installer.Enable(context.Background(), "plugin-0000", 1); err != nil {
			t.Fatalf("enable: %v", err)
		}
	}
	return installer
}

func TestPluginExecuteColdStartSpawnsOnce(t *testing.T) {
	ctx := context.Background()
	spawner := &executeSpawner{}
	installer := executeInstaller(t, spawner, true)
	first, err := installer.Execute(ctx, "plugin-0000", ExecuteRequest{ID: "req-1", Payload: []byte("a")})
	if err != nil {
		t.Fatalf("execute 1: %v", err)
	}
	if string(first.Output) != "ok:a" || first.Reused {
		t.Fatalf("first=%+v", first)
	}
	second, err := installer.Execute(ctx, "plugin-0000", ExecuteRequest{ID: "req-2", Payload: []byte("b")})
	if err != nil {
		t.Fatalf("execute 2: %v", err)
	}
	if string(second.Output) != "ok:b" {
		t.Fatalf("second output=%s", second.Output)
	}
	if spawner.Calls() != 1 || spawner.Last().Invocations() != 2 {
		t.Fatalf("spawns=%d invokes=%d", spawner.Calls(), spawner.Last().Invocations())
	}
	if state, _ := installer.State("plugin-0000"); state != LifecycleActivated {
		t.Fatalf("state=%s want activated", state)
	}
	executeLog(t, "cold spawns=1 invokes=2 state=activated")
}

func TestPluginExecuteInstalledDenied(t *testing.T) {
	spawner := &executeSpawner{}
	installer := executeInstaller(t, spawner, false)
	_, err := installer.Execute(context.Background(), "plugin-0000", ExecuteRequest{ID: "req-1"})
	if !errors.Is(err, ErrExecuteDenied) {
		t.Fatalf("err=%v want ErrExecuteDenied", err)
	}
	if spawner.Calls() != 0 || installer.Snapshot().Processes != 0 {
		t.Fatalf("denied execute spawned runtime: calls=%d", spawner.Calls())
	}
	executeLog(t, "installed denied=ErrExecuteDenied spawns=0")
}

func TestPluginExecuteActivationTimeoutRollsBack(t *testing.T) {
	spawner := &executeSpawner{block: true}
	installer := executeInstaller(t, spawner, true)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	_, err := installer.Execute(ctx, "plugin-0000", ExecuteRequest{ID: "req-1"})
	if !errors.Is(err, ErrActivationTimeout) {
		t.Fatalf("err=%v want ErrActivationTimeout", err)
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("error chain lost DeadlineExceeded: %v", err)
	}
	if state, _ := installer.State("plugin-0000"); state != LifecycleEnabled {
		t.Fatalf("state=%s want enabled after rollback", state)
	}
	if snapshot := installer.Snapshot(); snapshot.Processes != 0 || snapshot.Workers != 0 {
		t.Fatalf("rollback left runtime: %+v", snapshot)
	}
	executeLog(t, "timeout rollback=true state=enabled processes=0 cause=DeadlineExceeded")
}

func TestPluginExecuteSpawnErrorRollsBack(t *testing.T) {
	failure := errors.New("spawn offline")
	spawner := &executeSpawner{fail: failure}
	installer := executeInstaller(t, spawner, true)
	_, err := installer.Execute(context.Background(), "plugin-0000", ExecuteRequest{ID: "req-1"})
	if !errors.Is(err, ErrExecuteFailed) || !errors.Is(err, failure) {
		t.Fatalf("err=%v want ErrExecuteFailed wrapping %v", err, failure)
	}
	if state, _ := installer.State("plugin-0000"); state != LifecycleEnabled {
		t.Fatalf("state=%s want enabled", state)
	}
	if installer.Snapshot().Processes != 0 {
		t.Fatalf("spawn failure left a process")
	}
	executeLog(t, "spawn_error rollback=true chained=true state=enabled")
}

func TestPluginExecuteInvokeErrorRollsBackColdStart(t *testing.T) {
	failure := errors.New("runtime crashed")
	spawner := &executeSpawner{}
	installer := executeInstaller(t, spawner, true)
	runtime := &executeRuntime{id: "plugin-0000", workers: 1, fail: failure}
	spawner.handle = runtime
	_, err := installer.Execute(context.Background(), "plugin-0000", ExecuteRequest{ID: "req-1"})
	if !errors.Is(err, ErrExecuteFailed) || !errors.Is(err, failure) {
		t.Fatalf("err=%v want ErrExecuteFailed wrapping %v", err, failure)
	}
	if !runtime.Closed() {
		t.Fatal("failed cold start did not close the runtime")
	}
	if state, _ := installer.State("plugin-0000"); state != LifecycleEnabled {
		t.Fatalf("state=%s want enabled", state)
	}
	if installer.Snapshot().Processes != 0 {
		t.Fatalf("failed cold start left a process")
	}
	executeLog(t, "invoke_error rollback=true closed=true state=enabled")
}

func TestPluginExecuteDuplicateRequestIdempotent(t *testing.T) {
	ctx := context.Background()
	spawner := &executeSpawner{}
	installer := executeInstaller(t, spawner, true)
	request := ExecuteRequest{ID: "req-same", Payload: []byte("x")}
	first, err := installer.Execute(ctx, "plugin-0000", request)
	if err != nil {
		t.Fatalf("first: %v", err)
	}
	second, err := installer.Execute(ctx, "plugin-0000", request)
	if err != nil {
		t.Fatalf("second: %v", err)
	}
	if !second.Reused || string(second.Output) != string(first.Output) {
		t.Fatalf("second=%+v first=%+v", second, first)
	}
	if spawner.Last().Invocations() != 1 {
		t.Fatalf("duplicate request invoked runtime %d times", spawner.Last().Invocations())
	}
	executeLog(t, "duplicate reused=true invokes=1")
}

func TestPluginExecuteDuplicateInFlight(t *testing.T) {
	spawner := &executeSpawner{}
	block := make(chan struct{})
	runtime := &executeRuntime{id: "plugin-0000", workers: 1, block: block}
	spawner.handle = runtime
	installer := executeInstaller(t, spawner, true)
	done := make(chan error, 1)
	go func() {
		_, err := installer.Execute(context.Background(), "plugin-0000", ExecuteRequest{ID: "req-same"})
		done <- err
	}()
	deadline := time.Now().Add(2 * time.Second)
	for {
		if runtime.Invocations() > 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("first request never reached the runtime")
		}
		time.Sleep(time.Millisecond)
	}
	_, err := installer.Execute(context.Background(), "plugin-0000", ExecuteRequest{ID: "req-same"})
	if !errors.Is(err, ErrRequestInFlight) {
		t.Fatalf("err=%v want ErrRequestInFlight", err)
	}
	close(block)
	if err := <-done; err != nil {
		t.Fatalf("first request failed: %v", err)
	}
	executeLog(t, "in_flight rejected=ErrRequestInFlight invokes=1")
}

func TestPluginExecuteCanceledBeforeStart(t *testing.T) {
	spawner := &executeSpawner{}
	installer := executeInstaller(t, spawner, true)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := installer.Execute(ctx, "plugin-0000", ExecuteRequest{ID: "req-1"})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err=%v want context.Canceled", err)
	}
	if spawner.Calls() != 0 {
		t.Fatalf("cancelled execute spawned %d runtimes", spawner.Calls())
	}
	executeLog(t, "cancel spawns=0 err=context.Canceled")
}

func TestPluginExecuteInvalidRequest(t *testing.T) {
	spawner := &executeSpawner{}
	installer := executeInstaller(t, spawner, true)
	if _, err := installer.Execute(context.Background(), "plugin-0000", ExecuteRequest{}); !errors.Is(err, ErrExecuteInvalid) {
		t.Fatalf("empty id err=%v want ErrExecuteInvalid", err)
	}
	if _, err := installer.Execute(context.Background(), "missing", ExecuteRequest{ID: "req-1"}); !errors.Is(err, ErrInstallerState) {
		t.Fatalf("unknown plugin err=%v want ErrInstallerState", err)
	}
	if spawner.Calls() != 0 {
		t.Fatalf("invalid requests spawned %d runtimes", spawner.Calls())
	}
	executeLog(t, "invalid empty_id=ErrExecuteInvalid unknown=ErrInstallerState")
}

func TestPluginExecuteNotInvocableRollsBack(t *testing.T) {
	spawner := &executeSpawner{handle: mockRuntime{id: "plugin-0000", workers: 1}}
	installer := executeInstaller(t, spawner, true)
	_, err := installer.Execute(context.Background(), "plugin-0000", ExecuteRequest{ID: "req-1"})
	if !errors.Is(err, ErrRuntimeNotInvocable) {
		t.Fatalf("err=%v want ErrRuntimeNotInvocable", err)
	}
	if state, _ := installer.State("plugin-0000"); state != LifecycleEnabled {
		t.Fatalf("state=%s want enabled", state)
	}
	if installer.Snapshot().Processes != 0 {
		t.Fatalf("non-invocable runtime left a process")
	}
	executeLog(t, fmt.Sprintf("not_invocable rejected=ErrRuntimeNotInvocable state=%s", LifecycleEnabled))
}
