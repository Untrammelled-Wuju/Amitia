package task_runtime

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

func TestOwnedTaskArtifactsConfirmPayloadAndKeepLargeOutcomeOnlyAtOwner(t *testing.T) {
	service, _, authority, run, definition := taskAuthorityFixture(t)
	run.TaskDefinitionID, run.Input = definition.TaskID, json.RawMessage(`{"private":"input"}`)
	run.DefinitionFingerprint, _ = taskDefinitionFingerprint(definition)
	run.InputHash = hashBytes(run.Input)
	run.Generation, run.ExecutionAttemptID, run.Status = 1, "attempt", RunStatusRunning
	ctx := coordination.WithScope(t.Context(), authority)
	data := &taskInputData{}
	if err := (AcknowledgedTaskInputPort{Data: data}).SaveInput(ctx, run); err != nil {
		t.Fatal(err)
	}
	run.Input = nil
	port := AcknowledgedTaskArtifactPort{Data: data}
	output, _ := json.Marshal(map[string]string{"private": strings.Repeat("中", 24000)})
	params, _ := json.Marshal(map[string]any{"task_run_id": run.TaskRunID, "name": "result.json", "data": json.RawMessage(output)})
	data.wrongAck = true
	if _, err := port.Call(ctx, run, "unconfirmed", "task.artifact.saveData", params); err == nil {
		t.Fatal("unconfirmed artifact reported saved")
	}
	data.wrongAck = false
	reconciled, err := port.Call(ctx, run, "unconfirmed", "task.artifact.saveData", params)
	if err != nil {
		t.Fatalf("所有者已保存但确认丢失的产物不能恢复: %v", err)
	}
	indexBefore := data.resources["checkpoint/task/artifacts/"+run.TaskRunID].Revision
	if replay, err := port.Call(ctx, run, "unconfirmed", "task.artifact.saveData", params); err != nil || string(replay) != string(reconciled) || data.resources["checkpoint/task/artifacts/"+run.TaskRunID].Revision != indexBefore {
		t.Fatalf("重复产物请求再次写入或返回不同引用: %v", err)
	}
	conflicting, _ := json.Marshal(map[string]any{"task_run_id": run.TaskRunID, "name": "result.json", "data": map[string]string{"private": "different"}})
	if _, err := port.Call(ctx, run, "unconfirmed", "task.artifact.saveData", conflicting); !errors.Is(err, coordination.ErrRequestConflict) {
		t.Fatalf("同一请求编号覆盖了已保存产物: %v", err)
	}
	var recovered ownedTaskArtifactMetadata
	if json.Unmarshal(reconciled, &recovered) != nil {
		t.Fatal("产物恢复引用无效")
	}
	stored := data.resources["checkpoint/task/artifact/"+recovered.ArtifactID]
	originalBody := append(json.RawMessage(nil), stored.Body...)
	stored.Body = json.RawMessage(`{}`)
	if _, err := port.Call(ctx, run, "unconfirmed", "task.artifact.saveData", params); !errors.Is(err, coordination.ErrRequestConflict) {
		t.Fatalf("仅凭索引确认了损坏的产物: %v", err)
	}
	stored.Body = originalBody
	stored.Deleted = true
	if _, err := port.Call(ctx, run, "unconfirmed", "task.artifact.saveData", params); !errors.Is(err, coordination.ErrRequestConflict) {
		t.Fatalf("已删除产物被重复请求恢复: %v", err)
	}
	stored.Deleted = false
	response, err := port.Call(ctx, run, "confirmed", "task.artifact.saveData", params)
	if err != nil {
		t.Fatal(err)
	}
	var artifact ownedTaskArtifactMetadata
	if json.Unmarshal(response, &artifact) != nil || artifact.Size != len(output) || artifact.Hash != hashBytes(output) {
		t.Fatal("artifact confirmation reference changed")
	}
	if content, hash, err := port.Result(ctx, run, artifact.ArtifactID); err != nil || string(content) != string(output) || hash != artifact.Hash {
		t.Fatalf("owner artifact result changed: %v", err)
	}
	changed := authority
	changed.PermissionRevision++
	if _, _, err := port.Result(coordination.WithScope(t.Context(), changed), run, artifact.ArtifactID); !IsTaskErrorCode(err, ErrTaskScopeDenied) {
		t.Fatalf("changed authority reused artifact: %v", err)
	}
	listParams, _ := json.Marshal(map[string]string{"task_run_id": run.TaskRunID})
	if listed, err := port.Call(ctx, run, "list", "task.artifact.list", listParams); err != nil || !strings.Contains(string(listed), artifact.ArtifactID) || strings.Contains(string(listed), "中") {
		t.Fatalf("artifact list lost reference or copied body: %s %v", listed, err)
	}
	store := &ownedOutcomeStore{completionFenceStore: completionFenceStore{run: CloneTaskRun(run)}}
	service.store = store
	service.config.OwnedArtifacts = port
	service.config.OwnedOutcomes = AcknowledgedTaskOutcomePort{Data: data}
	if err := service.handleFinished(ctx, run, "succeeded", nil, artifact.ArtifactID, "", ""); err != nil {
		t.Fatal(err)
	}
	if store.run.Status != RunStatusSucceeded || store.result == nil || store.result.ResultType != ResultArtifact || store.result.ArtifactID != artifact.ArtifactID || len(store.result.ResultJSON) != 0 || store.result.ResultHash != artifact.Hash {
		t.Fatalf("large artifact copied into ordinary task result: %+v", store.result)
	}
	result, err := service.GetResult(ctx, run.TaskRunID)
	if err != nil || string(result.ResultJSON) != string(output) || len(store.result.ResultJSON) != 0 {
		t.Fatalf("owner artifact output not hydrated in memory: %v", err)
	}
	var document ownedTaskOutcome
	if json.Unmarshal(data.resource.Body, &document) != nil || len(document.Output) != 0 || document.Result == nil || len(document.Result.ResultJSON) != 0 {
		t.Fatal("artifact body copied into outcome reference")
	}
	data.resources["checkpoint/task/artifact/"+artifact.ArtifactID].Deleted = true
	if _, err := service.GetResult(ctx, run.TaskRunID); err == nil {
		t.Fatal("deleted artifact returned from a cached body")
	}
}
