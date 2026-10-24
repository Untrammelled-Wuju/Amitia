package task_runtime

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

type round20LostCommitAck struct {
	*ownerCASFixture
	loseConfirmation bool
	persist          bool
}

func (d *round20LostCommitAck) Commit(ctx context.Context, commit coordination.Commit) (coordination.Acknowledgement, error) {
	for _, mutation := range commit.Mutations {
		if mutation.Kind == "tool-result" && mutation.ExpectedRevision == 1 && d.loseConfirmation {
			if d.persist {
				if _, err := d.ownerCASFixture.Commit(ctx, commit); err != nil {
					return coordination.Acknowledgement{}, err
				}
			}
			return coordination.Acknowledgement{}, errors.New("Owner confirmation transport interrupted")
		}
	}
	return d.ownerCASFixture.Commit(ctx, commit)
}

func TestRound20LostNativeConfirmationReconcilesOnlyActualOwnerCommit(t *testing.T) {
	for _, committed := range []bool{true, false} {
		t.Run(map[bool]string{true: "persisted", false: "not-persisted"}[committed], func(t *testing.T) {
			ctx, original, run, definition := round17OwnerFixture(t)
			data := &round20LostCommitAck{ownerCASFixture: newOwnerCASFixture(original), loseConfirmation: true, persist: committed}
			calls := 0
			port := AcknowledgedTaskHostPort{Data: data, Executor: taskHostExecutorFunc(func(context.Context, *TaskRun, *TaskDefinition, string, string, TaskHostNativeCall) (json.RawMessage, error) {
				calls++
				return json.RawMessage(`{"result":{"value":17}}`), nil
			})}
			params := round17ToolRequest(run, "round20.ack")
			result, err := port.Call(ctx, run, definition, "rpc", "task.host.executeTool", params)
			if committed {
				if err != nil || len(result) == 0 || calls != 1 {
					t.Fatalf("verified Owner result not recovered: %s %v count=%d", result, err, calls)
				}
				var ack TaskHostNativeConfirmation
				if json.Unmarshal(result, &ack) != nil || ack.ResultHash != hashBytes(ack.ResultBytes) || ack.Generation != run.Generation {
					t.Fatalf("result not a validated Owner confirmation: %s", result)
				}
			} else if err == nil || len(result) != 0 || calls != 1 {
				t.Fatalf("uncommitted result incorrectly considered confirmed: %s %v calls=%d", result, err, calls)
			}

			replayed, retryErr := port.Call(ctx, run, definition, "rpc", "task.host.executeTool", params)
			if committed && (retryErr != nil || len(replayed) == 0 || calls != 1) {
				t.Fatalf("committed receipt did not replay: %v count=%d", retryErr, calls)
			}
			if !committed && (retryErr == nil || len(replayed) != 0 || calls != 1) {
				t.Fatalf("unknown effect re-executed: %v count=%d", retryErr, calls)
			}
		})
	}
}

func TestRound20NativeInspectionVerifiesExistingCommitCursor(t *testing.T) {
	for _, state := range []string{"started", "confirmed"} {
		t.Run(state, func(t *testing.T) {
			ctx, original, run, definition := round17OwnerFixture(t)
			data := newOwnerCASFixture(original)
			port := AcknowledgedTaskHostPort{Data: data, Executor: taskHostExecutorFunc(func(context.Context, *TaskRun, *TaskDefinition, string, string, TaskHostNativeCall) (json.RawMessage, error) {
				if state == "started" {
					return nil, errors.New("external result unknown")
				}
				return json.RawMessage(`{"result":true}`), nil
			})}
			params := round17ToolRequest(run, "round20.binding")
			_, _ = port.Call(ctx, run, definition, "rpc", "task.host.executeTool", params)
			before, err := port.Inspect(ctx, run, definition, "rpc", "task.host.executeTool", params)
			if err != nil || before.State != state {
				t.Fatalf("unmodified Owner ledger could not be inspected: %+v %v", before, err)
			}
			id := taskOwnerCommitCursorID(run)
			resource := data.resources["checkpoint/"+id]
			if resource == nil {
				t.Fatal("missing existing Owner CAS cursor")
			}
			var cursor ownedTaskCommitCursor
			if err := json.Unmarshal(resource.Body, &cursor); err != nil {
				t.Fatal(err)
			}
			operationID := TaskHostNativeOperationID(run, "rpc", TaskHostNativeCall{EffectID: "round20.binding"})
			if state == "started" {
				delete(cursor.InFlightEffects, operationID)
			} else {
				cursor.InFlightEffects = map[string]string{operationID: hashBytes(params)}
			}
			resource.Body, _ = json.Marshal(cursor)
			if _, err := port.Inspect(ctx, run, definition, "rpc", "task.host.executeTool", params); !IsTaskErrorCode(err, ErrTaskExecutionAttemptInvalid) {
				t.Fatalf("corrupt Owner cursor was treated as a valid native receipt: %v", err)
			}
		})
	}
}

func TestRound20PendingEffectsReadsExistingOwnerRecordsWithoutSensitiveResults(t *testing.T) {
	ctx, original, run, definition := round17OwnerFixture(t)
	data := newOwnerCASFixture(original)
	port := AcknowledgedTaskHostPort{Data: data, Executor: taskHostExecutorFunc(func(context.Context, *TaskRun, *TaskDefinition, string, string, TaskHostNativeCall) (json.RawMessage, error) {
		return nil, errors.New("device tool outcome unavailable")
	})}
	params := round17ToolRequest(run, "round20.unfinished")
	_, _ = port.Call(ctx, run, definition, "rpc", "task.host.executeTool", params)
	pending, err := port.InspectPendingNativeEffects(ctx, run)
	if err != nil || len(pending) != 1 {
		t.Fatalf("Owner in-flight tool was not located: %#v %v", pending, err)
	}
	if pending[0] != TaskHostNativeOperationID(run, "rpc", TaskHostNativeCall{EffectID: "round20.unfinished"}) {
		t.Fatalf("unexpected Owner operation identity: %#v", pending)
	}

	run.Generation, run.ExecutionAttemptID = 2, "restarted-attempt"
	pending, err = port.InspectPendingNativeEffects(ctx, run)
	if err != nil || len(pending) != 1 {
		t.Fatalf("old generation pending effect was lost: %#v %v", pending, err)
	}
	resource := data.resources["tool-result/"+pending[0]]
	var document ownedTaskHostOperation
	if err := json.Unmarshal(resource.Body, &document); err != nil {
		t.Fatal(err)
	}
	document.InputHash = hashBytes([]byte("tampered"))
	resource.Body, _ = json.Marshal(document)
	if _, err := port.InspectPendingNativeEffects(ctx, run); !IsTaskErrorCode(err, ErrTaskExecutionAttemptInvalid) {
		t.Fatalf("tampered in-flight effect was accepted: %v", err)
	}
}

func TestRound20PendingEffectsDoesNotExposePrivateOwnerResults(t *testing.T) {
	ctx, original, run, definition := round17OwnerFixture(t)
	data := newOwnerCASFixture(original)
	port := AcknowledgedTaskHostPort{Data: data, Executor: taskHostExecutorFunc(func(context.Context, *TaskRun, *TaskDefinition, string, string, TaskHostNativeCall) (json.RawMessage, error) {
		return json.RawMessage(`{"result":{"secret":"private-result"}}`), nil
	})}
	params := round17ToolRequest(run, "round20.completed")
	if _, err := port.Call(ctx, run, definition, "rpc", "task.host.executeTool", params); err != nil {
		t.Fatal(err)
	}
	pending, err := port.InspectPendingNativeEffects(ctx, run)
	if err != nil || len(pending) != 0 {
		t.Fatalf("confirmed operation appeared to be pending: %#v %v", pending, err)
	}
}
