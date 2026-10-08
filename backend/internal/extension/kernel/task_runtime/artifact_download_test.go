package task_runtime

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"testing"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

func TestOwnedTaskArtifactDownloadKeepsBinaryAtOwnerAndRejectsCorruption(t *testing.T) {
	_, _, authority, run, definition := taskAuthorityFixture(t)
	run.TaskDefinitionID, run.Input = definition.TaskID, json.RawMessage(`{"private":"input"}`)
	run.DefinitionFingerprint, _ = taskDefinitionFingerprint(definition)
	run.InputHash = hashBytes(run.Input)
	run.Generation, run.ExecutionAttemptID = 1, "attempt"
	ctx := coordination.WithScope(t.Context(), authority)
	data := &taskInputData{}
	if err := (AcknowledgedTaskInputPort{Data: data}).SaveInput(ctx, run); err != nil {
		t.Fatal(err)
	}
	run.Input = nil
	port := AcknowledgedTaskArtifactPort{Data: data}
	content := []byte{0, 255, 7, 13, 10, 128}
	params, _ := json.Marshal(map[string]any{"task_run_id": run.TaskRunID, "name": "私有结果.bin", "content": base64.StdEncoding.EncodeToString(content), "options": map[string]string{"mimeType": "application/octet-stream"}})
	response, err := port.Call(ctx, run, "binary", "task.artifact.saveFile", params)
	if err != nil {
		t.Fatal(err)
	}
	var metadata ownedTaskArtifactMetadata
	if json.Unmarshal(response, &metadata) != nil {
		t.Fatal("产物确认无效")
	}
	download, err := port.Download(ctx, run, metadata.ArtifactID)
	if err != nil || download == nil || !bytes.Equal(download.Content, content) || download.Hash != hashBytes(content) || download.Name != "私有结果.bin" || download.MimeType != "application/octet-stream" {
		t.Fatalf("二进制产物下载改变了所有者数据: %+v %v", download, err)
	}
	if _, _, err := port.Result(ctx, run, metadata.ArtifactID); err == nil {
		t.Fatal("任意二进制被当作 JSON 任务成功结果")
	}
	resource := data.resources["checkpoint/task/artifact/"+metadata.ArtifactID]
	original := append(json.RawMessage(nil), resource.Body...)
	var document ownedTaskArtifactDocument
	if json.Unmarshal(resource.Body, &document) != nil {
		t.Fatal("产物文档无效")
	}
	document.Content[0] = 1
	resource.Body, _ = json.Marshal(document)
	if _, err := port.Download(ctx, run, metadata.ArtifactID); err == nil {
		t.Fatal("产物正文损坏后仍可以下载")
	}
	resource.Body = original
	resource.Deleted = true
	if _, err := port.Download(ctx, run, metadata.ArtifactID); err == nil {
		t.Fatal("被删除的产物仍可以下载")
	}
	resource.Deleted = false
	foreign := CloneTaskRun(run)
	foreign.TaskRunID = "other-task"
	if _, err := port.Download(ctx, foreign, metadata.ArtifactID); err == nil {
		t.Fatal("其他任务可以使用此产物编号下载")
	}
}
