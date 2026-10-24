package task_runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

func TestRound13ResumedStepManifestAllowsOnlyPlannedFirstExecution(t *testing.T) {
	_, _, authority, run, definition := taskAuthorityFixture(t)
	definition.PermissionRequirementStrings = []string{"service.tool.execute"}
	definition.Checkpoint = true
	definition.Recoverability = CheckpointRecoverable
	definition.Idempotency = Idempotent
	definition.DefinitionHash = "definition-v1"
	run.TaskDefinitionID = definition.TaskID
	run.DefinitionFingerprint, _ = taskDefinitionFingerprint(definition)
	run.Input = json.RawMessage(`{"private":true}`)
	run.InputHash = hashBytes(run.Input)
	run.Generation, run.Attempt, run.ExecutionAttemptID = 1, 1, "original-attempt"
	ctx := coordination.WithScope(t.Context(), authority)
	data := &taskInputData{}
	if err := (AcknowledgedTaskInputPort{Data: data}).SaveInput(ctx, run); err != nil {
		t.Fatal(err)
	}
	run.Input = nil
	payload := json.RawMessage(fmt.Sprintf(`{"cursor":1,"nextEffects":[
        {"effectId":"step.003","method":"task.host.executeTool","toolId":"native.echo","inputHash":"%s"},
        {"effectId":"step.004","method":"task.host.executeTool","toolId":"native.echo","inputHash":"%s"}
    ]}`, hashBytes([]byte(`{}`)), hashBytes([]byte(`{}`))))
	cp := &TaskCheckpoint{CheckpointID: ownedStableCheckpointID(run, 1), TaskRunID: run.TaskRunID, Version: 1,
		Payload: payload, PayloadHash: hashBytes(payload), DefinitionHash: definition.DefinitionHash, InputHash: run.InputHash, CreatedAt: time.Now().UTC()}
	if err := (AcknowledgedTaskCheckpointPort{Data: data}).SaveCheckpoint(ctx, run, cp); err != nil {
		t.Fatal(err)
	}
	run.CheckpointID = &cp.CheckpointID
	run.Generation, run.Attempt, run.ExecutionAttemptID = 2, 1, "resumed-attempt"
	calls := 0
	port := AcknowledgedTaskHostPort{Data: data, Executor: taskHostExecutorFunc(func(_ context.Context, _ *TaskRun, _ *TaskDefinition, _, _ string, _ TaskHostNativeCall) (json.RawMessage, error) {
		calls++
		return json.RawMessage(`{"result":{"ok":true}}`), nil
	})}
	args := func(effect string) json.RawMessage {
		return json.RawMessage(`{"task_run_id":"` + run.TaskRunID + `","effect_id":"` + effect + `","tool_id":"native.echo","input":{}}`)
	}
	if _, err := port.Call(ctx, run, definition, "unknown", "task.host.executeTool", args("step.not-planned")); !IsTaskErrorCode(err, ErrTaskExecutionAttemptInvalid) || calls != 0 {
		t.Fatalf("unplanned effect dispatched: %v calls=%d", err, calls)
	}
	if _, err := port.Call(ctx, run, definition, "planned", "task.host.executeTool", args("step.003")); err != nil || calls != 1 {
		t.Fatalf("planned new effect not dispatched: %v calls=%d", err, calls)
	}
	if _, err := port.Call(ctx, run, definition, "replay", "task.host.executeTool", args("step.003")); err != nil || calls != 1 {
		t.Fatalf("confirmed effect executed twice: %v calls=%d", err, calls)
	}
	modified := json.RawMessage(`{"task_run_id":"` + run.TaskRunID + `","effect_id":"step.003","tool_id":"native.echo","input":{"changed":true}}`)
	if _, err := port.Call(ctx, run, definition, "tamper", "task.host.executeTool", modified); err == nil || calls != 1 {
		t.Fatalf("changed effect input was accepted: %v calls=%d", err, calls)
	}
	if _, err := port.Call(ctx, run, definition, "legacy", "task.host.executeTool", json.RawMessage(`{"task_run_id":"`+run.TaskRunID+`","tool_id":"native.echo","input":{}}`)); err == nil || calls != 1 {
		t.Fatalf("legacy retry bypassed step plan: %v calls=%d", err, calls)
	}

	run.Generation, run.ExecutionAttemptID = 3, "third-attempt"
	if _, err := port.Call(ctx, run, definition, "new-rpc", "task.host.executeTool", args("step.003")); err != nil || calls != 1 {
		t.Fatalf("cross-generation replay failed: %v calls=%d", err, calls)
	}

	unknownArgs := args("step.004")
	unknownCall := TaskHostNativeCall{TaskRunID: run.TaskRunID, EffectID: "step.004"}
	id := TaskHostNativeOperationID(run, "unknown-previous-attempt", unknownCall)
	document := ownedTaskHostOperation{Scope: authority, TaskRunID: run.TaskRunID, Generation: 2,
		AttemptID: "resumed-attempt", DefinitionFingerprint: run.DefinitionFingerprint, RequestID: "unknown-previous-attempt",
		EffectID: "step.004", Method: "task.host.executeTool", InputHash: hashBytes(unknownArgs), State: "started"}
	encoded, err := json.Marshal(document)
	if err != nil {
		t.Fatal(err)
	}
	data.resources["tool-result/"+id] = &coordination.Resource{OwnerID: authority.ResourceOwnerID, RoleID: authority.RoleID, Kind: "tool-result", ID: id, Revision: 1, Body: encoded}
	if _, err := port.Call(ctx, run, definition, "replay-unknown", "task.host.executeTool", unknownArgs); !IsTaskErrorCode(err, ErrTaskExecutionAttemptInvalid) || calls != 1 {
		t.Fatalf("unconfirmed planned effect was dispatched: %v calls=%d", err, calls)
	}
}

func TestRound13CheckpointManifestValidationAndTamperFailClosed(t *testing.T) {
	_, _, authority, run, def := taskAuthorityFixture(t)
	def.PermissionRequirementStrings = []string{"service.tool.execute"}
	def.Checkpoint, def.Recoverability, def.Idempotency = true, CheckpointRecoverable, Idempotent
	def.DefinitionHash = "definition-v1"
	run.TaskDefinitionID = def.TaskID
	run.DefinitionFingerprint, _ = taskDefinitionFingerprint(def)
	run.Input = json.RawMessage(`{}`)
	run.InputHash = hashBytes(run.Input)
	run.Generation, run.Attempt, run.ExecutionAttemptID = 1, 1, "original"
	ctx := coordination.WithScope(t.Context(), authority)
	data := &taskInputData{}
	if err := (AcknowledgedTaskInputPort{Data: data}).SaveInput(ctx, run); err != nil {
		t.Fatal(err)
	}
	run.Input = nil
	for _, payload := range []json.RawMessage{
		json.RawMessage(`{"cursor":1,"nextEffectIds":[]}`),
		json.RawMessage(`{"cursor":1,"nextEffectIds":null}`),
		json.RawMessage(`{"cursor":1,"nextEffectIds":["bad/path"]}`),
		json.RawMessage(`{"cursor":1,"nextEffectIds":["duplicate","duplicate"]}`),
		json.RawMessage(`{"cursor":1,"nextEffectIds":["` + strings.Repeat("X", 129) + `"]}`),
	} {
		cp := &TaskCheckpoint{CheckpointID: ownedStableCheckpointID(run, 1), TaskRunID: run.TaskRunID,
			Version: 1, Payload: payload, PayloadHash: hashBytes(payload), DefinitionHash: def.DefinitionHash,
			InputHash: run.InputHash, CreatedAt: time.Now().UTC()}
		if err := (AcknowledgedTaskCheckpointPort{Data: data}).SaveCheckpoint(ctx, run, cp); err == nil {
			t.Fatalf("invalid plan committed: %s", payload)
		}
	}
	payload := json.RawMessage(fmt.Sprintf(`{"cursor":1,"nextEffects":[
        {"effectId":"approved","method":"task.host.executeTool","toolId":"native.echo","inputHash":"%s"}
    ]}`, hashBytes([]byte(`{}`))))
	cp := &TaskCheckpoint{CheckpointID: ownedStableCheckpointID(run, 1), TaskRunID: run.TaskRunID,
		Version: 1, Payload: payload, PayloadHash: hashBytes(payload), DefinitionHash: def.DefinitionHash,
		InputHash: run.InputHash, CreatedAt: time.Now().UTC()}
	if err := (AcknowledgedTaskCheckpointPort{Data: data}).SaveCheckpoint(ctx, run, cp); err != nil {
		t.Fatal(err)
	}
	run.CheckpointID = &cp.CheckpointID
	run.Generation, run.ExecutionAttemptID = 2, "resumed"
	inspector := AcknowledgedTaskCheckpointPort{Data: data}
	if allowed, err := inspector.AuthorizesPostCheckpointEffect(ctx, run, def, "task.host.executeTool", TaskHostNativeCall{EffectID: "approved", ToolID: "native.echo", Input: json.RawMessage(`{}`)}); err != nil || !allowed {
		t.Fatalf("valid plan not trusted: %v", err)
	}
	changed := authority
	changed.RoleRevision++
	if allowed, err := inspector.AuthorizesPostCheckpointEffect(coordination.WithScope(t.Context(), changed), run, def, "task.host.executeTool", TaskHostNativeCall{EffectID: "approved", ToolID: "native.echo", Input: json.RawMessage(`{}`)}); err == nil || allowed {
		t.Fatal("changed Owner authorization retained effect approval")
	}
	resource := data.resources["checkpoint/task/checkpoint/"+cp.CheckpointID]
	originalBody := append([]byte(nil), resource.Body...)
	var mismatched ownedTaskCheckpoint
	if err := json.Unmarshal(originalBody, &mismatched); err != nil {
		t.Fatal(err)
	}
	mismatched.ExecutionGeneration = run.Generation + 1
	resource.Body, _ = json.Marshal(mismatched)
	if allowed, err := inspector.AuthorizesPostCheckpointEffect(ctx, run, def, "task.host.executeTool", TaskHostNativeCall{EffectID: "approved", ToolID: "native.echo", Input: json.RawMessage(`{}`)}); err == nil || allowed {
		t.Fatal("checkpoint from a future generation was approved")
	}
	resource.Body = originalBody
	resource.Body = json.RawMessage(`{"tampered":true}`)
	if allowed, err := inspector.AuthorizesPostCheckpointEffect(ctx, run, def, "task.host.executeTool", TaskHostNativeCall{EffectID: "approved", ToolID: "native.echo", Input: json.RawMessage(`{}`)}); err == nil || allowed {
		t.Fatal("tampered Owner checkpoint authorized a side effect")
	}
}
