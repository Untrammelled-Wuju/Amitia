package task_runtime

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

func TestLegacyOwnerOutcomeIsReadOnlyAndRequiresCompletedLocalProof(t *testing.T) {
	_, _, scope, run, definition := taskAuthorityFixture(t)
	run.TaskDefinitionID = definition.TaskID
	run.DefinitionFingerprint, _ = taskDefinitionFingerprint(definition)
	run.Generation, run.ExecutionAttemptID = 1, "legacy-attempt"
	run.Input = json.RawMessage(`{"input":"original"}`)
	run.InputHash = hashBytes(run.Input)
	ctx := coordination.WithScope(t.Context(), scope)
	data := &taskInputData{}
	if err := (AcknowledgedTaskInputPort{Data: data}).SaveInput(ctx, run); err != nil {
		t.Fatal(err)
	}
	body := json.RawMessage(`{"result":"historical"}`)
	metadata := &TaskRunResult{TaskRunID: run.TaskRunID, ResultType: ResultInlineJSON, ResultHash: hashBytes(body)}
	document := ownedTaskOutcome{Scope: scope, DefinitionFingerprint: run.DefinitionFingerprint, TaskRunID: run.TaskRunID, Generation: run.Generation, AttemptID: run.ExecutionAttemptID, Status: "succeeded", Result: metadata, Output: body}
	id := taskOutcomeID(run)
	encoded, err := json.Marshal(document)
	if err != nil {
		t.Fatal(err)
	}
	resource := &coordination.Resource{OwnerID: scope.ResourceOwnerID, RoleID: scope.RoleID, Kind: "checkpoint", ID: id, Revision: 1, Body: encoded}
	data.resources["checkpoint/"+id] = resource
	port := AcknowledgedTaskOutcomePort{Data: data}
	if _, err := port.Result(ctx, run, metadata); err == nil {
		t.Fatal("unprojected legacy outcome was readable")
	}
	finished := time.Now().UTC()
	run.Status, run.FinishedAt = RunStatusSucceeded, &finished
	result, err := port.Result(ctx, run, metadata)
	if err != nil || string(result.ResultJSON) != string(body) {
		t.Fatalf("confirmed legacy result became unreadable: %+v %v", result, err)
	}
	if _, err := port.InspectOutcome(ctx, run); err == nil {
		t.Fatal("legacy read unlocked automated recovery")
	}
	if err := port.SaveOutcome(ctx, run, "succeeded", result, "", ""); err == nil {
		t.Fatal("legacy result unlocked commit replay")
	}
	forged := *metadata
	forged.ResultHash = hashBytes([]byte("forged"))
	if _, err := port.Result(ctx, run, &forged); err == nil {
		t.Fatal("forged local metadata exposed old result")
	}
	document.CommitProtocolVersion = 1
	resource.Body, _ = json.Marshal(document)
	if _, err := port.Result(ctx, run, metadata); err == nil {
		t.Fatal("new cursorless outcome used legacy exception")
	}
	document.CommitProtocolVersion = 2
	resource.Body, _ = json.Marshal(document)
	if _, err := port.Result(ctx, run, metadata); err == nil {
		t.Fatal("unknown commit protocol accepted")
	}
}
