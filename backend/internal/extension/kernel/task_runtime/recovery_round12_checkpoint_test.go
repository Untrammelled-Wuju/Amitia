package task_runtime

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

func TestRound12LocalCheckpointReplayUsesConfirmedProjection(t *testing.T) {
	svc, _, _, run, def := taskAuthorityFixture(t)
	run.Status, run.Generation, run.ExecutionAttemptID = RunStatusRunning, 2, "attempt-local"
	run.InputHash = hashBytes([]byte(`{}`))
	def.DefinitionHash = "definition-v1"
	store := &checkpointFenceStore{completionFenceStore: completionFenceStore{run: CloneTaskRun(run)}}
	svc.store = store
	payload := json.RawMessage(`{"cursor":3,"data":{"important":true}}`)
	ctx := t.Context()
	if err := svc.handleCheckpoint(ctx, run, def, payload, hashBytes(payload), 3); err != nil {
		t.Fatal(err)
	}
	firstID := store.checkpoint.CheckpointID
	revision := store.run.Revision
	if err := svc.handleCheckpoint(ctx, run, def, payload, hashBytes(payload), 3); err != nil {
		t.Fatalf("exact repeat was rejected: %v", err)
	}
	if store.casCalls != 1 || store.run.Revision != revision || store.checkpoint.CheckpointID != firstID {
		t.Fatal("exact checkpoint replay caused a second local mutation")
	}
	changed := json.RawMessage(`{"cursor":3,"data":{"important":false}}`)
	if err := svc.handleCheckpoint(ctx, run, def, changed, hashBytes(changed), 3); err == nil {
		t.Fatal("same version with different bytes accepted")
	}
	if err := svc.handleCheckpoint(ctx, run, def, payload, hashBytes(payload), 2); err == nil {
		t.Fatal("stale version accepted")
	}
	if store.casCalls != 1 {
		t.Fatal("conflicting checkpoint mutated local TaskRun")
	}
}

func TestRound12OwnerCheckpointReplayChecksOriginalPrivateResource(t *testing.T) {
	svc, _, authority, run, def := taskAuthorityFixture(t)
	run.TaskDefinitionID = def.TaskID
	run.DefinitionFingerprint, _ = taskDefinitionFingerprint(def)
	run.Status, run.Generation, run.ExecutionAttemptID = RunStatusRunning, 3, "owner-attempt"
	run.Input = json.RawMessage(`{"secret":true}`)
	run.InputHash = hashBytes(run.Input)
	def.DefinitionHash = "definition-v1"
	ctx := coordination.WithScope(t.Context(), authority)
	data := &taskInputData{}
	if err := (AcknowledgedTaskInputPort{Data: data}).SaveInput(ctx, run); err != nil {
		t.Fatal(err)
	}
	run.Input = nil
	svc.config.OwnedCheckpoints = AcknowledgedTaskCheckpointPort{Data: data}
	store := &checkpointFenceStore{completionFenceStore: completionFenceStore{run: CloneTaskRun(run)}}
	svc.store = store
	payload := json.RawMessage(`{"cursor":1,"secret":"private checkpoint"}`)
	if err := svc.handleCheckpoint(ctx, run, def, payload, hashBytes(payload), 1); err != nil {
		t.Fatal(err)
	}
	committedID := store.checkpoint.CheckpointID
	ownerCommits := 0
	data.afterCommit = func() { ownerCommits++ }
	if err := svc.handleCheckpoint(ctx, run, def, payload, hashBytes(payload), 1); err != nil {
		t.Fatalf("confirmed Owner checkpoint refused retry: %v", err)
	}
	if store.casCalls != 1 || ownerCommits != 0 || store.checkpoint.CheckpointID != committedID || len(store.checkpoint.Payload) != 0 {
		t.Fatal("Owner replay wrote a second checkpoint or leaked private bytes")
	}
	changed := json.RawMessage(`{"cursor":1,"secret":"different"}`)
	if err := svc.handleCheckpoint(ctx, run, def, changed, hashBytes(changed), 1); err == nil {
		t.Fatal("different Owner checkpoint accepted")
	}
	ownerRecord := data.resources["checkpoint/task/checkpoint/"+committedID]
	if ownerRecord == nil {
		t.Fatal("Owner checkpoint not found")
	}
	ownerRecord.Body = json.RawMessage(`{"tampered":true}`)
	if err := svc.handleCheckpoint(ctx, run, def, payload, hashBytes(payload), 1); err == nil {
		t.Fatal("corrupt Owner checkpoint verified by local metadata alone")
	}
	if store.casCalls != 1 || ownerCommits != 0 {
		t.Fatal("replayed callback altered original state")
	}
}

func TestRound12DeviceResumeUsesSingleCheckpointValidator(t *testing.T) {
	run := &TaskRun{TaskRunID: "run", InputHash: "input", Generation: 2}
	def := &TaskDefinition{DefinitionHash: "definition"}
	if got, err := verifiedSourceResumeCheckpoint(run, def, nil); err != nil || got != nil {
		t.Fatalf("fresh run cannot require checkpoint: %v", err)
	}
	id := "confirmed-checkpoint"
	run.CheckpointID = &id
	payload := json.RawMessage(`{"cursor":5,"data":"private"}`)
	cp := TaskCheckpoint{CheckpointID: id, TaskRunID: run.TaskRunID, Version: 5, Payload: payload, PayloadHash: hashBytes(payload), DefinitionHash: def.DefinitionHash, InputHash: run.InputHash}
	pack := func(c TaskCheckpoint) json.RawMessage {
		b, err := json.Marshal(c)
		if err != nil {
			t.Fatal(err)
		}
		return b
	}
	if got, err := verifiedSourceResumeCheckpoint(run, def, pack(cp)); err != nil || string(got) != string(payload) {
		t.Fatalf("valid Owner checkpoint was rejected: %v %s", err, got)
	}
	for name, mutate := range map[string]func(*TaskCheckpoint){
		"wrong definition": func(c *TaskCheckpoint) { c.DefinitionHash = "replaced" },
		"wrong input":      func(c *TaskCheckpoint) { c.InputHash = "other" },
		"wrong reference":  func(c *TaskCheckpoint) { c.CheckpointID = "other" },
		"changed bytes":    func(c *TaskCheckpoint) { c.Payload = json.RawMessage(`{"cursor":5,"data":"altered"}`) },
		"wrong cursor": func(c *TaskCheckpoint) {
			c.Payload = json.RawMessage(`{"cursor":4}`)
			c.PayloadHash = hashBytes(c.Payload)
		},
		"missing payload": func(c *TaskCheckpoint) { c.Payload = nil; c.PayloadHash = hashBytes(c.Payload) },
	} {
		t.Run(name, func(t *testing.T) {
			bad := cp
			mutate(&bad)
			if _, err := verifiedSourceResumeCheckpoint(run, def, pack(bad)); err == nil {
				t.Fatal("untrusted device checkpoint accepted")
			}
		})
	}
	run.CheckpointID = nil
	if _, err := verifiedSourceResumeCheckpoint(run, def, pack(cp)); err == nil {
		t.Fatal("new task accepted an unbound resume checkpoint")
	}
}

func TestRound12RecoveryGenerationCannotDispatchNewNativeEffect(t *testing.T) {
	_, _, authority, run, def := taskAuthorityFixture(t)
	def.PermissionRequirementStrings = []string{"service.tool.execute"}
	run.TaskDefinitionID = def.TaskID
	run.DefinitionFingerprint, _ = taskDefinitionFingerprint(def)
	run.Input = json.RawMessage(`{"input":"private"}`)
	run.InputHash = hashBytes(run.Input)
	run.Generation, run.Attempt, run.ExecutionAttemptID = 1, 1, "initial-attempt"
	ctx := coordination.WithScope(t.Context(), authority)
	data := &taskInputData{}
	if err := (AcknowledgedTaskInputPort{Data: data}).SaveInput(ctx, run); err != nil {
		t.Fatal(err)
	}
	run.Input = nil
	callCount := 0
	port := AcknowledgedTaskHostPort{Data: data, Executor: taskHostExecutorFunc(func(_ context.Context, _ *TaskRun, _ *TaskDefinition, _, _ string, _ TaskHostNativeCall) (json.RawMessage, error) {
		callCount++
		return json.RawMessage(`{"result":{"ok":true}}`), nil
	})}
	args := func(id string) json.RawMessage {
		return json.RawMessage(`{"task_run_id":"` + run.TaskRunID + `","effect_id":"` + id + `","tool_id":"native.echo","input":{}}`)
	}
	if _, err := port.Call(ctx, run, def, "original-rpc", "task.host.executeTool", args("step-001")); err != nil {
		t.Fatal(err)
	}
	run.Generation, run.ExecutionAttemptID = 2, "resumed-attempt"
	if _, err := port.Call(ctx, run, def, "next-rpc", "task.host.executeTool", args("step-002")); !IsTaskErrorCode(err, ErrTaskExecutionAttemptInvalid) || callCount != 1 {
		t.Fatalf("generation-only recovery dispatched an untracked native effect: %v calls=%d", err, callCount)
	}
	if _, err := port.Call(ctx, run, def, "replayed-rpc", "task.host.executeTool", args("step-001")); err != nil || callCount != 1 {
		t.Fatalf("confirmed stable effect did not replay: %v calls=%d", err, callCount)
	}
	if _, err := port.Call(ctx, run, def, "legacy-rpc", "task.host.executeTool", json.RawMessage(`{"task_run_id":"`+run.TaskRunID+`","tool_id":"native.echo","input":{}}`)); !IsTaskErrorCode(err, ErrTaskExecutionAttemptInvalid) || callCount != 1 {
		t.Fatalf("legacy RPC bypassed recovered generation: %v calls=%d", err, callCount)
	}
}

func TestRound12OwnerCheckpointCommitSurvivesFailedLocalCAS(t *testing.T) {
	svc, _, authority, run, def := taskAuthorityFixture(t)
	run.TaskDefinitionID = def.TaskID
	run.DefinitionFingerprint, _ = taskDefinitionFingerprint(def)
	def.DefinitionHash = "def-v1"
	run.Status, run.Generation, run.ExecutionAttemptID = RunStatusRunning, 1, "old-attempt"
	run.Input = json.RawMessage(`{"private":1}`)
	run.InputHash = hashBytes(run.Input)
	ctx := coordination.WithScope(t.Context(), authority)
	data := &taskInputData{}
	if err := (AcknowledgedTaskInputPort{Data: data}).SaveInput(ctx, run); err != nil {
		t.Fatal(err)
	}
	run.Input = nil
	svc.config.OwnedCheckpoints = AcknowledgedTaskCheckpointPort{Data: data}
	store := &checkpointFenceStore{completionFenceStore: completionFenceStore{run: CloneTaskRun(run), reject: true}}
	svc.store = store
	payload := json.RawMessage(`{"cursor":1}`)
	commits := 0
	data.afterCommit = func() { commits++ }
	if err := svc.handleCheckpoint(ctx, run, def, payload, hashBytes(payload), 1); err == nil {
		t.Fatal("uncommitted Core checkpoint reported success")
	}
	if commits != 1 || store.checkpoint != nil || store.run.CheckpointID != nil {
		t.Fatal("Core failed CAS incorrectly changed local checkpoint")
	}
	store.reject = false
	if err := svc.handleCheckpoint(ctx, run, def, payload, hashBytes(payload), 1); err != nil {
		t.Fatalf("original Owner commit was not reusable after failed Core CAS: %v", err)
	}
	if commits != 1 || store.checkpoint == nil || store.run.CheckpointID == nil || store.casCalls != 2 {
		t.Fatal("checkpoint retry duplicated Owner commit or failed Core projection")
	}
	if err := svc.handleCheckpoint(ctx, run, def, payload, hashBytes(payload), 1); err != nil || store.casCalls != 2 || commits != 1 {
		t.Fatalf("confirmed checkpoint retry was not idempotent: %v", err)
	}
}
