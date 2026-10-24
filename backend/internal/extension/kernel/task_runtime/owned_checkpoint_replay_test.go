package task_runtime

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

func TestOwnedCheckpointStableIdentityAndOwnerReplay(t *testing.T) {
	_, _, authority, run, definition := taskAuthorityFixture(t)
	run.TaskDefinitionID = definition.TaskID
	run.DefinitionFingerprint, _ = taskDefinitionFingerprint(definition)
	run.Generation = 5
	run.ExecutionAttemptID = "attempt-one"
	run.Input = json.RawMessage(`{"private":true}`)
	run.InputHash = hashBytes(run.Input)
	data := &taskInputData{}
	ctx := coordination.WithScope(t.Context(), authority)
	if err := (AcknowledgedTaskInputPort{Data: data}).SaveInput(ctx, run); err != nil {
		t.Fatal(err)
	}
	run.Input = nil
	cpID := ownedStableCheckpointID(run, 1)
	cpIDAgain := ownedStableCheckpointID(run, 1)
	if cpID == "" || cpIDAgain != cpID || ownedStableCheckpointID(run, 2) == cpID {
		t.Fatal("checkpoint logical step ID is unstable")
	}
	differentAttempt := CloneTaskRun(run)
	differentAttempt.ExecutionAttemptID = "attempt-two"
	if ownedStableCheckpointID(differentAttempt, 1) == cpID {
		t.Fatal("checkpoint reused a foreign attempt")
	}
	body := json.RawMessage(`{"cursor":1}`)
	cp := &TaskCheckpoint{CheckpointID: cpID, TaskRunID: run.TaskRunID, Version: 1, Payload: body, PayloadHash: hashBytes(body), DefinitionHash: definition.DefinitionHash, InputHash: run.InputHash, CreatedAt: time.Now().Add(-time.Minute).UTC()}
	commits := 0
	data.afterCommit = func() { commits++ }
	port := AcknowledgedTaskCheckpointPort{Data: data}
	if err := port.SaveCheckpoint(ctx, run, cp); err != nil {
		t.Fatal(err)
	}
	retry := *cp
	retry.CreatedAt = time.Now().UTC()
	if err := port.SaveCheckpoint(ctx, run, &retry); err != nil {
		t.Fatalf("confirmed checkpoint replay rejected: %v", err)
	}
	if commits != 1 {
		t.Fatalf("Owner checkpoint was written twice: %d", commits)
	}
	wrong := retry
	wrong.Payload = json.RawMessage(`{"cursor":2}`)
	wrong.PayloadHash = hashBytes(wrong.Payload)
	if err := port.SaveCheckpoint(ctx, run, &wrong); err == nil {
		t.Fatal("different checkpoint silently reused")
	}
	if commits != 1 {
		t.Fatal("checkpoint conflict mutated owner")
	}
	record := data.resources["checkpoint/task/checkpoint/"+cpID]
	if record == nil {
		t.Fatal("authoritative checkpoint record missing")
	}
	record.Body = json.RawMessage(`{"tampered":true}`)
	if err := port.SaveCheckpoint(ctx, run, cp); err == nil {
		t.Fatal("corrupted checkpoint accepted")
	}
}
