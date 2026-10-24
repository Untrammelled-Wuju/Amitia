package task_runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

func TestRound14OwnerBoundEffectsRejectArgumentOrToolSubstitution(t *testing.T) {
	_, _, scope, run, definition := taskAuthorityFixture(t)
	definition.PermissionRequirementStrings = []string{"service.tool.execute", "event.emit"}
	definition.Checkpoint, definition.Recoverability, definition.Idempotency = true, CheckpointRecoverable, Idempotent
	definition.DefinitionHash = "definition-v1"
	run.TaskDefinitionID = definition.TaskID
	run.DefinitionFingerprint, _ = taskDefinitionFingerprint(definition)
	run.Input = json.RawMessage(`{"plan":true}`)
	run.InputHash = hashBytes(run.Input)
	run.Generation, run.Attempt, run.ExecutionAttemptID = 1, 1, "origin"
	ctx := coordination.WithScope(t.Context(), scope)
	data := &taskInputData{}
	if err := (AcknowledgedTaskInputPort{Data: data}).SaveInput(ctx, run); err != nil {
		t.Fatal(err)
	}
	run.Input = nil
	payload := json.RawMessage(fmt.Sprintf(`{"cursor":1,"nextEffects":[
      {"effectId":"write","method":"task.host.executeTool","toolId":"native.write","inputHash":"%s","timeoutMs":1000},
      {"effectId":"emit","method":"task.host.emitEvent","type":"audit.event","payloadHash":"%s"}
    ]}`, hashBytes([]byte(`{"value":1}`)), hashBytes([]byte(`{"value":2}`))))
	checkpoint := &TaskCheckpoint{CheckpointID: ownedStableCheckpointID(run, 1), TaskRunID: run.TaskRunID,
		Version: 1, Payload: payload, PayloadHash: hashBytes(payload), DefinitionHash: definition.DefinitionHash, InputHash: run.InputHash, CreatedAt: time.Now().UTC()}
	if err := (AcknowledgedTaskCheckpointPort{Data: data}).SaveCheckpoint(ctx, run, checkpoint); err != nil {
		t.Fatal(err)
	}
	run.CheckpointID = &checkpoint.CheckpointID
	run.Generation, run.ExecutionAttemptID = 2, "resumed"
	calls := 0
	port := AcknowledgedTaskHostPort{Data: data, Executor: taskHostExecutorFunc(func(_ context.Context, _ *TaskRun, _ *TaskDefinition, _, method string, _ TaskHostNativeCall) (json.RawMessage, error) {
		calls++
		if method == "task.host.emitEvent" {
			return json.RawMessage(`{"confirmed":true,"eventId":"evt-1","outboxId":"out-1"}`), nil
		}
		return json.RawMessage(`{"result":{"ok":true}}`), nil
	})}
	tool := func(effect, toolID, input string, timeout int) json.RawMessage {
		return json.RawMessage(fmt.Sprintf(`{"task_run_id":%q,"effect_id":%q,"tool_id":%q,"input":%s,"timeout_ms":%d}`, run.TaskRunID, effect, toolID, input, timeout))
	}
	for _, args := range []json.RawMessage{
		tool("write", "native.other", `{"value":1}`, 1000),
		tool("write", "native.write", `{"value":999}`, 1000),
		tool("write", "native.write", `{"value":1}`, 4000),
		tool("unplanned", "native.write", `{"value":1}`, 1000),
	} {
		if _, err := port.Call(ctx, run, definition, "bad-tool", "task.host.executeTool", args); !IsTaskErrorCode(err, ErrTaskExecutionAttemptInvalid) || calls != 0 {
			t.Fatalf("unapproved tool effect was executed: %v calls=%d args=%s", err, calls, args)
		}
	}
	event := func(eventType, body string) json.RawMessage {
		return json.RawMessage(fmt.Sprintf(`{"task_run_id":%q,"effect_id":"emit","type":%q,"payload":%s}`, run.TaskRunID, eventType, body))
	}
	for _, args := range []json.RawMessage{event("audit.other", `{"value":2}`), event("audit.event", `{"value":999}`)} {
		if _, err := port.Call(ctx, run, definition, "bad-event", "task.host.emitEvent", args); !IsTaskErrorCode(err, ErrTaskExecutionAttemptInvalid) || calls != 0 {
			t.Fatalf("unapproved event was emitted: %v calls=%d", err, calls)
		}
	}
	if _, err := port.Call(ctx, run, definition, "good-tool", "task.host.executeTool", tool("write", "native.write", `{"value":1}`, 1000)); err != nil || calls != 1 {
		t.Fatalf("bound tool was not permitted: %v calls=%d", err, calls)
	}
	if _, err := port.Call(ctx, run, definition, "good-event", "task.host.emitEvent", event("audit.event", `{"value":2}`)); err != nil || calls != 2 {
		t.Fatalf("bound event was not permitted: %v calls=%d", err, calls)
	}
	run.Generation, run.ExecutionAttemptID = 3, "third"
	if _, err := port.Call(ctx, run, definition, "another-rpc", "task.host.executeTool", tool("write", "native.write", `{"value":1}`, 1000)); err != nil || calls != 2 {
		t.Fatalf("confirmed effect did not replay: %v calls=%d", err, calls)
	}
}

func TestRound14CheckpointPlanLegacyCannotAuthorizeNewSideEffects(t *testing.T) {
	_, _, scope, run, def := taskAuthorityFixture(t)
	def.PermissionRequirementStrings = []string{"service.tool.execute"}
	def.Checkpoint, def.Recoverability, def.Idempotency = true, CheckpointRecoverable, Idempotent
	def.DefinitionHash = "def"
	run.TaskDefinitionID = def.TaskID
	run.DefinitionFingerprint, _ = taskDefinitionFingerprint(def)
	run.Input = json.RawMessage(`{}`)
	run.InputHash = hashBytes(run.Input)
	run.Generation, run.Attempt, run.ExecutionAttemptID = 1, 1, "origin"
	ctx := coordination.WithScope(t.Context(), scope)
	data := &taskInputData{}
	if err := (AcknowledgedTaskInputPort{Data: data}).SaveInput(ctx, run); err != nil {
		t.Fatal(err)
	}
	run.Input = nil
	payload := json.RawMessage(`{"cursor":1,"nextEffectIds":["legacy.write"]}`)
	cp := &TaskCheckpoint{CheckpointID: ownedStableCheckpointID(run, 1), TaskRunID: run.TaskRunID, Version: 1, Payload: payload, PayloadHash: hashBytes(payload), InputHash: run.InputHash, DefinitionHash: def.DefinitionHash, CreatedAt: time.Now().UTC()}
	if err := (AcknowledgedTaskCheckpointPort{Data: data}).SaveCheckpoint(ctx, run, cp); err != nil {
		t.Fatal(err)
	}
	run.CheckpointID = &cp.CheckpointID
	run.Generation, run.ExecutionAttemptID = 2, "resumed"
	called := false
	port := AcknowledgedTaskHostPort{Data: data, Executor: taskHostExecutorFunc(func(_ context.Context, _ *TaskRun, _ *TaskDefinition, _, _ string, _ TaskHostNativeCall) (json.RawMessage, error) {
		called = true
		return json.RawMessage(`{"result":true}`), nil
	})}
	args := json.RawMessage(fmt.Sprintf(`{"task_run_id":%q,"effect_id":"legacy.write","tool_id":"native.write","input":{}}`, run.TaskRunID))
	if _, err := port.Call(ctx, run, def, "rpc", "task.host.executeTool", args); !IsTaskErrorCode(err, ErrTaskExecutionAttemptInvalid) || called {
		t.Fatalf("legacy id-only checkpoint authorized a new effect: %v", err)
	}
}

func TestRound14MalformedBoundPlansAreNotCommitted(t *testing.T) {
	for _, payload := range []json.RawMessage{
		json.RawMessage(`{"nextEffects":[]}`),
		json.RawMessage(`{"nextEffects":null}`),
		json.RawMessage(`{"nextEffects":[{"effectId":"x","method":"task.host.executeTool","toolId":"native.write","inputHash":"bad"}]}`),
		json.RawMessage(`{"nextEffects":[{"effectId":"x","method":"task.host.other","toolId":"native.write","inputHash":"` + hashBytes([]byte(`{}`)) + `"}]}`),
		json.RawMessage(`{"nextEffects":[{"effectId":"x","method":"task.host.executeTool","toolId":"native.write","inputHash":"` + hashBytes([]byte(`{}`)) + `","input":{"not":"allowed"}}]}`),
		json.RawMessage(`{"nextEffectIds":["x"],"nextEffects":[{"effectId":"x","method":"task.host.executeTool","toolId":"native.write","inputHash":"` + hashBytes([]byte(`{}`)) + `"}]}`),
	} {
		if _, err := checkpointNextEffects(payload); err == nil {
			t.Fatalf("invalid plan accepted: %s", payload)
		}
	}
}

func TestRound14MultipleRestoresUseLatestOwnerBoundCheckpoint(t *testing.T) {
	_, _, scope, run, def := taskAuthorityFixture(t)
	def.PermissionRequirementStrings = []string{"service.tool.execute"}
	def.Checkpoint, def.Recoverability, def.Idempotency = true, CheckpointRecoverable, Idempotent
	def.DefinitionHash = "definition-v1"
	run.TaskDefinitionID = def.TaskID
	run.DefinitionFingerprint, _ = taskDefinitionFingerprint(def)
	run.Input = json.RawMessage(`{}`)
	run.InputHash = hashBytes(run.Input)
	run.Generation, run.Attempt, run.ExecutionAttemptID = 1, 1, "first"
	ctx := coordination.WithScope(t.Context(), scope)
	data := &taskInputData{}
	if err := (AcknowledgedTaskInputPort{Data: data}).SaveInput(ctx, run); err != nil {
		t.Fatal(err)
	}
	run.Input = nil
	save := func(version int64, step string) {
		body := json.RawMessage(fmt.Sprintf(`{"cursor":%d,"nextEffects":[{"effectId":%q,"method":"task.host.executeTool","toolId":"native.echo","inputHash":%q}]}`, version, step, hashBytes([]byte(`{}`))))
		cp := &TaskCheckpoint{CheckpointID: ownedStableCheckpointID(run, version), TaskRunID: run.TaskRunID, Version: version,
			Payload: body, PayloadHash: hashBytes(body), DefinitionHash: def.DefinitionHash, InputHash: run.InputHash, CreatedAt: time.Now().UTC()}
		if err := (AcknowledgedTaskCheckpointPort{Data: data}).SaveCheckpoint(ctx, run, cp); err != nil {
			t.Fatal(err)
		}
		run.CheckpointID = &cp.CheckpointID
	}
	calls := 0
	port := AcknowledgedTaskHostPort{Data: data, Executor: taskHostExecutorFunc(func(_ context.Context, _ *TaskRun, _ *TaskDefinition, _, _ string, _ TaskHostNativeCall) (json.RawMessage, error) {
		calls++
		return json.RawMessage(`{"result":true}`), nil
	})}
	execute := func(step string) error {
		params := json.RawMessage(fmt.Sprintf(`{"task_run_id":%q,"effect_id":%q,"tool_id":"native.echo","input":{}}`, run.TaskRunID, step))
		_, err := port.Call(ctx, run, def, fmt.Sprintf("rpc-%d-%s", run.Generation, step), "task.host.executeTool", params)
		return err
	}
	save(1, "step.one")
	run.Generation, run.ExecutionAttemptID = 2, "second"
	if err := execute("step.one"); err != nil || calls != 1 {
		t.Fatalf("first recovered step: %v calls=%d", err, calls)
	}
	save(2, "step.two")
	run.Generation, run.ExecutionAttemptID = 3, "third"
	if err := execute("step.two"); err != nil || calls != 2 {
		t.Fatalf("second recovered step: %v calls=%d", err, calls)
	}
	if err := execute("step.one"); err != nil || calls != 2 {
		t.Fatalf("confirmed old effect should only replay: %v calls=%d", err, calls)
	}
	if err := execute("step.three"); !IsTaskErrorCode(err, ErrTaskExecutionAttemptInvalid) || calls != 2 {
		t.Fatalf("unplanned future step ran: %v calls=%d", err, calls)
	}
}
