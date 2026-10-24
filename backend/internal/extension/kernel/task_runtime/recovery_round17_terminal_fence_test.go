package task_runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

func round17OwnerFixture(t *testing.T) (context.Context, *taskInputData, *TaskRun, *TaskDefinition) {
	t.Helper()
	_, _, scope, run, def := taskAuthorityFixture(t)
	def.PermissionRequirementStrings = []string{"service.tool.execute"}
	def.DefinitionHash = "round17-definition"
	run.TaskDefinitionID = def.TaskID
	run.DefinitionFingerprint, _ = taskDefinitionFingerprint(def)
	run.Input = json.RawMessage(`{"secret":17}`)
	run.InputHash = hashBytes(run.Input)
	run.Generation, run.ExecutionAttemptID = 1, "round17-original"
	ctx := coordination.WithScope(t.Context(), scope)
	data := &taskInputData{}
	if err := (AcknowledgedTaskInputPort{Data: data}).SaveInput(ctx, run); err != nil {
		t.Fatal(err)
	}
	run.Input = nil
	return ctx, data, run, def
}

func round17ToolRequest(run *TaskRun, effectID string) json.RawMessage {
	return json.RawMessage(fmt.Sprintf(`{"task_run_id":%q,"effect_id":%q,"tool_id":"native.echo","input":{}}`, run.TaskRunID, effectID))
}

func TestRound17OwnerTerminalFencesNewEffectsButAllowsConfirmedReplay(t *testing.T) {
	for _, terminal := range []string{"failed", "cancelled", "timed_out", "manual_intervention"} {
		t.Run(terminal, func(t *testing.T) {
			ctx, data, run, def := round17OwnerFixture(t)
			calls := 0
			port := AcknowledgedTaskHostPort{Data: data, Executor: taskHostExecutorFunc(func(context.Context, *TaskRun, *TaskDefinition, string, string, TaskHostNativeCall) (json.RawMessage, error) {
				calls++
				return json.RawMessage(`{"result":{"ok":true}}`), nil
			})}
			previous := round17ToolRequest(run, "completed.effect")
			first, err := port.Call(ctx, run, def, "request-1", "task.host.executeTool", previous)
			if err != nil || calls != 1 {
				t.Fatalf("original effect failed: %v calls=%d", err, calls)
			}
			if err := (AcknowledgedTaskOutcomePort{Data: data}).SaveOutcome(ctx, run, terminal, nil, "reason", "private detail"); err != nil {
				t.Fatal(err)
			}
			second, err := port.Call(ctx, run, def, "different-rpc", "task.host.executeTool", previous)
			var before, after TaskHostNativeConfirmation
			if err != nil || json.Unmarshal(first, &before) != nil || json.Unmarshal(second, &after) != nil || before.ResultHash != after.ResultHash || string(before.ResultBytes) != string(after.ResultBytes) || after.RequestID != "different-rpc" || before.RequestID != "request-1" || calls != 1 {
				t.Fatalf("confirmed historical effect did not replay: %v calls=%d", err, calls)
			}
			unseen := round17ToolRequest(run, "unplanned.effect")
			if _, err := port.Call(ctx, run, def, "request-2", "task.host.executeTool", unseen); !IsTaskErrorCode(err, ErrTaskExecutionAttemptInvalid) || calls != 1 {
				t.Fatalf("new effect executed after Owner terminal: %v calls=%d", err, calls)
			}
			newID := TaskHostNativeOperationID(run, "request-2", TaskHostNativeCall{EffectID: "unplanned.effect"})
			if data.resources["tool-result/"+newID] != nil {
				t.Fatal("terminal-fenced effect wrote a new started record")
			}
		})
	}
}

func TestRound17OwnerTerminalFencesNewCheckpointsButAllowsExactReplay(t *testing.T) {
	ctx, data, run, _ := round17OwnerFixture(t)
	port := AcknowledgedTaskCheckpointPort{Data: data}
	body := json.RawMessage(`{"cursor":1,"data":{"phase":"done"}}`)
	first := &TaskCheckpoint{
		CheckpointID: ownedStableCheckpointID(run, 1), TaskRunID: run.TaskRunID,
		Version: 1, Payload: body, PayloadHash: hashBytes(body),
		DefinitionHash: "round17-definition", InputHash: run.InputHash,
		CreatedAt: time.Now().UTC(),
	}
	if err := port.SaveCheckpoint(ctx, run, first); err != nil {
		t.Fatal(err)
	}
	run.CheckpointID = &first.CheckpointID
	if err := (AcknowledgedTaskOutcomePort{Data: data}).SaveOutcome(ctx, run, "failed", nil, "failed", "private"); err != nil {
		t.Fatal(err)
	}
	if err := port.SaveCheckpoint(ctx, run, first); err != nil {
		t.Fatalf("exact existing checkpoint should remain idempotent: %v", err)
	}
	second := *first
	second.CheckpointID = ownedStableCheckpointID(run, 2)
	second.Version = 2
	second.Payload = json.RawMessage(`{"cursor":2,"data":{"phase":"late"}}`)
	second.PayloadHash = hashBytes(second.Payload)
	if err := port.SaveCheckpoint(ctx, run, &second); !IsTaskErrorCode(err, ErrTaskExecutionAttemptInvalid) {
		t.Fatalf("Owner committed a checkpoint after terminal outcome: %v", err)
	}
	if data.resources["checkpoint/task/checkpoint/"+second.CheckpointID] != nil {
		t.Fatal("terminal-fenced checkpoint unexpectedly exists")
	}
}

func TestRound17TamperedOwnerOutcomeNeverUnlocksEffectsOrCheckpoints(t *testing.T) {
	ctx, data, run, def := round17OwnerFixture(t)
	if err := (AcknowledgedTaskOutcomePort{Data: data}).SaveOutcome(ctx, run, "failed", nil, "failed", "private"); err != nil {
		t.Fatal(err)
	}
	resource := data.resources["checkpoint/"+taskOutcomeID(run)]
	resource.Revision = 7
	called := false
	port := AcknowledgedTaskHostPort{Data: data, Executor: taskHostExecutorFunc(func(context.Context, *TaskRun, *TaskDefinition, string, string, TaskHostNativeCall) (json.RawMessage, error) {
		called = true
		return json.RawMessage(`{"result":true}`), nil
	})}
	if _, err := port.Call(ctx, run, def, "tool", "task.host.executeTool", round17ToolRequest(run, "new.effect")); err == nil || called {
		t.Fatalf("tampered terminal unlocked effect: %v", err)
	}
	body := json.RawMessage(`{"cursor":1}`)
	cp := &TaskCheckpoint{CheckpointID: ownedStableCheckpointID(run, 1), TaskRunID: run.TaskRunID, Version: 1,
		Payload: body, PayloadHash: hashBytes(body), DefinitionHash: def.DefinitionHash, InputHash: run.InputHash}
	if err := (AcknowledgedTaskCheckpointPort{Data: data}).SaveCheckpoint(ctx, run, cp); err == nil {
		t.Fatal("tampered terminal unlocked a checkpoint")
	}
}

func TestRound17TerminalRequiresLatestOwnerCheckpointProjection(t *testing.T) {
	ctx, data, run, def := round17OwnerFixture(t)
	checkpoints := AcknowledgedTaskCheckpointPort{Data: data}
	outcomes := AcknowledgedTaskOutcomePort{Data: data}
	checkpoint := func(version int64) *TaskCheckpoint {
		payload := json.RawMessage(fmt.Sprintf(`{"cursor":%d,"data":{"version":%d}}`, version, version))
		return &TaskCheckpoint{CheckpointID: ownedStableCheckpointID(run, version), TaskRunID: run.TaskRunID,
			Version: version, Payload: payload, PayloadHash: hashBytes(payload),
			DefinitionHash: def.DefinitionHash, InputHash: run.InputHash, CreatedAt: time.Now().UTC()}
	}
	first := checkpoint(1)
	if err := checkpoints.SaveCheckpoint(ctx, run, first); err != nil {
		t.Fatal(err)
	}
	if err := outcomes.SaveOutcome(ctx, run, "failed", nil, "reason", "secret"); !IsTaskErrorCode(err, ErrTaskCheckpointIncompatible) {
		t.Fatalf("unprojected first checkpoint allowed terminal: %v", err)
	}
	if data.resources["checkpoint/"+taskOutcomeID(run)] != nil {
		t.Fatal("stale checkpoint produced a terminal resource")
	}
	run.CheckpointID = &first.CheckpointID
	second := checkpoint(2)
	if err := checkpoints.SaveCheckpoint(ctx, run, second); err != nil {
		t.Fatal(err)
	}
	if err := outcomes.SaveOutcome(ctx, run, "failed", nil, "reason", "secret"); !IsTaskErrorCode(err, ErrTaskCheckpointIncompatible) {
		t.Fatalf("unprojected second checkpoint allowed terminal: %v", err)
	}
	run.CheckpointID = &second.CheckpointID
	if err := outcomes.SaveOutcome(ctx, run, "failed", nil, "reason", "secret"); err != nil {
		t.Fatalf("fully projected Owner cursor rejected terminal: %v", err)
	}
	if err := outcomes.SaveOutcome(ctx, run, "failed", nil, "reason", "secret"); err != nil {
		t.Fatalf("identical terminal retry was not idempotent: %v", err)
	}
}

func TestRound17TamperedReferencedOwnerCheckpointCannotSealOutcome(t *testing.T) {
	ctx, data, run, def := round17OwnerFixture(t)
	body := json.RawMessage(`{"cursor":1,"data":{"ok":true}}`)
	cp := &TaskCheckpoint{CheckpointID: ownedStableCheckpointID(run, 1), TaskRunID: run.TaskRunID,
		Version: 1, Payload: body, PayloadHash: hashBytes(body),
		DefinitionHash: def.DefinitionHash, InputHash: run.InputHash, CreatedAt: time.Now().UTC()}
	if err := (AcknowledgedTaskCheckpointPort{Data: data}).SaveCheckpoint(ctx, run, cp); err != nil {
		t.Fatal(err)
	}
	run.CheckpointID = &cp.CheckpointID
	resource := data.resources["checkpoint/task/checkpoint/"+cp.CheckpointID]
	resource.Body = json.RawMessage(`{"checkpoint":{"checkpointId":"fake"}}`)
	if err := (AcknowledgedTaskOutcomePort{Data: data}).SaveOutcome(ctx, run, "failed", nil, "reason", "secret"); err == nil {
		t.Fatal("tampered referenced checkpoint was trusted during terminal commit")
	}
	if data.resources["checkpoint/"+taskOutcomeID(run)] != nil {
		t.Fatal("tampered checkpoint caused a terminal Owner write")
	}
}
