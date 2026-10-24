package task_runtime

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

func TestRound21MissingOwnerCursorCannotAuthorizeRetry(t *testing.T) {
	ctx, data, run, _ := round17OwnerFixture(t)
	port := AcknowledgedTaskHostPort{Data: newOwnerCASFixture(data)}
	if pending, err := port.InspectPendingNativeEffects(ctx, run); !IsTaskErrorCode(err, ErrTaskExecutionAttemptInvalid) || pending != nil {
		t.Fatalf("missing Owner cursor incorrectly treated as no pending effects: %#v %v", pending, err)
	}
}

func TestRound21OwnerTerminalCursorWithoutOutcomeCannotAuthorizeRetry(t *testing.T) {
	ctx, data, run, _ := round17OwnerFixture(t)
	store := newOwnerCASFixture(data)
	scope, err := taskInputScope(ctx, run)
	if err != nil {
		t.Fatal(err)
	}
	cursor := ownedTaskCommitCursor{
		Scope: scope, TaskRunID: run.TaskRunID,
		DefinitionFingerprint: run.DefinitionFingerprint,
		Generation:            run.Generation, AttemptID: run.ExecutionAttemptID.String(),
		Terminal: true, OutcomeID: taskOutcomeID(run),
	}
	body, err := json.Marshal(cursor)
	if err != nil {
		t.Fatal(err)
	}
	id := taskOwnerCommitCursorID(run)
	store.resources["checkpoint/"+id] = &coordination.Resource{
		OwnerID: scope.ResourceOwnerID, RoleID: scope.RoleID, Kind: "checkpoint",
		ID: id, Revision: 1, Body: body,
	}
	port := AcknowledgedTaskHostPort{Data: store}
	if pending, err := port.InspectPendingNativeEffects(ctx, run); !IsTaskErrorCode(err, ErrTaskExecutionAttemptInvalid) || pending != nil {
		t.Fatalf("terminal cursor without Owner outcome incorrectly allowed retry: %#v %v", pending, err)
	}
}

func TestRound21PendingNativeEffectIdentityCannotBeForged(t *testing.T) {
	ctx, data, run, def := round17OwnerFixture(t)
	store := newOwnerCASFixture(data)
	port := AcknowledgedTaskHostPort{Data: store, Executor: taskHostExecutorFunc(func(context.Context, *TaskRun, *TaskDefinition, string, string, TaskHostNativeCall) (json.RawMessage, error) {
		return nil, errors.New("effect result unknown")
	})}
	params := round17ToolRequest(run, "round21.original")
	if _, err := port.Call(ctx, run, def, "rpc", "task.host.executeTool", params); err == nil {
		t.Fatal("expected unresolved effect")
	}
	pending, err := port.InspectPendingNativeEffects(ctx, run)
	if err != nil || len(pending) != 1 {
		t.Fatalf("original started effect not recorded: %#v %v", pending, err)
	}
	resource := store.resources["tool-result/"+pending[0]]
	var document ownedTaskHostOperation
	if err := json.Unmarshal(resource.Body, &document); err != nil {
		t.Fatal(err)
	}

	document.EffectID = "round21.substituted"
	resource.Body, _ = json.Marshal(document)
	if _, err := port.InspectPendingNativeEffects(ctx, run); !IsTaskErrorCode(err, ErrTaskExecutionAttemptInvalid) {
		t.Fatalf("forged effect identity was accepted: %v", err)
	}
}

func TestRound21ConfirmedEffectsStillPermitEmptyPendingInspection(t *testing.T) {
	ctx, data, run, def := round17OwnerFixture(t)
	store := newOwnerCASFixture(data)
	port := AcknowledgedTaskHostPort{Data: store, Executor: taskHostExecutorFunc(func(context.Context, *TaskRun, *TaskDefinition, string, string, TaskHostNativeCall) (json.RawMessage, error) {
		return json.RawMessage(`{"result":{"ok":true}}`), nil
	})}
	if _, err := port.Call(ctx, run, def, "rpc", "task.host.executeTool", round17ToolRequest(run, "round21.confirmed")); err != nil {
		t.Fatal(err)
	}
	pending, err := port.InspectPendingNativeEffects(ctx, run)
	if err != nil || len(pending) != 0 {
		t.Fatalf("confirmed effect incorrectly blocked recovery: %#v %v", pending, err)
	}
}

func TestRound21MissingToolResultWithInFlightCursorCannotBeReexecuted(t *testing.T) {
	ctx, data, run, def := round17OwnerFixture(t)
	store := newOwnerCASFixture(data)
	port := AcknowledgedTaskHostPort{Data: store, Executor: taskHostExecutorFunc(func(context.Context, *TaskRun, *TaskDefinition, string, string, TaskHostNativeCall) (json.RawMessage, error) {
		t.Fatal("missing started record incorrectly caused external execution")
		return nil, nil
	})}
	scope, err := taskInputScope(ctx, run)
	if err != nil {
		t.Fatal(err)
	}
	params := round17ToolRequest(run, "round21.missing-record")
	id := TaskHostNativeOperationID(run, "rpc", TaskHostNativeCall{EffectID: "round21.missing-record"})
	cursor := ownedTaskCommitCursor{
		Scope: scope, TaskRunID: run.TaskRunID,
		DefinitionFingerprint: run.DefinitionFingerprint,
		Generation:            run.Generation, AttemptID: run.ExecutionAttemptID.String(),
		InFlightEffects: map[string]string{id: hashBytes(params)},
	}
	body, err := json.Marshal(cursor)
	if err != nil {
		t.Fatal(err)
	}
	cursorID := taskOwnerCommitCursorID(run)
	store.resources["checkpoint/"+cursorID] = &coordination.Resource{
		OwnerID: scope.ResourceOwnerID, RoleID: scope.RoleID,
		Kind: "checkpoint", ID: cursorID, Revision: 1, Body: body,
	}
	if inspection, err := port.Inspect(ctx, run, def, "rpc", "task.host.executeTool", params); !IsTaskErrorCode(err, ErrTaskExecutionAttemptInvalid) || inspection.State != "" {
		t.Fatalf("orphaned pending marker was reported as fresh effect: %#v %v", inspection, err)
	}
	if _, err := port.Call(ctx, run, def, "rpc", "task.host.executeTool", params); !IsTaskErrorCode(err, ErrTaskExecutionAttemptInvalid) {
		t.Fatalf("orphaned pending marker allowed execution: %v", err)
	}
}

func TestRound21NativeRecoveryInspectionMatchesRealPermissionDeclarations(t *testing.T) {
	for _, tc := range []struct {
		name     string
		grants   []string
		expected bool
	}{
		{"tool", []string{"service.tool.execute"}, true},
		{"event", []string{"event.emit"}, true},
		{"non-native", []string{"task.storage.get"}, false},
		{"empty", nil, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			actual, err := taskMayCallNativeEffects(&TaskDefinition{PermissionRequirementStrings: tc.grants})
			if err != nil || actual != tc.expected {
				t.Fatalf("native recovery permission classification incorrect: %v %v", actual, err)
			}
		})
	}
	if _, err := taskMayCallNativeEffects(&TaskDefinition{PermissionRequirementStrings: []string{" bad "}}); !IsTaskErrorCode(err, ErrTaskPermissionDenied) {
		t.Fatalf("invalid permission set did not fail closed: %v", err)
	}
}
