package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"testing"

	"github.com/u-ai/backend/internal/character"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/extension/kernel/task_runtime"
)

func TestMeshTaskInputActualDevicePortStoresOnlyAtOwnerAndRejectsChangedRole(t *testing.T) {
	p := setupMeshLocalDataPort(t)
	db := p.services.KernelContainer.DeviceRegistry.Database()
	if _, err := db.Exec(`INSERT INTO kernel_devices(device_id,space_id,trust_state,created_at,last_seen_at) VALUES('device-a','core','trusted','now','now')`); err != nil {
		t.Fatal(err)
	}
	service := coordination.NewService(db)
	ctx, scope, finish, err := service.Begin(t.Context(), "core", "device-a", "device-a", "core", "one", "owned-task")
	if err != nil {
		t.Fatal(err)
	}
	defer finish()
	scope.RoleRevision, scope.TurnID, scope.ExecutionID = 3, "turn", "execution"
	ctx = coordination.WithScope(ctx, scope)
	input := json.RawMessage("{\n  \"private\": \"device task input\"\n}")
	digest := sha256.Sum256(input)
	run := &task_runtime.TaskRun{TaskRunID: "owned-task-run", TaskDefinitionID: "task", ExtensionID: "extension", ModuleID: "module", ScopeSnapshotID: "persisted-snapshot", DefinitionFingerprint: "definition-pin", Input: input, InputHash: hex.EncodeToString(digest[:])}
	port := task_runtime.AcknowledgedTaskInputPort{Data: p}
	if err := port.SaveInput(ctx, run); err != nil {
		t.Fatal(err)
	}
	metadata := task_runtime.CloneTaskRun(run)
	metadata.Input = nil
	if loaded, err := port.Input(ctx, metadata); err != nil || string(loaded) != string(input) {
		t.Fatalf("device input could not be restored: %s %v", loaded, err)
	}
	payload := json.RawMessage("{\n  \"cursor\": 2, \"private\": \"device checkpoint\"\n}")
	payloadDigest := sha256.Sum256(payload)
	cp := &task_runtime.TaskCheckpoint{CheckpointID: "device-checkpoint", TaskRunID: run.TaskRunID, Version: 2, Payload: payload, PayloadHash: hex.EncodeToString(payloadDigest[:]), DefinitionHash: "definition", InputHash: run.InputHash}
	checkpointPort := task_runtime.AcknowledgedTaskCheckpointPort{Data: p}
	if err := checkpointPort.SaveCheckpoint(ctx, metadata, cp); err != nil {
		t.Fatal(err)
	}
	cpMetadata := *cp
	cpMetadata.Payload = nil
	if loaded, err := checkpointPort.Checkpoint(ctx, metadata, &cpMetadata); err != nil || string(loaded.Payload) != string(payload) {
		t.Fatalf("device checkpoint could not be restored: %+v %v", loaded, err)
	}
	metadata.Generation, metadata.ExecutionAttemptID = 1, "attempt"
	output := json.RawMessage("{\n  \"private\": \"device outcome\"\n}")
	outputDigest := sha256.Sum256(output)
	result := &task_runtime.TaskRunResult{TaskRunID: run.TaskRunID, ResultType: task_runtime.ResultInlineJSON, ResultJSON: output, ResultHash: hex.EncodeToString(outputDigest[:])}
	outcomePort := task_runtime.AcknowledgedTaskOutcomePort{Data: p}
	if err := outcomePort.SaveOutcome(ctx, metadata, "succeeded", result, "", ""); err != nil {
		t.Fatal(err)
	}
	resultMetadata := *result
	resultMetadata.ResultJSON = nil
	if loaded, err := outcomePort.Result(ctx, metadata, &resultMetadata); err != nil || string(loaded.ResultJSON) != string(output) {
		t.Fatalf("device outcome could not be restored: %+v %v", loaded, err)
	}
	progressPort := task_runtime.AcknowledgedTaskProgressPort{Data: p}
	progressReference, err := progressPort.SaveProgress(ctx, metadata, task_runtime.TaskRunProgress{TaskRunID: run.TaskRunID, Sequence: 1, Message: "device-private-progress"})
	if err != nil {
		t.Fatal(err)
	}
	if loaded, err := progressPort.Progress(ctx, metadata, &progressReference); err != nil || loaded.Message != "device-private-progress" || progressReference.Message != "" {
		t.Fatalf("device progress could not be restored: %+v %v", loaded, err)
	}
	storagePort := task_runtime.AcknowledgedTaskStoragePort{Data: p}
	storageParams, _ := json.Marshal(map[string]any{"task_run_id": run.TaskRunID, "key": "cursor", "value": map[string]string{"private": "device-owned-storage"}})
	if _, err := storagePort.Call(ctx, metadata, "storage-set", "task.storage.set", storageParams); err != nil {
		t.Fatal(err)
	}
	storageGet, _ := json.Marshal(map[string]string{"task_run_id": run.TaskRunID, "key": "cursor"})
	if value, err := storagePort.Call(ctx, metadata, "storage-get", "task.storage.get", storageGet); err != nil || string(value) != `{"value":{"private":"device-owned-storage"}}` {
		t.Fatalf("device storage changed: %s %v", value, err)
	}
	artifactPort := task_runtime.AcknowledgedTaskArtifactPort{Data: p}
	artifactParams, _ := json.Marshal(map[string]any{"task_run_id": run.TaskRunID, "name": "result.json", "data": map[string]string{"private": "device-owned-artifact"}})
	artifactResponse, err := artifactPort.Call(ctx, metadata, "artifact-save", "task.artifact.saveData", artifactParams)
	if err != nil {
		t.Fatal(err)
	}
	if replay, err := artifactPort.Call(ctx, metadata, "artifact-save", "task.artifact.saveData", artifactParams); err != nil || string(replay) != string(artifactResponse) {
		t.Fatalf("实际设备端口未确认相同产物请求: %s %v", replay, err)
	}
	conflictingArtifact, _ := json.Marshal(map[string]any{"task_run_id": run.TaskRunID, "name": "result.json", "data": map[string]string{"private": "changed-artifact"}})
	if _, err := artifactPort.Call(ctx, metadata, "artifact-save", "task.artifact.saveData", conflictingArtifact); !errors.Is(err, coordination.ErrRequestConflict) {
		t.Fatalf("实际设备端口接受了相同请求编号的不同内容: %v", err)
	}
	var artifact struct {
		ArtifactID string `json:"artifactId"`
		Hash       string `json:"hash"`
	}
	if json.Unmarshal(artifactResponse, &artifact) != nil || artifact.ArtifactID == "" {
		t.Fatal("device artifact reference invalid")
	}
	if content, hash, err := artifactPort.Result(ctx, metadata, artifact.ArtifactID); err != nil || string(content) != `{"private":"device-owned-artifact"}` || hash != artifact.Hash {
		t.Fatalf("device artifact body changed: %s %v", content, err)
	}
	artifactRun := task_runtime.CloneTaskRun(metadata)
	artifactRun.Generation, artifactRun.ExecutionAttemptID = 2, "artifact-attempt"
	artifactResult := &task_runtime.TaskRunResult{TaskRunID: run.TaskRunID, ResultType: task_runtime.ResultArtifact, ArtifactID: artifact.ArtifactID, ResultHash: artifact.Hash}
	if err := outcomePort.SaveOutcome(ctx, artifactRun, "succeeded", artifactResult, "", ""); err != nil {
		t.Fatal(err)
	}
	if loaded, err := outcomePort.Result(ctx, artifactRun, artifactResult); err != nil || string(loaded.ResultJSON) != `{"private":"device-owned-artifact"}` || len(artifactResult.ResultJSON) != 0 {
		t.Fatalf("device artifact outcome copied or lost: %v", err)
	}
	snapshot, err := p.Snapshot(ctx, scope, coordination.DataQuery{})
	if err != nil {
		t.Fatal(err)
	}
	for _, resource := range snapshot.Resources {
		if resource.ID == "task/input/"+run.TaskRunID || resource.ID == "task/checkpoint/"+cp.CheckpointID || resource.ID == "task/storage/"+run.TaskRunID {
			t.Fatal("chat snapshot loaded unrelated private task body")
		}
	}
	core, err := coordination.NewOwnershipStore(db, "core").List(t.Context(), "checkpoint", "one", false)
	if err != nil || len(core) != 0 {
		t.Fatalf("device input mirrored into Core: %v", err)
	}
	if err := p.services.DB.Model(&character.Character{}).Where("id = ?", "one").Update("revision", 4).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := port.Input(ctx, metadata); !errors.Is(err, coordination.ErrScopeExpired) {
		t.Fatalf("old role restored device task input: %v", err)
	}
}
