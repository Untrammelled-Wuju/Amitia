package dev_mode

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

type fakeCandidateRunner struct {
	mu          sync.Mutex
	drainCalls  []string
	stopCalls   []string
	startCalls  []string
	healthCalls []string
	drainErr    error
	stopErr     error
	startErr    error
	healthErr   error
	instanceID  string
}

func (f *fakeCandidateRunner) StartCandidate(_ context.Context, id WorkspaceID, _ *Revision) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.startCalls = append(f.startCalls, string(id))
	if f.startErr != nil {
		return "", f.startErr
	}
	if f.instanceID == "" {
		f.instanceID = "inst-" + string(id)
	}
	return f.instanceID, nil
}

func (f *fakeCandidateRunner) HealthCheck(_ context.Context, instanceID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.healthCalls = append(f.healthCalls, instanceID)
	return f.healthErr
}

func (f *fakeCandidateRunner) DrainInstance(_ context.Context, instanceID string, _ time.Duration) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.drainCalls = append(f.drainCalls, instanceID)
	return f.drainErr
}

func (f *fakeCandidateRunner) StopInstance(_ context.Context, instanceID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.stopCalls = append(f.stopCalls, instanceID)
	return f.stopErr
}

func (f *fakeCandidateRunner) getDrainCalls() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]string, len(f.drainCalls))
	copy(out, f.drainCalls)
	return out
}

func (f *fakeCandidateRunner) getStopCalls() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]string, len(f.stopCalls))
	copy(out, f.stopCalls)
	return out
}

func setupReloaderTest(t *testing.T) (*RuntimeReloader, *WorkspaceRegistry, WorkspaceID, string) {
	t.Helper()
	tmpDir := t.TempDir()

	manifestPath := filepath.Join(tmpDir, "manifest.json")
	manifestContent := `{"extension":{"id":"com.test/ext"},"version":"1.0.0"}`
	if err := os.WriteFile(manifestPath, []byte(manifestContent), 0o644); err != nil {
		t.Fatal(err)
	}

	registry := NewWorkspaceRegistry()
	ctx := context.Background()
	ws, err := registry.Register(ctx, RegisterWorkspaceInput{
		WorkspaceID:   "ws-1",
		ExtensionID:   "com.test/ext",
		PathReference: tmpDir,
		ManifestPath:  manifestPath,
	})
	if err != nil {
		t.Fatal(err)
	}
	_ = registry.GrantDevTrust(ws.WorkspaceID)

	pipeline := NewRebuildPipeline("node").WithRegistry(registry)
	preserver := NewStatePreserver()
	reloader := NewRuntimeReloader(registry, pipeline, preserver)
	reloader.Enable(ws.WorkspaceID)

	return reloader, registry, ws.WorkspaceID, tmpDir
}

func TestReloadStopsOldInstance(t *testing.T) {
	reloader, _, wsID, _ := setupReloaderTest(t)
	ctx := context.Background()

	runner1 := &fakeCandidateRunner{instanceID: "inst-candidate-1"}
	reloader.SetCandidateRunner(runner1)

	ev, err := reloader.Reload(ctx, wsID, "test", nil)
	if err != nil {
		t.Fatalf("Reload 1: %v", err)
	}
	if !ev.Success {
		t.Fatalf("Reload 1 failed: %s", ev.Error)
	}

	if len(runner1.getDrainCalls()) != 0 {
		t.Fatalf("expected 0 drain calls on first reload, got %d", len(runner1.getDrainCalls()))
	}
	if len(runner1.getStopCalls()) != 0 {
		t.Fatalf("expected 0 stop calls on first reload, got %d", len(runner1.getStopCalls()))
	}

	runner2 := &fakeCandidateRunner{instanceID: "inst-candidate-2"}
	reloader.SetCandidateRunner(runner2)

	ev2, err := reloader.Reload(ctx, wsID, "test2", nil)
	if err != nil {
		t.Fatalf("Reload 2: %v", err)
	}
	if !ev2.Success {
		t.Fatalf("Reload 2 failed: %s", ev2.Error)
	}

	drains := runner2.getDrainCalls()
	stops := runner2.getStopCalls()
	if len(drains) != 1 {
		t.Fatalf("expected 1 drain call on second reload, got %d", len(drains))
	}
	if len(stops) != 1 {
		t.Fatalf("expected 1 stop call on second reload, got %d", len(stops))
	}
	if drains[0] != "inst-candidate-1" {
		t.Fatalf("expected drain on inst-candidate-1, got %s", drains[0])
	}
	if stops[0] != "inst-candidate-1" {
		t.Fatalf("expected stop on inst-candidate-1, got %s", stops[0])
	}
}

func TestReloadStopFailureRecordsError(t *testing.T) {
	reloader, _, wsID, _ := setupReloaderTest(t)
	ctx := context.Background()

	runner1 := &fakeCandidateRunner{instanceID: "inst-candidate-1"}
	reloader.SetCandidateRunner(runner1)
	_, err := reloader.Reload(ctx, wsID, "test", nil)
	if err != nil {
		t.Fatalf("Reload 1: %v", err)
	}

	runner2 := &fakeCandidateRunner{
		instanceID: "inst-candidate-2",
		stopErr:    errors.New("stop failed"),
	}
	reloader.SetCandidateRunner(runner2)

	ev2, err := reloader.Reload(ctx, wsID, "test2", nil)
	if err != nil {
		t.Fatalf("Reload 2 should still succeed: %v", err)
	}
	if !ev2.Success {
		t.Fatalf("Reload 2 should succeed even if stop fails: %s", ev2.Error)
	}
	if ev2.Error == "" {
		t.Fatal("expected error message in event for stop failure")
	}
	if !ev2.CleanupFailed {
		t.Fatal("expected CleanupFailed to be true when stop fails")
	}
	if ev2.Status != ReloadSucceededWithCleanupFailure {
		t.Fatalf("expected status reload_succeeded_with_cleanup_failure, got %s", ev2.Status)
	}
}

func TestRecoverStaleInstances(t *testing.T) {
	reloader, _, wsID, _ := setupReloaderTest(t)
	ctx := context.Background()

	runner := &fakeCandidateRunner{instanceID: "inst-1"}
	reloader.SetCandidateRunner(runner)

	reloader.instanceMu.Lock()
	reloader.oldInstance[wsID] = "inst-stale-1"
	reloader.instanceMu.Unlock()

	cleaned := reloader.RecoverStaleInstances(ctx)
	if cleaned != 1 {
		t.Fatalf("expected 1 cleaned, got %d", cleaned)
	}

	stops := runner.getStopCalls()
	if len(stops) != 1 {
		t.Fatalf("expected 1 stop call, got %d", len(stops))
	}
	if stops[0] != "inst-stale-1" {
		t.Fatalf("expected stop on inst-stale-1, got %s", stops[0])
	}

	reloader.instanceMu.Lock()
	_, exists := reloader.oldInstance[wsID]
	reloader.instanceMu.Unlock()
	if exists {
		t.Fatal("expected oldInstance to be cleared after recovery")
	}
}

func TestRecoverStaleInstancesNoRunner(t *testing.T) {
	reloader, _, wsID, _ := setupReloaderTest(t)
	ctx := context.Background()

	reloader.instanceMu.Lock()
	reloader.oldInstance[wsID] = "inst-stale-1"
	reloader.instanceMu.Unlock()

	cleaned := reloader.RecoverStaleInstances(ctx)
	if cleaned != 0 {
		t.Fatalf("expected 0 cleaned without runner, got %d", cleaned)
	}

	reloader.instanceMu.Lock()
	_, exists := reloader.oldInstance[wsID]
	reloader.instanceMu.Unlock()
	if exists {
		t.Fatal("expected oldInstance to be cleared even without runner")
	}
}

func TestReloadDrainFailureStillStops(t *testing.T) {
	reloader, _, wsID, _ := setupReloaderTest(t)
	ctx := context.Background()

	runner1 := &fakeCandidateRunner{instanceID: "inst-candidate-1"}
	reloader.SetCandidateRunner(runner1)
	_, _ = reloader.Reload(ctx, wsID, "test", nil)

	runner2 := &fakeCandidateRunner{
		instanceID: "inst-candidate-2",
		drainErr:   errors.New("drain timeout"),
	}
	reloader.SetCandidateRunner(runner2)

	ev2, err := reloader.Reload(ctx, wsID, "test2", nil)
	if err != nil {
		t.Fatalf("Reload 2 should succeed: %v", err)
	}
	if !ev2.Success {
		t.Fatalf("Reload 2 should succeed: %s", ev2.Error)
	}

	stops := runner2.getStopCalls()
	if len(stops) != 1 {
		t.Fatalf("expected stop to be called even after drain failure, got %d calls", len(stops))
	}
}
