package runtimeorchestrator

import (
	"context"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/runtimehost"
)

func TestOptionalComponentRecoversFromRestartBackoffQA(t *testing.T) {
	orch := newTestOrchestrator()
	var clockMu sync.Mutex
	current := time.Now().UTC()
	orch.nowFn = func() time.Time {
		clockMu.Lock()
		defer clockMu.Unlock()
		return current
	}
	component := &fakeComponent{ComponentDescriptor: ComponentDescriptor{
		ID: "qa.vector", Phase: PhaseInfrastructure, Enabled: true, Required: false,
	}}
	component.startErr = errors.New("wait for qdrant ready: process amitia.qdrant is restart-backoff")
	if err := orch.Register(component); err != nil {
		t.Fatal(err)
	}
	if err := orch.StartPhase(t.Context(), PhaseInfrastructure); err != nil {
		t.Fatal(err)
	}

	failed := orch.Snapshot()
	if failed.Components[component.ID].State != StateDegraded {
		t.Fatalf("optional start failure state = %s", failed.Components[component.ID].State)
	}
	if !strings.Contains(failed.Components[component.ID].LastError, "restart-backoff") {
		t.Fatalf("optional start failure lost the original error: %q", failed.Components[component.ID].LastError)
	}

	component.startErr = nil
	clockMu.Lock()
	current = current.Add(3 * time.Second)
	clockMu.Unlock()

	orch.RefreshHealth(t.Context())
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if orch.Snapshot().Components[component.ID].State == StateReady {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	recovered := orch.Snapshot()
	if recovered.Components[component.ID].State != StateReady {
		t.Fatalf("component did not recover after backoff: %+v", recovered.Components[component.ID])
	}
	if recovered.State != OrchestratorReady {
		t.Fatalf("aggregate state after recovery = %s", recovered.State)
	}
	if got := atomic.LoadInt32(&component.startCalls); got < 2 {
		t.Fatalf("component was not restarted, startCalls = %d", got)
	}
}

func TestComponentRecoveryKeepsOriginalFailureAndRetriesLaterQA(t *testing.T) {
	orch := newTestOrchestrator()
	var clockMu sync.Mutex
	current := time.Now().UTC()
	orch.nowFn = func() time.Time {
		clockMu.Lock()
		defer clockMu.Unlock()
		return current
	}
	component := &fakeComponent{ComponentDescriptor: ComponentDescriptor{
		ID: "qa.optional-retry", Phase: PhaseInfrastructure, Enabled: true, Required: false,
	}}
	component.startErr = errors.New("build qdrant spec: qdrant executable is unavailable for darwin/arm64")
	if err := orch.Register(component); err != nil {
		t.Fatal(err)
	}
	if err := orch.StartPhase(t.Context(), PhaseInfrastructure); err != nil {
		t.Fatal(err)
	}

	recoveryFailure := errors.New("start qdrant process: exec format error")
	component.startErr = recoveryFailure
	clockMu.Lock()
	current = current.Add(3 * time.Second)
	clockMu.Unlock()
	orch.RefreshHealth(t.Context())
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if strings.Contains(orch.Snapshot().Components[component.ID].LastError, "exec format error") {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	snapshot := orch.Snapshot()
	if snapshot.Components[component.ID].State != StateDegraded {
		t.Fatalf("recovery failure state = %s", snapshot.Components[component.ID].State)
	}
	if !strings.Contains(snapshot.Components[component.ID].LastError, "exec format error") {
		t.Fatalf("recovery failure lost the original error: %q", snapshot.Components[component.ID].LastError)
	}

	component.startErr = nil
	clockMu.Lock()
	current = current.Add(componentRecoveryMaxDelay + time.Second)
	clockMu.Unlock()
	orch.RefreshHealth(t.Context())
	deadline = time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if orch.Snapshot().Components[component.ID].State == StateReady {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if state := orch.Snapshot().Components[component.ID].State; state != StateReady {
		t.Fatalf("component did not recover on the next retry: %s", state)
	}
}

func TestComponentRecoveryPanicKeepsStackAndDoesNotCrashQA(t *testing.T) {
	orch := newTestOrchestrator()
	var clockMu sync.Mutex
	current := time.Now().UTC()
	orch.nowFn = func() time.Time {
		clockMu.Lock()
		defer clockMu.Unlock()
		return current
	}
	component := &panickingComponent{ComponentDescriptor: ComponentDescriptor{
		ID: "qa.panic", Phase: PhaseInfrastructure, Enabled: true, Required: false,
	}}
	component.startErr = errors.New("initial start failure")
	if err := orch.Register(component); err != nil {
		t.Fatal(err)
	}
	if err := orch.StartPhase(t.Context(), PhaseInfrastructure); err != nil {
		t.Fatal(err)
	}

	component.panicOnStart = true
	clockMu.Lock()
	current = current.Add(3 * time.Second)
	clockMu.Unlock()
	orch.RefreshHealth(t.Context())
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if strings.Contains(orch.Snapshot().Components[component.ID].LastError, "component recovery panic") {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	snapshot := orch.Snapshot()
	if !strings.Contains(snapshot.Components[component.ID].LastError, "component recovery panic") {
		t.Fatalf("recovery panic lost the original panic: %q", snapshot.Components[component.ID].LastError)
	}
	if !strings.Contains(snapshot.Components[component.ID].LastError, "goroutine") {
		t.Fatalf("recovery panic lost the stack trace: %q", snapshot.Components[component.ID].LastError)
	}

	component.panicOnStart = false
	component.startErr = nil
	clockMu.Lock()
	current = current.Add(componentRecoveryMaxDelay + time.Second)
	clockMu.Unlock()
	orch.RefreshHealth(t.Context())
	deadline = time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if orch.Snapshot().Components[component.ID].State == StateReady {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if state := orch.Snapshot().Components[component.ID].State; state != StateReady {
		t.Fatalf("component after panic recovery = %s", state)
	}
}

type panickingComponent struct {
	ComponentDescriptor
	panicOnStart bool
	startErr     error
}

func (p *panickingComponent) Descriptor() ComponentDescriptor { return p.ComponentDescriptor }

func (p *panickingComponent) Start(ctx context.Context) error {
	if p.panicOnStart {
		panic("qa recovery panic sentinel")
	}
	return p.startErr
}

func (p *panickingComponent) Ready(ctx context.Context) error { return nil }

func (p *panickingComponent) Stop(ctx context.Context) error { return nil }

func TestComponentRecoveryIgnoresStopOnUnstartedProcessQA(t *testing.T) {
	orch := newTestOrchestrator()
	var clockMu sync.Mutex
	current := time.Now().UTC()
	orch.nowFn = func() time.Time {
		clockMu.Lock()
		defer clockMu.Unlock()
		return current
	}
	component := &fakeComponent{ComponentDescriptor: ComponentDescriptor{
		ID: "qa.graph-store", Phase: PhaseInfrastructure, Enabled: true, Required: false,
	}}
	component.stopErr = runtimehost.ErrProcessNotFound
	component.startErr = errors.New("build surrealdb spec: archive provided [surreal.exe carries pe executable header], want surreal with mach-o header for darwin/arm64")
	if err := orch.Register(component); err != nil {
		t.Fatal(err)
	}
	if err := orch.StartPhase(t.Context(), PhaseInfrastructure); err != nil {
		t.Fatal(err)
	}

	clockMu.Lock()
	current = current.Add(3 * time.Second)
	clockMu.Unlock()
	orch.RefreshHealth(t.Context())
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if atomic.LoadInt32(&component.stopCalls) > 0 && atomic.LoadInt32(&component.startCalls) > 1 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	state := orch.Snapshot().Components[component.ID]
	if state.State != StateDegraded {
		t.Fatalf("optional recovery failure state = %s", state.State)
	}
	if !strings.Contains(state.LastError, "carries pe executable header") || !strings.Contains(state.LastError, "want surreal with mach-o header") {
		t.Fatalf("recovery lost the original failure: %q", state.LastError)
	}
	if strings.Contains(state.LastError, "stop before recovery") {
		t.Fatalf("idempotent stop polluted the failure: %q", state.LastError)
	}
	if strings.Contains(state.LastError, "process not found") {
		t.Fatalf("idempotent stop leaked into the failure: %q", state.LastError)
	}
}

func TestComponentRecoveryKeepsDegradedStateWhileRetryingQA(t *testing.T) {
	orch := newTestOrchestrator()
	var clockMu sync.Mutex
	current := time.Now().UTC()
	orch.nowFn = func() time.Time {
		clockMu.Lock()
		defer clockMu.Unlock()
		return current
	}
	component := &fakeComponent{
		ComponentDescriptor: ComponentDescriptor{
			ID: "qa.retry-state", Phase: PhaseInfrastructure, Enabled: true, Required: false,
		},
		startDelay: 300 * time.Millisecond,
	}
	component.startErr = errors.New("wait for qdrant ready: process amitia.qdrant is restart-backoff")
	steady := &fakeComponent{ComponentDescriptor: ComponentDescriptor{
		ID: "qa.steady", Phase: PhaseInfrastructure, Enabled: true, Required: true,
	}}
	if err := orch.Register(component); err != nil {
		t.Fatal(err)
	}
	if err := orch.Register(steady); err != nil {
		t.Fatal(err)
	}
	if err := orch.StartPhase(t.Context(), PhaseInfrastructure); err != nil {
		t.Fatal(err)
	}

	component.startErr = nil
	clockMu.Lock()
	current = current.Add(3 * time.Second)
	clockMu.Unlock()
	orch.RefreshHealth(t.Context())

	snapshot := orch.Snapshot()
	if snapshot.Components[component.ID].State != StateDegraded {
		t.Fatalf("state during recovery attempt = %s", snapshot.Components[component.ID].State)
	}
	if snapshot.State == OrchestratorStarting {
		t.Fatalf("aggregate state flipped to starting while retrying a degraded component")
	}

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if orch.Snapshot().Components[component.ID].State == StateReady {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if state := orch.Snapshot().Components[component.ID].State; state != StateReady {
		t.Fatalf("state after recovery attempt = %s", state)
	}
}
