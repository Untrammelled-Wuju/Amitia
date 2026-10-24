package task_runtime

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

func TestRound22StableEffectRejectsFutureOrWrongAttemptReceipt(t *testing.T) {
	for _, tc := range []struct {
		name       string
		generation int64
		attempt    string
		allowed    bool
	}{
		{"future-generation", 2, "future-attempt", false},
		{"wrong-attempt-in-same-generation", 1, "different-attempt", false},
		{"original-attempt", 1, "round17-original", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, data, run, def := round17OwnerFixture(t)
			store := newOwnerCASFixture(data)
			scope, err := taskInputScope(ctx, run)
			if err != nil {
				t.Fatal(err)
			}
			params := round17ToolRequest(run, "round22.stable")
			result := json.RawMessage(`{"result":true}`)
			operation := ownedTaskHostOperation{
				Scope: scope, TaskRunID: run.TaskRunID,
				Generation: tc.generation, AttemptID: tc.attempt,
				DefinitionFingerprint: run.DefinitionFingerprint,
				RequestID:             "rpc", EffectID: "round22.stable",
				Method: "task.host.executeTool", InputHash: hashBytes(params),
				State: "confirmed", ResultHash: hashBytes(result), ResultBytes: result,
			}
			body, err := json.Marshal(operation)
			if err != nil {
				t.Fatal(err)
			}
			id := TaskHostNativeOperationID(run, "rpc", TaskHostNativeCall{EffectID: "round22.stable"})
			store.resources["tool-result/"+id] = &coordination.Resource{
				OwnerID: scope.ResourceOwnerID, RoleID: scope.RoleID,
				Kind: "tool-result", ID: id, Revision: 2, Body: body,
			}
			inspected, err := (AcknowledgedTaskHostPort{Data: store}).Inspect(ctx, run, def, "rpc", "task.host.executeTool", params)
			if tc.allowed {
				if err != nil || inspected.State != "confirmed" {
					t.Fatalf("original confirmed receipt must replay: %#v %v", inspected, err)
				}
			} else if !IsTaskErrorCode(err, ErrTaskExecutionAttemptInvalid) || inspected.State != "" {
				t.Fatalf("out-of-order receipt authorized replay: %#v %v", inspected, err)
			}
		})
	}
}

func TestRound22StableConfirmedEffectCanReplayIntoLaterGeneration(t *testing.T) {
	ctx, data, run, def := round17OwnerFixture(t)
	store := newOwnerCASFixture(data)
	port := AcknowledgedTaskHostPort{Data: store, Executor: taskHostExecutorFunc(func(_ context.Context, _ *TaskRun, _ *TaskDefinition, _, _ string, _ TaskHostNativeCall) (json.RawMessage, error) {
		return json.RawMessage(`{"result":true}`), nil
	})}
	params := round17ToolRequest(run, "round22.history")
	if _, err := port.Call(ctx, run, def, "rpc-old", "task.host.executeTool", params); err != nil {
		t.Fatal(err)
	}
	run.Generation++
	run.ExecutionAttemptID = "second-attempt"
	inspected, err := port.Inspect(ctx, run, def, "rpc-new", "task.host.executeTool", params)
	if err != nil || inspected.State != "confirmed" {
		t.Fatalf("valid old effect could not be reused: %#v %v", inspected, err)
	}
}
