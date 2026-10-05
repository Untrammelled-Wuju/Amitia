package task_runtime

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

type ownedOutcomeStore struct {
	completionFenceStore
	result *TaskRunResult
}

func (s *ownedOutcomeStore) PutResult(_ context.Context, result *TaskRunResult) error {
	copy := *result
	copy.ResultJSON = append(json.RawMessage(nil), result.ResultJSON...)
	s.result = &copy
	s.results++
	return nil
}

func (s *ownedOutcomeStore) GetResult(context.Context, string) (*TaskRunResult, error) {
	return s.result, nil
}

func TestOwnedTaskOutcomeConfirmsOwnerBeforeTerminalAndKeepsOnlyMetadata(t *testing.T) {
	for _, scenario := range []string{"succeeded", "failed", "cancelled", "timed_out", "manual_intervention", "wrong_ack"} {
		t.Run(scenario, func(t *testing.T) {
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
			store := &ownedOutcomeStore{completionFenceStore: completionFenceStore{run: CloneTaskRun(run)}}
			service.store = store
			service.config.OwnedOutcomes = AcknowledgedTaskOutcomePort{Data: data}
			status := scenario
			result := json.RawMessage("{\n  \"private\": \"owner result\"\n}")
			if scenario == "wrong_ack" {
				data.wrongAck, status = true, "succeeded"
			}
			if scenario == "cancelled" {
				store.run.Status, status = RunStatusCancelling, "succeeded"
			}
			service.handleFinished(ctx, run, status, result, "", "private-error-code", "private failure details")
			if scenario == "wrong_ack" {
				if store.run.Status != RunStatusRunning || store.result != nil {
					t.Fatal("unconfirmed outcome became terminal")
				}
				return
			}
			if !store.run.Status.IsTerminal() || data.resource.OwnerID != authority.ResourceOwnerID {
				t.Fatal("confirmed outcome was not committed")
			}
			if scenario == "succeeded" {
				if store.result == nil || len(store.result.ResultJSON) != 0 || store.result.ResultHash != hashBytes(result) {
					t.Fatalf("result body copied to ordinary task table: %+v", store.result)
				}
				loaded, err := service.GetResult(ctx, run.TaskRunID)
				if err != nil || string(loaded.ResultJSON) != string(result) || len(store.result.ResultJSON) != 0 {
					t.Fatalf("owner result bytes changed or mirrored: %+v %v", loaded, err)
				}
				changed := authority
				changed.PermissionRevision++
				if _, err := service.GetResult(coordination.WithScope(t.Context(), changed), run.TaskRunID); !IsTaskErrorCode(err, ErrTaskScopeDenied) {
					t.Fatalf("different authority read result: %v", err)
				}
			} else {
				if store.result != nil || store.run.ErrorMessage == nil || strings.Contains(*store.run.ErrorMessage, "private") || store.run.ErrorCode == nil || strings.Contains(*store.run.ErrorCode, "private") {
					t.Fatal("failure detail copied into ordinary task metadata")
				}
				var document ownedTaskOutcome
				if err := json.Unmarshal(data.resource.Body, &document); err != nil || document.Status != scenario {
					t.Fatalf("owner outcome changed: %+v %v", document, err)
				}
				if scenario == "failed" && document.ErrorMessage != "private failure details" {
					t.Fatal("owner failure details lost")
				}
				if scenario == "cancelled" && (document.Result != nil || len(document.Output) != 0) {
					t.Fatal("cancelled task saved late success result")
				}
			}
		})
	}
}
