package executionjournal_test

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/devicemesh/executionjournal"
	"github.com/u-ai/backend/internal/deviceruntime/protocol"
)

func coordinatedScope() coordination.ExecutionScope {
	return coordination.ExecutionScope{CoreID: "core-b", SpaceID: "core-b", ResourceOwnerID: "core-b", RoleOwnerID: "core-b", TargetDeviceID: "device-a", Coordinated: true}
}

func TestTaskInspectionReadOnlyAndOwnerScoped(t *testing.T) {
	db, store := harness(t)
	binding := executionjournal.TaskExecutionBinding{AttemptID: "invoke", LeaseID: "lease-1", RuntimeID: "runtime", SessionID: "session-1", ConnectionGeneration: 2, TaskDefinitionID: "task.action"}
	req := executionjournal.TaskInspectionRequest{TaskRunID: "task-1", TaskGeneration: 3, TaskExecutionBinding: binding}
	if got, err := store.InspectTask(t.Context(), "core-b", "device-a", req); err != nil || got.State != "absent" {
		t.Fatalf("absent: %+v %v", got, err)
	}
	invoke := invocation()
	invoke.IdempotencyKey = "task/task-1/3"
	invoke.RuntimeType = "task"
	invoke.RuntimeID = "runtime"
	invoke.RuntimeSessionID = "session-1"
	invoke.ConnectionGeneration = 2
	invoke.Handler = "task.action"
	scope := coordinatedScope()
	invoke.OwnedExecutionScope, _ = json.Marshal(scope)
	if _, err := store.ExecuteTask(coordination.WithScope(t.Context(), scope), invoke, binding, func() (*protocol.RuntimeResultPayload, error) {
		return &protocol.RuntimeResultPayload{Status: "completed", Result: json.RawMessage(`{"private":"owner"}`)}, nil
	}); err != nil {
		t.Fatal(err)
	}
	restarted := executionjournal.NewStore(db)
	got, err := restarted.InspectTask(t.Context(), "core-b", "device-a", req)
	if err != nil || got.State != "performed" || got.Result != nil {
		t.Fatalf("private owned result disclosed: %+v %v", got, err)
	}
	if _, err := restarted.InspectTask(t.Context(), "core-other", "device-a", req); !errors.Is(err, executionjournal.ErrConflict) {
		t.Fatalf("wrong Core inspected task: %v", err)
	}
	if got, err := restarted.InspectTask(t.Context(), "core-b", "device-other", req); err != nil || got.State != "absent" {
		t.Fatalf("wrong device: %+v %v", got, err)
	}
	if got, err := restarted.InspectTask(t.Context(), "core-b", "device-a", executionjournal.TaskInspectionRequest{TaskRunID: "task-1", TaskGeneration: 2, TaskExecutionBinding: binding}); err != nil || got.State != "absent" {
		t.Fatalf("wrong generation: %+v %v", got, err)
	}
	for _, invalid := range []executionjournal.TaskInspectionRequest{
		{TaskRunID: "task-1", TaskGeneration: 0, TaskExecutionBinding: binding},
		{TaskRunID: "../task-1", TaskGeneration: 3, TaskExecutionBinding: binding},
		{TaskRunID: "task/1", TaskGeneration: 3, TaskExecutionBinding: binding},
		{TaskRunID: "task-1\n", TaskGeneration: 3, TaskExecutionBinding: binding},
	} {
		if _, err := restarted.InspectTask(t.Context(), "core-b", "device-a", invalid); !errors.Is(err, executionjournal.ErrConflict) {
			t.Fatalf("accepted invalid task identity: %+v %v", invalid, err)
		}
	}
	for _, variant := range []struct {
		name   string
		adjust func(*executionjournal.TaskExecutionBinding)
	}{
		{"attempt", func(x *executionjournal.TaskExecutionBinding) { x.AttemptID = "later" }},
		{"lease", func(x *executionjournal.TaskExecutionBinding) { x.LeaseID = "later" }},
		{"session", func(x *executionjournal.TaskExecutionBinding) { x.SessionID = "later" }},
		{"connection-generation", func(x *executionjournal.TaskExecutionBinding) { x.ConnectionGeneration++ }},
		{"runtime", func(x *executionjournal.TaskExecutionBinding) { x.RuntimeID = "other" }},
		{"task-definition", func(x *executionjournal.TaskExecutionBinding) { x.TaskDefinitionID = "other" }},
	} {
		t.Run(variant.name, func(t *testing.T) {
			wrong := req
			variant.adjust(&wrong.TaskExecutionBinding)
			if _, err := restarted.InspectTask(t.Context(), "core-b", "device-a", wrong); !errors.Is(err, executionjournal.ErrUncertain) {
				t.Fatalf("accepted wrong worker identity: %v", err)
			}
		})
	}
}

func TestTaskInspectionDoesNotTurnUnknownIntoSuccess(t *testing.T) {
	db, store := harness(t)
	inv := invocation()
	inv.IdempotencyKey = "task/task-unknown/4"
	inv.RuntimeType = "task"
	inv.RuntimeID = "runtime"
	inv.RuntimeSessionID = "session-1"
	inv.ConnectionGeneration = 2
	inv.Handler = "task.action"
	binding := executionjournal.TaskExecutionBinding{AttemptID: "invoke", LeaseID: "lease-1", RuntimeID: "runtime", SessionID: "session-1", ConnectionGeneration: 2, TaskDefinitionID: "task.action"}
	scope := coordinatedScope()
	inv.OwnedExecutionScope, _ = json.Marshal(scope)
	if _, err := store.ExecuteTask(coordination.WithScope(t.Context(), scope), inv, binding, func() (*protocol.RuntimeResultPayload, error) { return nil, errors.New("disconnected") }); err == nil {
		t.Fatal("expected unknown")
	}
	result, err := executionjournal.NewStore(db).InspectTask(t.Context(), "core-b", "device-a", executionjournal.TaskInspectionRequest{TaskRunID: "task-unknown", TaskGeneration: 4, TaskExecutionBinding: binding})
	if err != nil || result.State != "started" || result.Result != nil {
		t.Fatalf("uncertain work upgraded: %+v %v", result, err)
	}
}

func TestLegacyTaskRecordCannotBeUpgradedToAttemptEvidence(t *testing.T) {
	db, store := harness(t)
	inv := invocation()
	inv.IdempotencyKey = "task/legacy/2"
	if _, err := store.Execute(t.Context(), inv, func() (*protocol.RuntimeResultPayload, error) { return nil, errors.New("unknown") }); err == nil {
		t.Fatal("expected uncertain")
	}
	binding := executionjournal.TaskExecutionBinding{AttemptID: "invoke", LeaseID: "lease", RuntimeID: "runtime", SessionID: "session", ConnectionGeneration: 1, TaskDefinitionID: "task.action"}
	req := executionjournal.TaskInspectionRequest{TaskRunID: "legacy", TaskGeneration: 2, TaskExecutionBinding: binding}
	if _, err := executionjournal.NewStore(db).InspectTask(t.Context(), "core-b", "device-a", req); !errors.Is(err, executionjournal.ErrUncertain) {
		t.Fatalf("legacy row trusted: %v", err)
	}
}

func TestTaskBindingCannotBeRewrittenByNewWorkerAttempt(t *testing.T) {
	db, store := harness(t)
	invoke := invocation()
	invoke.IdempotencyKey = "task/replay/8"
	invoke.RuntimeType = "task"
	invoke.Handler = "task.action"
	invoke.RuntimeID = "runtime"
	invoke.RuntimeSessionID = "session-old"
	invoke.ConnectionGeneration = 9
	scope := coordinatedScope()
	invoke.OwnedExecutionScope, _ = json.Marshal(scope)
	binding := executionjournal.TaskExecutionBinding{AttemptID: "invoke", LeaseID: "lease-old", RuntimeID: "runtime", SessionID: "session-old", ConnectionGeneration: 9, TaskDefinitionID: "task.action"}
	calls := 0
	work := func() (*protocol.RuntimeResultPayload, error) {
		calls++
		return &protocol.RuntimeResultPayload{Status: "completed", Result: json.RawMessage(`{"private":"result"}`)}, nil
	}
	ctx := coordination.WithScope(t.Context(), scope)
	if _, err := store.ExecuteTask(ctx, invoke, binding, work); err != nil {
		t.Fatal(err)
	}
	changed := binding
	changed.LeaseID = "lease-new"
	if _, err := executionjournal.NewStore(db).ExecuteTask(ctx, invoke, changed, work); !errors.Is(err, executionjournal.ErrConflict) {
		t.Fatalf("lease overwrite accepted: %v", err)
	}
	if _, err := executionjournal.NewStore(db).ExecuteTask(ctx, invoke, binding, work); !errors.Is(err, executionjournal.ErrUncertain) {
		t.Fatalf("performed result reused: %v", err)
	}
	if calls != 1 {
		t.Fatalf("external action repeated %d times", calls)
	}
}

func TestDeviceOwnedTaskKeepsWorkerBindingAndRefusesReplay(t *testing.T) {
	db, store := harness(t)
	invoke := invocation()
	invoke.IdempotencyKey = "task/device-owned/5"
	invoke.RuntimeType = "task"
	invoke.Handler = "task.action"
	invoke.RuntimeID = "runtime"
	invoke.RuntimeSessionID = "session-1"
	invoke.ConnectionGeneration = 3
	binding := executionjournal.TaskExecutionBinding{AttemptID: "invoke", LeaseID: "lease-1", RuntimeID: "runtime", SessionID: "session-1", ConnectionGeneration: 3, TaskDefinitionID: "task.action"}
	scope := coordination.ExecutionScope{CoreID: "core-b", SpaceID: "core-b", ResourceOwnerID: "device-a", RoleOwnerID: "device-a", TargetDeviceID: "device-a"}
	invoke.OwnedExecutionScope, _ = json.Marshal(scope)
	calls := 0
	work := func() (*protocol.RuntimeResultPayload, error) {
		calls++
		return &protocol.RuntimeResultPayload{Status: "completed", Result: json.RawMessage(`{"private":"device"}`)}, nil
	}
	ctx := coordination.WithScope(t.Context(), scope)
	if _, err := store.ExecuteTask(ctx, invoke, binding, work); err != nil {
		t.Fatal(err)
	}
	req := executionjournal.TaskInspectionRequest{TaskRunID: "device-owned", TaskGeneration: 5, TaskExecutionBinding: binding}
	inspected, err := executionjournal.NewStore(db).InspectTask(t.Context(), "core-b", "device-a", req)
	if err != nil || inspected.State != "performed" || inspected.Result != nil {
		t.Fatalf("device owned task evidence: %+v %v", inspected, err)
	}
	changed := binding
	changed.LeaseID = "lease-2"
	if _, err := executionjournal.NewStore(db).ExecuteTask(ctx, invoke, changed, work); !errors.Is(err, executionjournal.ErrConflict) {
		t.Fatalf("new worker rewrote binding: %v", err)
	}
	if _, err := executionjournal.NewStore(db).ExecuteTask(ctx, invoke, binding, work); !errors.Is(err, executionjournal.ErrUncertain) {
		t.Fatalf("performed task replayed: %v", err)
	}
	if calls != 1 {
		t.Fatalf("device task executed %d times", calls)
	}
	if err := executionjournal.NewStore(db).RecordWorkerStop(t.Context(), "core-b", "device-a", req); err != nil {
		t.Fatalf("stop receipt rejected: %v", err)
	}
	stopped, err := executionjournal.NewStore(db).InspectTask(t.Context(), "core-b", "device-a", req)
	if err != nil || stopped.State != "stopped_performed" || stopped.Result != nil {
		t.Fatalf("stop receipt lost: %+v %v", stopped, err)
	}
}

func TestTaskStopReceiptSurvivesRestartAndRetainsOwnerPrivacy(t *testing.T) {
	for _, scenario := range []struct {
		name      string
		taskFails bool
		want      string
	}{
		{"returned", false, "stopped_performed"},
		{"unknown", true, "stopped_unknown"},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			db, store := harness(t)
			invoke := invocation()
			invoke.IdempotencyKey = "task/task-stop/3"
			invoke.RuntimeType = "task"
			invoke.RuntimeID = "runtime"
			invoke.RuntimeSessionID = "session-1"
			invoke.ConnectionGeneration = 4
			invoke.Handler = "task.action"
			binding := executionjournal.TaskExecutionBinding{AttemptID: "invoke", LeaseID: "lease", RuntimeID: "runtime", SessionID: "session-1", ConnectionGeneration: 4, TaskDefinitionID: "task.action"}
			req := executionjournal.TaskInspectionRequest{TaskRunID: "task-stop", TaskGeneration: 3, TaskExecutionBinding: binding}
			scope := coordinatedScope()
			invoke.OwnedExecutionScope, _ = json.Marshal(scope)
			calls := 0
			work := func() (*protocol.RuntimeResultPayload, error) {
				calls++
				if scenario.taskFails {
					return nil, errors.New("disconnected after possible side effects")
				}
				return &protocol.RuntimeResultPayload{Status: "completed", Result: json.RawMessage(`{"private":"never-disclose"}`)}, nil
			}
			_, runErr := store.ExecuteTask(coordination.WithScope(t.Context(), scope), invoke, binding, work)
			if (runErr != nil) != scenario.taskFails {
				t.Fatalf("unexpected run error: %v", runErr)
			}

			before, err := store.InspectTask(t.Context(), "core-b", "device-a", req)
			if err != nil || before.State == scenario.want {
				t.Fatalf("stop manufactured: %+v %v", before, err)
			}
			if err := store.RecordWorkerStop(t.Context(), "core-b", "device-a", req); err != nil {
				t.Fatal(err)
			}
			restarted := executionjournal.NewStore(db)
			got, err := restarted.InspectTask(t.Context(), "core-b", "device-a", req)
			if err != nil || got.State != scenario.want || got.Result != nil {
				t.Fatalf("stop evidence: %+v %v", got, err)
			}
			if err := restarted.RecordWorkerStop(t.Context(), "core-b", "device-a", req); err != nil {
				t.Fatalf("idempotent stop: %v", err)
			}
			if _, err := restarted.ExecuteTask(coordination.WithScope(t.Context(), scope), invoke, binding, work); !errors.Is(err, executionjournal.ErrUncertain) || calls != 1 {
				t.Fatalf("stop replayed action: calls=%d err=%v", calls, err)
			}
			wrong := req
			wrong.LeaseID = "another-lease"
			if err := restarted.RecordWorkerStop(t.Context(), "core-b", "device-a", wrong); !errors.Is(err, executionjournal.ErrConflict) {
				t.Fatalf("foreign lease stopped worker: %v", err)
			}
			wrong = req
			wrong.TaskGeneration++
			if err := restarted.RecordWorkerStop(t.Context(), "core-b", "device-a", wrong); !errors.Is(err, executionjournal.ErrUncertain) {
				t.Fatalf("missing generation stopped worker: %v", err)
			}
			if err := restarted.RecordWorkerStop(t.Context(), "other-core", "device-a", req); !errors.Is(err, executionjournal.ErrConflict) {
				t.Fatalf("foreign core stopped worker: %v", err)
			}
		})
	}
}

func TestTaskStopCannotUpgradeLegacyOrInventExecution(t *testing.T) {
	db, store := harness(t)
	req := executionjournal.TaskInspectionRequest{TaskRunID: "unseen", TaskGeneration: 1, TaskExecutionBinding: executionjournal.TaskExecutionBinding{
		AttemptID: "invoke", LeaseID: "lease", RuntimeID: "runtime", SessionID: "session", ConnectionGeneration: 1, TaskDefinitionID: "task.action",
	}}
	if err := store.RecordWorkerStop(t.Context(), "core-b", "device-a", req); !errors.Is(err, executionjournal.ErrUncertain) {
		t.Fatalf("no row upgraded: %v", err)
	}
	inv := invocation()
	inv.IdempotencyKey = "task/unseen/1"
	if _, err := store.Execute(t.Context(), inv, func() (*protocol.RuntimeResultPayload, error) { return nil, errors.New("unknown") }); err == nil {
		t.Fatal("expected started legacy record")
	}
	if err := executionjournal.NewStore(db).RecordWorkerStop(t.Context(), "core-b", "device-a", req); !errors.Is(err, executionjournal.ErrConflict) {
		t.Fatalf("legacy upgraded: %v", err)
	}
	if err := store.RecordWorkerStop(t.Context(), "core-b", "device-a", executionjournal.TaskInspectionRequest{TaskRunID: "../malformed", TaskGeneration: 1, TaskExecutionBinding: req.TaskExecutionBinding}); !errors.Is(err, executionjournal.ErrConflict) {
		t.Fatalf("malformed identity accepted: %v", err)
	}
}
