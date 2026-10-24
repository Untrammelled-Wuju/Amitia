package task_runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"testing"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

func round19Fixture(t *testing.T) (context.Context, *ownerCASFixture, *TaskRun, *TaskDefinition) {
	t.Helper()
	ctx, original, run, def := round17OwnerFixture(t)
	store := newOwnerCASFixture(original)
	store.bothRead = nil
	return ctx, store, run, def
}

func TestRound19NativeStartedPreventsTerminalAndCheckpoint(t *testing.T) {
	ctx, store, run, def := round19Fixture(t)
	started, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	port := AcknowledgedTaskHostPort{Data: store, Executor: taskHostExecutorFunc(func(context.Context, *TaskRun, *TaskDefinition, string, string, TaskHostNativeCall) (json.RawMessage, error) {
		once.Do(func() { close(started) })
		<-release
		return json.RawMessage(`{"result":{"ok":true}}`), nil
	})}
	finished := make(chan error, 1)
	go func() {
		_, err := port.Call(ctx, run, def, "rpc-1", "task.host.executeTool", round17ToolRequest(run, "round19.effect"))
		finished <- err
	}()
	<-started
	if err := (AcknowledgedTaskOutcomePort{Data: store}).SaveOutcome(ctx, run, "failed", nil, "failed", "private"); !IsTaskErrorCode(err, ErrTaskExecutionAttemptInvalid) {
		t.Fatalf("terminal incorrectly passed a pending Native effect: %v", err)
	}
	checkpoint := round18Checkpoint(run, 1)
	if err := (AcknowledgedTaskCheckpointPort{Data: store}).SaveCheckpoint(ctx, run, checkpoint); !IsTaskErrorCode(err, ErrTaskExecutionAttemptInvalid) {
		t.Fatalf("checkpoint incorrectly passed a pending Native effect: %v", err)
	}
	if store.resources["checkpoint/"+taskOutcomeID(run)] != nil || store.resources["checkpoint/task/checkpoint/"+checkpoint.CheckpointID] != nil {
		t.Fatal("terminal or checkpoint unexpectedly persisted while tool was in flight")
	}
	close(release)
	if err := <-finished; err != nil {
		t.Fatalf("Native confirmation failed: %v", err)
	}
	if err := (AcknowledgedTaskOutcomePort{Data: store}).SaveOutcome(ctx, run, "failed", nil, "failed", "private"); err != nil {
		t.Fatalf("terminal could not commit after Native confirmation: %v", err)
	}
	var cursor ownedTaskCommitCursor
	resource := store.resources["checkpoint/"+taskOwnerCommitCursorID(run)]
	if resource == nil || resource.Revision != 3 || json.Unmarshal(resource.Body, &cursor) != nil || !cursor.Terminal || len(cursor.InFlightEffects) != 0 {
		t.Fatal("Owner cursor lost Native started, confirmed, or terminal sequence")
	}
}

func TestRound19NativeStartAndTerminalRaceIsSerialized(t *testing.T) {
	ctx, original, run, def := round17OwnerFixture(t)
	store := newOwnerCASFixture(original)
	store.bothRead = make(chan struct{})
	port := AcknowledgedTaskHostPort{Data: store, Executor: taskHostExecutorFunc(func(context.Context, *TaskRun, *TaskDefinition, string, string, TaskHostNativeCall) (json.RawMessage, error) {
		return json.RawMessage(`{"result":true}`), nil
	})}
	errors := make(chan error, 2)
	go func() {
		_, err := port.Call(ctx, run, def, "rpc-1", "task.host.executeTool", round17ToolRequest(run, "round19.race"))
		errors <- err
	}()
	go func() {
		errors <- (AcknowledgedTaskOutcomePort{Data: store}).SaveOutcome(ctx, run, "failed", nil, "failed", "private")
	}()
	first, second := <-errors, <-errors
	if (first == nil) == (second == nil) {
		t.Fatalf("racing terminal and Native start both won or both failed: %v; %v", first, second)
	}
	operationID := TaskHostNativeOperationID(run, "rpc-1", TaskHostNativeCall{EffectID: "round19.race"})
	operation := store.resources["tool-result/"+operationID]
	terminal := store.resources["checkpoint/"+taskOutcomeID(run)]
	if (operation != nil) == (terminal != nil) {
		t.Fatalf("CAS allowed incompatible tool-result and terminal: operation=%v terminal=%v", operation != nil, terminal != nil)
	}
}

func TestRound19NativeErrorLeavesOwnerEffectUnresolved(t *testing.T) {
	ctx, store, run, def := round19Fixture(t)
	port := AcknowledgedTaskHostPort{Data: store, Executor: taskHostExecutorFunc(func(context.Context, *TaskRun, *TaskDefinition, string, string, TaskHostNativeCall) (json.RawMessage, error) {
		return nil, fmt.Errorf("device execution outcome unknown")
	})}
	request := round17ToolRequest(run, "round19.unknown")
	if _, err := port.Call(ctx, run, def, "rpc-1", "task.host.executeTool", request); err == nil {
		t.Fatal("Native execution failed without reporting error")
	}
	if err := (AcknowledgedTaskOutcomePort{Data: store}).SaveOutcome(ctx, run, "failed", nil, "failed", "private"); !IsTaskErrorCode(err, ErrTaskExecutionAttemptInvalid) {
		t.Fatalf("unknown tool result incorrectly allowed terminal: %v", err)
	}
	if _, err := port.Call(ctx, run, def, "rpc-1", "task.host.executeTool", request); !IsTaskErrorCode(err, ErrTaskExecutionAttemptInvalid) {
		t.Fatalf("unconfirmed tool effect was retried: %v", err)
	}
}

func TestRound19OwnerCursorRejectsForgedPendingState(t *testing.T) {
	ctx, store, run, def := round19Fixture(t)
	scope, err := taskInputScope(ctx, run)
	if err != nil {
		t.Fatal(err)
	}
	cursor := ownedTaskCommitCursor{Scope: scope, TaskRunID: run.TaskRunID, DefinitionFingerprint: run.DefinitionFingerprint, Generation: run.Generation, AttemptID: run.ExecutionAttemptID.String(), InFlightEffects: map[string]string{"bad": "invalid"}}
	body, err := json.Marshal(cursor)
	if err != nil {
		t.Fatal(err)
	}
	store.resources["checkpoint/"+taskOwnerCommitCursorID(run)] = &coordination.Resource{OwnerID: cursor.Scope.ResourceOwnerID, RoleID: cursor.Scope.RoleID, Kind: "checkpoint", ID: taskOwnerCommitCursorID(run), Revision: 1, Body: body}
	port := AcknowledgedTaskHostPort{Data: store, Executor: taskHostExecutorFunc(func(context.Context, *TaskRun, *TaskDefinition, string, string, TaskHostNativeCall) (json.RawMessage, error) {
		t.Fatal("corrupt cursor allowed a real Native tool execution")
		return nil, nil
	})}
	if _, err := port.Call(ctx, run, def, "rpc-1", "task.host.executeTool", round17ToolRequest(run, "round19.forged")); !IsTaskErrorCode(err, ErrTaskExecutionAttemptInvalid) {
		t.Fatalf("corrupted in-flight evidence was trusted: %v", err)
	}
}

func TestRound19NativeRejectsStaleCheckpointProjection(t *testing.T) {
	ctx, store, run, def := round19Fixture(t)
	cp := round18Checkpoint(run, 1)
	if err := (AcknowledgedTaskCheckpointPort{Data: store}).SaveCheckpoint(ctx, run, cp); err != nil {
		t.Fatal(err)
	}
	called := false
	port := AcknowledgedTaskHostPort{Data: store, Executor: taskHostExecutorFunc(func(context.Context, *TaskRun, *TaskDefinition, string, string, TaskHostNativeCall) (json.RawMessage, error) {
		called = true
		return json.RawMessage(`{"result":true}`), nil
	})}
	if _, err := port.Call(ctx, run, def, "rpc-1", "task.host.executeTool", round17ToolRequest(run, "round19.unprojected")); !IsTaskErrorCode(err, ErrTaskCheckpointIncompatible) || called {
		t.Fatalf("unprojected checkpoint permitted a Native side effect: %v", err)
	}
	operationID := TaskHostNativeOperationID(run, "rpc-1", TaskHostNativeCall{EffectID: "round19.unprojected"})
	if store.resources["tool-result/"+operationID] != nil {
		t.Fatal("unprojected checkpoint created a tool started record")
	}
	run.CheckpointID = &cp.CheckpointID
	if _, err := port.Call(ctx, run, def, "rpc-1", "task.host.executeTool", round17ToolRequest(run, "round19.unprojected")); err != nil || !called {
		t.Fatalf("projected checkpoint did not permit original-generation tool: %v", err)
	}
}

func TestRound19UnresolvedPreviousGenerationCannotAuthorizeNextEffect(t *testing.T) {
	ctx, store, run, def := round19Fixture(t)
	port := AcknowledgedTaskHostPort{Data: store, Executor: taskHostExecutorFunc(func(context.Context, *TaskRun, *TaskDefinition, string, string, TaskHostNativeCall) (json.RawMessage, error) {
		return nil, fmt.Errorf("unconfirmed tool result")
	})}
	if _, err := port.Call(ctx, run, def, "old-rpc", "task.host.executeTool", round17ToolRequest(run, "round19.old")); err == nil {
		t.Fatal("initial tool did not report an unresolved result")
	}
	run.Generation++
	run.ExecutionAttemptID = "second-attempt"
	scope, err := taskInputScope(ctx, run)
	if err != nil {
		t.Fatal(err)
	}
	cursor, revision, err := readTaskOwnerCommitCursor(ctx, store, scope, run)
	if err != nil || cursor == nil || len(cursor.InFlightEffects) != 1 {
		t.Fatalf("old pending cursor missing: %v", err)
	}
	id := TaskHostNativeOperationID(run, "new-rpc", TaskHostNativeCall{EffectID: "round19.new"})
	if _, err := ownerCursorNativeEffectMutation(scope, run, cursor, revision, id, hashBytes([]byte(`{"new":1}`)), false); !IsTaskErrorCode(err, ErrTaskExecutionAttemptInvalid) {
		t.Fatalf("old unresolved effect unlocked new generation: %v", err)
	}
}
