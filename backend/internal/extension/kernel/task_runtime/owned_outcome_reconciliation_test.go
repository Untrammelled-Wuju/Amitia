package task_runtime

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

func ownedCommittedOutcomeFixture(t *testing.T) (*TaskRuntimeService, *taskInputData, coordination.ExecutionScope, *TaskRun, *TaskDefinition) {
	t.Helper()
	service, _, authority, run, def := taskAuthorityFixture(t)
	run.TaskDefinitionID = def.TaskID
	run.DefinitionFingerprint, _ = taskDefinitionFingerprint(def)
	run.Input = json.RawMessage(`{"private":"input"}`)
	run.InputHash = hashBytes(run.Input)
	run.ExecutionAttemptID, run.Generation = "original-attempt", 3
	run.Status = RunStatusRecoveryRequired
	data := &taskInputData{}
	if err := (AcknowledgedTaskInputPort{Data: data}).SaveInput(coordination.WithScope(t.Context(), authority), run); err != nil {
		t.Fatal(err)
	}
	run.Input = nil
	service.config.OwnedOutcomes = AcknowledgedTaskOutcomePort{Data: data}
	service.config.OwnedExecutionGuard = func(ctx context.Context, scope coordination.ExecutionScope, _ *TaskRun) (context.Context, func(), error) {
		return coordination.WithScope(ctx, scope), func() {}, nil
	}
	return service, data, authority, run, def
}

func TestOwnerOutcomeReplayDoesNotCommitSecondTerminalResource(t *testing.T) {
	for _, status := range []string{"succeeded", "failed"} {
		t.Run(status, func(t *testing.T) {
			service, data, authority, run, _ := ownedCommittedOutcomeFixture(t)
			port := service.config.OwnedOutcomes.(AcknowledgedTaskOutcomePort)
			commits := 0
			data.afterCommit = func() { commits++ }
			var first, replay *TaskRunResult
			if status == "succeeded" {
				output := json.RawMessage(`{"answer":42}`)
				first = &TaskRunResult{TaskRunID: run.TaskRunID, ResultType: ResultInlineJSON, ResultJSON: output, ResultHash: hashBytes(output), CreatedAt: time.Now().Add(-time.Minute).UTC()}
				copied := *first
				copied.CreatedAt = time.Now().UTC()
				replay = &copied
			}
			ctx := coordination.WithScope(t.Context(), authority)
			if err := port.SaveOutcome(ctx, run, status, first, "original", "original-details"); err != nil {
				t.Fatal(err)
			}
			if err := port.SaveOutcome(ctx, run, status, replay, "original", "original-details"); err != nil {
				t.Fatalf("confirmed same outcome must be replayable: %v", err)
			}
			if commits != 1 {
				t.Fatalf("replayed Owner terminal mutation: %d", commits)
			}
			inspected, err := port.InspectOutcome(ctx, run)
			if err != nil || inspected == nil || inspected.Status != status {
				t.Fatalf("confirmed outcome inspection failed: %+v %v", inspected, err)
			}
			if inspected.Result != nil && len(inspected.Result.ResultJSON) != 0 {
				t.Fatal("private result leaked through inspection")
			}
			if status == "succeeded" {
				wrong := *replay
				wrong.ResultJSON = json.RawMessage(`{"answer":43}`)
				wrong.ResultHash = hashBytes(wrong.ResultJSON)
				if err := port.SaveOutcome(ctx, run, status, &wrong, "original", "original-details"); err == nil {
					t.Fatal("different outcome used confirmed identity")
				}
			} else if err := port.SaveOutcome(ctx, run, status, nil, "different", "original-details"); err == nil {
				t.Fatal("different error borrowed confirmed terminal outcome")
			}
			if commits != 1 {
				t.Fatal("conflicting retry mutated Owner")
			}
		})
	}
}

func TestOwnerOutcomeInspectionRejectsChangedAuthorityOrTamper(t *testing.T) {
	service, data, authority, run, _ := ownedCommittedOutcomeFixture(t)
	port := service.config.OwnedOutcomes.(AcknowledgedTaskOutcomePort)
	ctx := coordination.WithScope(t.Context(), authority)
	if out, err := port.InspectOutcome(ctx, run); err != nil || out != nil {
		t.Fatalf("no terminal outcome must be absent: %v %+v", err, out)
	}
	if err := port.SaveOutcome(ctx, run, "failed", nil, "code", "private-detail"); err != nil {
		t.Fatal(err)
	}
	changed := authority
	changed.PermissionRevision++
	if _, err := port.InspectOutcome(coordination.WithScope(t.Context(), changed), run); err == nil {
		t.Fatal("changed Owner authorization read terminal")
	}
	resource := data.resources["checkpoint/"+taskOutcomeID(run)]
	var document ownedTaskOutcome
	if err := json.Unmarshal(resource.Body, &document); err != nil {
		t.Fatal(err)
	}
	document.Status = "succeeded"
	resource.Body, _ = json.Marshal(document)
	if _, err := port.InspectOutcome(ctx, run); err == nil {
		t.Fatal("tampered terminal resource accepted")
	}
}

func TestRecoverReconcilesCommittedOwnerOutcomeWithoutNewGeneration(t *testing.T) {
	for _, status := range []string{"succeeded", "failed", "cancelled", "timed_out"} {
		t.Run(status, func(t *testing.T) {
			service, _, authority, run, def := ownedCommittedOutcomeFixture(t)
			ctx := coordination.WithScope(t.Context(), authority)
			store := &ownedCallbackStore{ownedOutcomeStore: ownedOutcomeStore{completionFenceStore: completionFenceStore{run: CloneTaskRun(run)}}, definition: def}
			service.store = store
			port := service.config.OwnedOutcomes.(AcknowledgedTaskOutcomePort)
			var output *TaskRunResult
			if status == "succeeded" {
				body := json.RawMessage(`{"private":"result"}`)
				output = &TaskRunResult{TaskRunID: run.TaskRunID, ResultType: ResultInlineJSON, ResultJSON: body, ResultHash: hashBytes(body), CreatedAt: time.Now().Add(-time.Minute).UTC()}
			}
			if err := port.SaveOutcome(ctx, run, status, output, "failure-code", "private details"); err != nil {
				t.Fatal(err)
			}
			recovered, err := service.Recover(t.Context(), run.TaskRunID)
			if err != nil {
				t.Fatal(err)
			}
			if recovered.Generation != 3 || recovered.ExecutionAttemptID != "original-attempt" || recovered.Status != TaskRunStatus(status) || store.run.Status != TaskRunStatus(status) || store.casCalls != 1 {
				t.Fatalf("Owner outcome not reconciled: %+v %+v", recovered, store.run)
			}
			if status == "succeeded" {
				if store.result == nil || len(store.result.ResultJSON) != 0 || store.result.ResultHash != output.ResultHash {
					t.Fatal("private bytes leaked or owner metadata lost")
				}
			} else if store.result != nil || recovered.ErrorMessage == nil || *recovered.ErrorMessage == "private details" {
				t.Fatal("private failure outcome leaked")
			}
			if _, err := service.Recover(t.Context(), run.TaskRunID); err == nil || store.casCalls != 1 {
				t.Fatal("terminal was reconciled twice")
			}
		})
	}
}

func TestRecoverRejectsUntrustedOwnerOutcomeWithoutRequeue(t *testing.T) {
	service, data, authority, run, def := ownedCommittedOutcomeFixture(t)
	ctx := coordination.WithScope(t.Context(), authority)
	store := &ownedCallbackStore{ownedOutcomeStore: ownedOutcomeStore{completionFenceStore: completionFenceStore{run: CloneTaskRun(run)}}, definition: def}
	service.store = store
	port := service.config.OwnedOutcomes.(AcknowledgedTaskOutcomePort)
	if err := port.SaveOutcome(ctx, run, "failed", nil, "original", "detail"); err != nil {
		t.Fatal(err)
	}
	record := data.resources["checkpoint/"+taskOutcomeID(run)]
	record.Revision = 99
	if _, err := service.Recover(t.Context(), run.TaskRunID); err == nil || store.casCalls != 0 || store.run.Generation != run.Generation {
		t.Fatalf("corrupted Owner outcome triggered recover: %v", err)
	}
}
