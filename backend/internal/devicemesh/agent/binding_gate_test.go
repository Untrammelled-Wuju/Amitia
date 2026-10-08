package agent

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/u-ai/backend/internal/deviceruntime/protocol"
)

type pausingGateWorker struct {
	calls   int
	request protocol.TaskPausePayload
}

func (w *pausingGateWorker) ExecuteTask(context.Context, protocol.TaskDispatchPayload) error {
	return nil
}
func (w *pausingGateWorker) CancelTask(context.Context, string, string, string) error { return nil }
func (w *pausingGateWorker) PauseTask(_ context.Context, request protocol.TaskPausePayload) error {
	w.calls++
	w.request = request
	return nil
}

func TestBindingGateBlocksCandidateTaskPauseAndPreservesExecutionBinding(t *testing.T) {
	worker := &pausingGateWorker{}
	gate := &bindingGate{worker: worker}
	request := protocol.TaskPausePayload{TaskRunID: "run", AttemptID: "attempt", LeaseID: "lease", RuntimeSessionID: "session", ConnectionGeneration: 7}
	if err := gate.PauseTask(t.Context(), request); err == nil || worker.calls != 0 {
		t.Fatal("candidate provider paused an active task")
	}
	gate.active.Store(true)
	if err := gate.PauseTask(t.Context(), request); err != nil || worker.calls != 1 || worker.request != request {
		t.Fatalf("active pause lost the execution identity: %v", err)
	}
	gate.active.Store(false)
	if err := gate.PauseTask(t.Context(), request); err == nil || worker.calls != 1 {
		t.Fatal("retired provider reached the task worker")
	}
}

func TestCandidateGateAllowsOnlyRolePreflight(t *testing.T) {
	dispatcher := NewRuntimeDispatcher()
	calls := 0
	handler := func(invocation protocol.RuntimeInvokePayload) (*protocol.RuntimeResultPayload, error) {
		calls++
		return &protocol.RuntimeResultPayload{InvocationID: invocation.InvocationID}, nil
	}
	dispatcher.Register("coordination.data", handler)
	dispatcher.Register("browser.navigate", handler)
	gate := &bindingGate{dispatcher: dispatcher}
	for _, action := range []string{"roles", "apply", "interrupted", "snapshot"} {
		input, _ := json.Marshal(map[string]string{"operation": action})
		_, err := gate.Resolve("coordination.data")(protocol.RuntimeInvokePayload{Input: input})
		if (action == "roles") != (err == nil) {
			t.Fatalf("candidate gate %s: %v", action, err)
		}
	}
	if _, err := gate.Resolve("browser.navigate")(protocol.RuntimeInvokePayload{Input: json.RawMessage(`{"operation":"roles"}`)}); err == nil {
		t.Fatal("candidate executed device capability before cutover")
	}
	if calls != 1 {
		t.Fatalf("side effects before binding: %d", calls)
	}
	gate.active.Store(true)
	if _, err := gate.Resolve("browser.navigate")(protocol.RuntimeInvokePayload{}); err != nil {
		t.Fatal(err)
	}
}
