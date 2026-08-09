package runtime

import (
	"context"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/gamehost/domain"
)

type fakeCrashContext struct {
	stopping  bool
	terminal  bool
	shutdown  bool
}

func (f *fakeCrashContext) IsRuntimeStopping(runtimeID domain.RuntimeInstanceID) (bool, error) {
	return f.stopping, nil
}

func (f *fakeCrashContext) IsRuntimeTerminal(runtimeID domain.RuntimeInstanceID) (bool, error) {
	return f.terminal, nil
}

func (f *fakeCrashContext) IsRuntimeShutdown() bool {
	return f.shutdown
}

func TestCrashHandler_ExpectedExit(t *testing.T) {
	handler := NewCrashHandler(nil)

	ctx := context.Background()
	decision, err := handler.HandleProcessExit(ctx, ProcessExitEvent{
		RuntimeID:  "rt-1",
		ServiceID:  "bridge",
		Expected:   true,
		ExitCode:   0,
		OccurredAt: time.Now(),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !decision.UpdateHealth {
		t.Error("expected health update for expected exit")
	}
	if decision.Health != domain.HealthUnknown {
		t.Errorf("expected unknown health for expected exit, got %s", decision.Health)
	}
	if decision.Resolution != CrashDeferToSupervisor {
		t.Errorf("expected defer to supervisor, got %s", decision.Resolution)
	}
}

func TestCrashHandler_UnexpectedExit(t *testing.T) {
	handler := NewCrashHandler(nil)

	ctx := context.Background()
	decision, err := handler.HandleProcessExit(ctx, ProcessExitEvent{
		RuntimeID:  "rt-1",
		ServiceID:  "bridge",
		Expected:   false,
		ExitCode:   1,
		OccurredAt: time.Now(),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !decision.UpdateHealth {
		t.Error("expected health update for unexpected exit")
	}
	if decision.Health != domain.HealthUnhealthy {
		t.Errorf("expected unhealthy for unexpected exit, got %s", decision.Health)
	}
}

func TestCrashHandler_RuntimeStopping(t *testing.T) {
	ctx := &fakeCrashContext{stopping: true}
	handler := NewCrashHandler(ctx)

	bg := context.Background()
	decision, err := handler.HandleProcessExit(bg, ProcessExitEvent{
		RuntimeID:  "rt-1",
		ServiceID:  "bridge",
		Expected:   false,
		ExitCode:   -1,
		OccurredAt: time.Now(),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if decision.UpdateHealth {
		t.Error("should not update health when runtime is stopping")
	}
	if decision.Reason != "runtime_stopping" {
		t.Errorf("expected runtime_stopping reason, got %s", decision.Reason)
	}
}

func TestCrashHandler_RuntimeTerminal(t *testing.T) {
	ctx := &fakeCrashContext{terminal: true}
	handler := NewCrashHandler(ctx)

	bg := context.Background()
	decision, err := handler.HandleProcessExit(bg, ProcessExitEvent{
		RuntimeID:  "rt-1",
		ServiceID:  "bridge",
		Expected:   false,
		ExitCode:   -1,
		OccurredAt: time.Now(),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if decision.UpdateHealth {
		t.Error("should not update health when runtime is terminal")
	}
	if decision.Reason != "runtime_terminal" {
		t.Errorf("expected runtime_terminal reason, got %s", decision.Reason)
	}
}

func TestCrashHandler_BackendShutdown(t *testing.T) {
	ctx := &fakeCrashContext{shutdown: true}
	handler := NewCrashHandler(ctx)

	bg := context.Background()
	decision, err := handler.HandleProcessExit(bg, ProcessExitEvent{
		RuntimeID:  "rt-1",
		ServiceID:  "bridge",
		Expected:   false,
		ExitCode:   -1,
		OccurredAt: time.Now(),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if decision.UpdateHealth {
		t.Error("should not update health during backend shutdown")
	}
	if decision.Reason != "backend_shutdown" {
		t.Errorf("expected backend_shutdown reason, got %s", decision.Reason)
	}
}

func TestCrashRecorder_RecordAndGet(t *testing.T) {
	recorder := NewCrashRecorder()
	now := time.Now()

	recorder.RecordCrash("rt-1", "bridge", 1, "process_exited", now)

	record, ok := recorder.GetLastCrash("rt-1", "bridge")
	if !ok {
		t.Fatal("expected crash record")
	}
	if record.ExitCode != 1 {
		t.Errorf("expected exit code 1, got %d", record.ExitCode)
	}
	if record.Reason != "process_exited" {
		t.Errorf("expected reason preserved, got %s", record.Reason)
	}
}

func TestCrashRecorder_IsolationByRuntime(t *testing.T) {
	recorder := NewCrashRecorder()

	recorder.RecordCrash("rt-1", "bridge", 1, "crash", time.Now())

	if _, ok := recorder.GetLastCrash("rt-2", "bridge"); ok {
		t.Error("expected crash record to be per-runtime")
	}
}

func TestCrashRecorder_RemoveService(t *testing.T) {
	recorder := NewCrashRecorder()
	r := recorder.(*crashRecorder)

	recorder.RecordCrash("rt-1", "bridge", 1, "crash", time.Now())

	r.RemoveService("rt-1", "bridge")

	if _, ok := recorder.GetLastCrash("rt-1", "bridge"); ok {
		t.Error("expected service to be removed")
	}
}

func TestValidateProcessExit_Expected(t *testing.T) {
	decision := ValidateProcessExit(true, 0)
	if !decision.UpdateHealth {
		t.Error("expected health update")
	}
	if decision.Health != domain.HealthUnknown {
		t.Errorf("expected unknown health, got %s", decision.Health)
	}
}

func TestValidateProcessExit_Unexpected(t *testing.T) {
	decision := ValidateProcessExit(false, 1)
	if !decision.UpdateHealth {
		t.Error("expected health update")
	}
	if decision.Health != domain.HealthUnhealthy {
		t.Errorf("expected unhealthy, got %s", decision.Health)
	}
}

func TestValidateProcessExit_ExitCodeZeroStillUnexpected(t *testing.T) {
	decision := ValidateProcessExit(false, 0)
	if decision.Health != domain.HealthUnhealthy {
		t.Errorf("expected unhealthy for unexpected exit code 0, got %s", decision.Health)
	}
}

func TestCrashHandler_UpdateContext(t *testing.T) {
	handler := &crashHandler{context: nil}
	ctx := &fakeCrashContext{stopping: true}
	handler.UpdateContext(ctx)

	bg := context.Background()
	decision, err := handler.HandleProcessExit(bg, ProcessExitEvent{
		RuntimeID: "rt-1", ServiceID: "bridge", Expected: false,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if decision.UpdateHealth {
		t.Error("after UpdateContext, stopping runtime should not update health")
	}
}

func TestProcessExitEvent_Identity(t *testing.T) {
	event := ProcessExitEvent{
		RuntimeID:  "rt-1",
		ServiceID:  "bridge",
		InstanceID: "rt-1/bridge",
		Generation: 2,
	}

	identity := event.Identity()
	if identity.RuntimeID != "rt-1" {
		t.Errorf("expected RuntimeID rt-1, got %s", identity.RuntimeID)
	}
	if identity.ServiceID != "bridge" {
		t.Errorf("expected ServiceID bridge, got %s", identity.ServiceID)
	}
	if identity.InstanceID != "rt-1/bridge" {
		t.Errorf("expected InstanceID rt-1/bridge, got %s", identity.InstanceID)
	}
	if identity.Generation != 2 {
		t.Errorf("expected Generation 2, got %d", identity.Generation)
	}
}
