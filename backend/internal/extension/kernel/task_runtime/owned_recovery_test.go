package task_runtime

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

func TestOwnedRestartAndLeaseExpiryNeverRestartUnknownWork(t *testing.T) {
	for _, method := range []string{"startup", "lease"} {
		t.Run(method, func(t *testing.T) {
			service, _, _, run, definition := taskAuthorityFixture(t)
			run.Status, run.Generation, run.ExecutionAttemptID = RunStatusRunning, 2, "attempt"
			expired := time.Now().Add(-time.Minute)
			run.LeaseExpiresAt = &expired
			definition.Idempotent, definition.Recoverability = true, RestartableFromBeginning
			store := &ownedCallbackStore{ownedOutcomeStore: ownedOutcomeStore{completionFenceStore: completionFenceStore{run: CloneTaskRun(run)}}, definition: definition}
			service.store = store
			var err error
			if method == "startup" {
				err = service.recoverRun(t.Context(), run)
			} else {
				err = service.recoverStaleTask(t.Context(), run)
			}
			if err != nil || store.run.Status != RunStatusRecoveryRequired || store.run.FinishedAt != nil || store.results != 0 || store.run.Generation != 2 {
				t.Fatalf("unknown device task restarted or marked completed: %+v %v", store.run, err)
			}
		})
	}
}

func TestOwnedExecutorOutcomeUsesOwnerConfirmationAndCompletionCanBeAcknowledgedAgain(t *testing.T) {
	service, _, authority, run, definition := taskAuthorityFixture(t)
	run.TaskDefinitionID, run.Input = definition.TaskID, json.RawMessage(`{"private":"input"}`)
	run.DefinitionFingerprint, _ = taskDefinitionFingerprint(definition)
	run.InputHash = hashBytes(run.Input)
	run.Status, run.ExecutionAttemptID, run.Generation = RunStatusRunning, "attempt", 1
	data := &taskInputData{}
	ctx := coordination.WithScope(t.Context(), authority)
	if err := (AcknowledgedTaskInputPort{Data: data}).SaveInput(ctx, run); err != nil {
		t.Fatal(err)
	}
	run.Input = nil
	store := &ownedCallbackStore{ownedOutcomeStore: ownedOutcomeStore{completionFenceStore: completionFenceStore{run: CloneTaskRun(run)}}, definition: definition}
	service.store = store
	service.config.OwnedOutcomes = AcknowledgedTaskOutcomePort{Data: data}
	service.config.OwnedExecutionGuard = func(ctx context.Context, scope coordination.ExecutionScope, _ *TaskRun) (context.Context, func(), error) {
		return coordination.WithScope(ctx, scope), func() {}, nil
	}
	output := json.RawMessage(`{"private":"output"}`)
	outcome := TaskExecutionOutcome{Status: RunStatusSucceeded, Result: &TaskRunResult{TaskRunID: run.TaskRunID, ResultType: ResultInlineJSON, ResultJSON: output, ResultHash: hashBytes(output)}}
	data.wrongAck = true
	service.applyExecutionOutcome(ctx, run, definition, outcome)
	if store.run.Status != RunStatusRunning || store.result != nil {
		t.Fatal("executor bypassed owner acknowledgement")
	}
	data.wrongAck = false
	service.applyExecutionOutcome(ctx, run, definition, outcome)
	if store.run.Status != RunStatusSucceeded || store.result == nil || len(store.result.ResultJSON) != 0 {
		t.Fatal("executor did not use owner outcome storage")
	}
	if err := service.ApplyRemoteCompletion(t.Context(), run.TaskRunID, "attempt", "", true, output, ""); err != nil || store.results != 1 {
		t.Fatalf("confirmed completion could not retry authority acknowledgement: %v", err)
	}
	if err := service.ApplyRemoteCompletion(t.Context(), run.TaskRunID, "attempt", "", true, json.RawMessage(`{"different":true}`), ""); err == nil || store.results != 1 {
		t.Fatal("different completion borrowed confirmed outcome")
	}
}

func TestRemoteUnconfirmedReceiptPreservesUnknownAndDoesNotSaveFailedOutcome(t *testing.T) {
	for _, attempt := range []string{"attempt", "foreign"} {
		t.Run(attempt, func(t *testing.T) {
			service, _, _, run, definition := taskAuthorityFixture(t)
			run.TaskDefinitionID = definition.TaskID
			run.DefinitionFingerprint, _ = taskDefinitionFingerprint(definition)
			run.Status, run.Generation, run.ExecutionAttemptID, run.LeaseID = RunStatusRunning, 2, "attempt", "lease"
			run.ExecutionPlacement = TaskExecutionPlacementDevice
			store := &ownedCallbackStore{ownedOutcomeStore: ownedOutcomeStore{completionFenceStore: completionFenceStore{run: CloneTaskRun(run)}}, definition: definition}
			service.store = store
			service.config.OwnedExecutionGuard = func(ctx context.Context, authority coordination.ExecutionScope, _ *TaskRun) (context.Context, func(), error) {
				return coordination.WithScope(ctx, authority), func() {}, nil
			}
			err := service.HandleRemoteUnconfirmed(t.Context(), run.TaskRunID, attempt, "lease")
			if attempt == "attempt" && (err != nil || store.run.Status != RunStatusRecoveryRequired || store.run.FinishedAt != nil || store.results != 0) || attempt == "foreign" && (err == nil || store.run.Status != RunStatusRunning) {
				t.Fatalf("unknown receipt changed terminal state: %+v %v", store.run, err)
			}
		})
	}
}

func TestRemoteArtifactCompletionUsesOwnerReferenceAndRevalidatesRetries(t *testing.T) {
	service, _, authority, run, definition := taskAuthorityFixture(t)
	run.TaskDefinitionID, run.Generation, run.ExecutionAttemptID, run.LeaseID, run.Status = definition.TaskID, 2, "attempt", "lease", RunStatusRunning
	run.DefinitionFingerprint, _ = taskDefinitionFingerprint(definition)
	run.Input = json.RawMessage(`{"private":"input"}`)
	run.InputHash = hashBytes(run.Input)
	ctx := coordination.WithScope(t.Context(), authority)
	data := &taskInputData{}
	if err := (AcknowledgedTaskInputPort{Data: data}).SaveInput(ctx, run); err != nil {
		t.Fatal(err)
	}
	run.Input = nil
	port := AcknowledgedTaskArtifactPort{Data: data}
	output, _ := json.Marshal(map[string]string{"private": strings.Repeat("中", 24000)})
	params, _ := json.Marshal(map[string]any{"task_run_id": run.TaskRunID, "name": "result.json", "data": json.RawMessage(output)})
	response, err := port.Call(ctx, run, "source-artifact", "task.artifact.saveData", params)
	if err != nil {
		t.Fatal(err)
	}
	var artifact ownedTaskArtifactMetadata
	if err := json.Unmarshal(response, &artifact); err != nil {
		t.Fatal(err)
	}
	store := &ownedCallbackStore{ownedOutcomeStore: ownedOutcomeStore{completionFenceStore: completionFenceStore{run: CloneTaskRun(run)}}, definition: definition}
	service.store = store
	service.config.OwnedArtifacts, service.config.OwnedOutcomes = port, AcknowledgedTaskOutcomePort{Data: data}
	service.config.OwnedExecutionGuard = func(ctx context.Context, authority coordination.ExecutionScope, _ *TaskRun) (context.Context, func(), error) {
		return coordination.WithScope(ctx, authority), func() {}, nil
	}
	if err := service.ApplyRemoteCompletion(t.Context(), run.TaskRunID, "attempt", "lease", true, nil, "", artifact.ArtifactID); err != nil {
		t.Fatal(err)
	}
	if store.run.Status != RunStatusSucceeded || store.result == nil || store.result.ArtifactID != artifact.ArtifactID || len(store.result.ResultJSON) != 0 || store.result.ResultHash != hashBytes(output) {
		t.Fatalf("remote artifact was mirrored or lost: %+v", store.result)
	}
	if err := service.ApplyRemoteCompletion(t.Context(), run.TaskRunID, "attempt", "lease", true, nil, "", artifact.ArtifactID); err != nil || store.results != 1 {
		t.Fatalf("artifact confirmation retry failed: %v", err)
	}
	data.resources["checkpoint/task/artifact/"+artifact.ArtifactID].Deleted = true
	if err := service.ApplyRemoteCompletion(t.Context(), run.TaskRunID, "attempt", "lease", true, nil, "", artifact.ArtifactID); err == nil {
		t.Fatal("deleted artifact accepted on completion retry")
	}
}
