package task_runtime

import (
	"encoding/json"
	"testing"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

func TestOwnedTaskCheckpointKeepsBodyAtOwnerAndRestoresExactBytes(t *testing.T) {
	service, _, authority, run, definition := taskAuthorityFixture(t)
	run.TaskDefinitionID = definition.TaskID
	run.DefinitionFingerprint, _ = taskDefinitionFingerprint(definition)
	run.Input = json.RawMessage(`{"private":"input"}`)
	run.InputHash, run.Status, run.ExecutionAttemptID, run.Generation = hashBytes(run.Input), RunStatusRunning, "attempt", 1
	data := &taskInputData{}
	service.config.OwnedCheckpoints = AcknowledgedTaskCheckpointPort{Data: data}
	ctx := coordination.WithScope(t.Context(), authority)
	if err := (AcknowledgedTaskInputPort{Data: data}).SaveInput(ctx, run); err != nil {
		t.Fatal(err)
	}
	run.Input = nil
	store := &checkpointFenceStore{completionFenceStore: completionFenceStore{run: CloneTaskRun(run)}}
	service.store = store
	payload := json.RawMessage("{\n  \"cursor\": 2, \"private\": \"source checkpoint\"\n}")
	hash := hashBytes(payload)
	data.wrongAck = true
	service.handleCheckpoint(ctx, run, definition, payload, hash, 2)
	if store.checkpoint != nil || store.run.CheckpointID != nil {
		t.Fatal("checkpoint metadata stored before owner acknowledgement")
	}
	data.wrongAck = false
	service.handleCheckpoint(ctx, run, definition, payload, hash, 2)
	if store.checkpoint == nil || len(store.checkpoint.Payload) != 0 || store.checkpoint.PayloadHash != hash || store.run.CheckpointID == nil || data.resource.OwnerID != authority.ResourceOwnerID {
		t.Fatalf("checkpoint body copied to queue storage: %+v", store.checkpoint)
	}
	loaded, err := service.readTaskCheckpoint(ctx, store.run)
	if err != nil || string(loaded.Payload) != string(payload) || loaded.PayloadHash != hash || len(store.checkpoint.Payload) != 0 {
		t.Fatalf("checkpoint changed bytes on restore: %+v %v", loaded, err)
	}
	changed := authority
	changed.PermissionRevision++
	if _, err := service.readTaskCheckpoint(coordination.WithScope(t.Context(), changed), store.run); !IsTaskErrorCode(err, ErrTaskScopeDenied) {
		t.Fatalf("old owner checkpoint reused under different permissions: %v", err)
	}
	store.checkpoint.PayloadHash = "changed-reference"
	if _, err := service.readTaskCheckpoint(ctx, store.run); !IsTaskErrorCode(err, ErrTaskCheckpointIncompatible) {
		t.Fatalf("checkpoint metadata mismatch accepted: %v", err)
	}
}
