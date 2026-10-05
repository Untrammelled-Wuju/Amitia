package task_runtime

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

type ownedCallbackStore struct {
	ownedOutcomeStore
	definition *TaskDefinition
	checkpoint *TaskCheckpoint
	progress   json.RawMessage
}

func (s *ownedCallbackStore) GetTaskDefinition(context.Context, string) (*TaskDefinition, error) {
	return s.definition, nil
}

func (s *ownedCallbackStore) GetLatestCheckpoint(context.Context, string) (*TaskCheckpoint, error) {
	return s.checkpoint, nil
}

func (s *ownedCallbackStore) PutCheckpoint(_ context.Context, cp *TaskCheckpoint) error {
	s.checkpoint = cp
	return nil
}

func (s *ownedCallbackStore) PutProgress(_ context.Context, _ string, _ int64, body []byte) error {
	s.progress = append(json.RawMessage(nil), body...)
	return nil
}

func TestOwnedCallbacksRestoreExactAuthorityAndRequireOwnerAcknowledgement(t *testing.T) {
	for _, kind := range []string{"external_finish", "remote_finish", "external_checkpoint", "remote_checkpoint", "external_progress", "remote_progress"} {
		for _, scenario := range []string{"valid", "wrong_ack", "expired", "replaced_attempt", "changed_definition", "different_scope", "changed_during_ack", "changed_installation"} {
			t.Run(kind+"/"+scenario, func(t *testing.T) {
				service, _, authority, run, definition := taskAuthorityFixture(t)
				run.TaskDefinitionID, run.Input = definition.TaskID, json.RawMessage(`{"private":"input"}`)
				run.DefinitionFingerprint, _ = taskDefinitionFingerprint(definition)
				run.InputHash = hashBytes(run.Input)
				run.Generation, run.ExecutionAttemptID, run.Status = 1, "attempt", RunStatusRunning
				data := &taskInputData{}
				if err := (AcknowledgedTaskInputPort{Data: data}).SaveInput(coordination.WithScope(t.Context(), authority), run); err != nil {
					t.Fatal(err)
				}
				run.Input = nil
				store := &ownedCallbackStore{ownedOutcomeStore: ownedOutcomeStore{completionFenceStore: completionFenceStore{run: CloneTaskRun(run)}}, definition: definition}
				service.store = store
				service.config.OwnedCheckpoints = AcknowledgedTaskCheckpointPort{Data: data}
				service.config.OwnedOutcomes = AcknowledgedTaskOutcomePort{Data: data}
				service.config.OwnedProgress = AcknowledgedTaskProgressPort{Data: data}
				service.config.OwnedExecutionGuard = func(ctx context.Context, scope coordination.ExecutionScope, _ *TaskRun) (context.Context, func(), error) {
					guarded := coordination.WithScope(ctx, scope)
					if scenario == "expired" {
						guarded = coordination.WithAdditionalGuard(guarded, func(context.Context) error { return coordination.ErrScopeExpired })
					}
					return guarded, func() {}, nil
				}
				ctx, attempt := t.Context(), "attempt"
				switch scenario {
				case "wrong_ack":
					data.wrongAck = true
				case "replaced_attempt":
					attempt = "old-attempt"
				case "changed_definition":
					definition.DefinitionHash = "changed"
				case "changed_during_ack":
					data.afterCommit = func() { definition.Entry = "changed-entry.cjs" }
				case "changed_installation":
					service.config.InstalledDefinitionValidator = func(context.Context, *TaskDefinition) error { return coordination.ErrScopeExpired }
				case "different_scope":
					changed := authority
					changed.RoleRevision++
					ctx = coordination.WithScope(ctx, changed)
				}
				payload := json.RawMessage(`{"private":"callback"}`)
				var err error
				switch kind {
				case "external_finish":
					err = service.HandleExternalFinish(ctx, run.TaskRunID, "succeeded", payload, "", "", "", attempt, 1)
				case "remote_finish":
					err = service.ApplyRemoteCompletion(ctx, run.TaskRunID, attempt, "", true, payload, "")
				case "external_checkpoint":
					err = service.HandleExternalCheckpoint(ctx, run.TaskRunID, 0, "", payload, attempt, 1)
				case "remote_checkpoint":
					err = service.HandleRemoteCheckpoint(ctx, run.TaskRunID, attempt, "remote-id", 1, payload, hashBytes(payload))
				case "external_progress":
					err = service.HandleExternalProgress(ctx, run.TaskRunID, 1, 2, "private", attempt, 1)
				case "remote_progress":
					err = service.HandleRemoteProgress(ctx, run.TaskRunID, attempt, 1, nil, nil, nil, "private", "private")
				}
				if scenario != "valid" {
					if err == nil || store.results != 0 || store.checkpoint != nil || len(store.progress) != 0 || store.run.Status != RunStatusRunning {
						t.Fatalf("unconfirmed callback persisted: %v", err)
					}
					return
				}
				if err != nil {
					t.Fatal(err)
				}
				if store.result != nil && len(store.result.ResultJSON) != 0 || store.checkpoint != nil && len(store.checkpoint.Payload) != 0 {
					t.Fatal("callback body copied into ordinary task tables")
				}
				var progress TaskRunProgress
				if len(store.progress) > 0 && (json.Unmarshal(store.progress, &progress) != nil || progress.Stage != "" || progress.Message != "") {
					t.Fatal("callback progress copied into ordinary task tables")
				}
			})
		}
	}
}
