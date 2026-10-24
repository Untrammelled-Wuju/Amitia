package task_runtime

import (
	"context"
	"encoding/json"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

func round16PendingCheckpointFixture(t *testing.T) (*TaskRuntimeService, context.Context, *taskInputData, *checkpointFenceStore, *TaskRun, *TaskDefinition, *TaskCheckpoint) {
	t.Helper()
	svc, _, scope, run, def := taskAuthorityFixture(t)
	def.DefinitionHash = "def-16"
	def.Checkpoint, def.Recoverability, def.Idempotency = true, CheckpointRecoverable, Idempotent
	run.TaskDefinitionID = def.TaskID
	run.DefinitionFingerprint, _ = taskDefinitionFingerprint(def)
	run.Status, run.Generation, run.Attempt, run.ExecutionAttemptID = RunStatusRecoveryRequired, 2, 1, "original-attempt"
	run.Input = json.RawMessage(`{"source":"owner"}`)
	run.InputHash = hashBytes(run.Input)
	ctx := coordination.WithScope(t.Context(), scope)
	data := &taskInputData{}
	if err := (AcknowledgedTaskInputPort{Data: data}).SaveInput(ctx, run); err != nil {
		t.Fatal(err)
	}
	run.Input = nil
	svc.config.OwnedCheckpoints = AcknowledgedTaskCheckpointPort{Data: data}
	store := &checkpointFenceStore{completionFenceStore: completionFenceStore{run: CloneTaskRun(run)}}
	svc.store = store
	body := json.RawMessage(`{"cursor":1,"data":{"step":"staged"}}`)
	cp := &TaskCheckpoint{CheckpointID: ownedStableCheckpointID(run, 1), TaskRunID: run.TaskRunID, Version: 1,
		Payload: body, PayloadHash: hashBytes(body), DefinitionHash: def.DefinitionHash,
		InputHash: run.InputHash, CreatedAt: time.Now().UTC()}

	if err := svc.config.OwnedCheckpoints.SaveCheckpoint(ctx, run, cp); err != nil {
		t.Fatal(err)
	}
	return svc, ctx, data, store, run, def, cp
}

func TestRound16UnprojectedOwnerCheckpointIsRecoveredWithoutDuplicateOwnerCommit(t *testing.T) {
	svc, ctx, data, store, run, def, cp := round16PendingCheckpointFixture(t)
	beforeCommits := 0
	data.afterCommit = func() { beforeCommits++ }
	updated, confirmed, err := svc.reconcilePendingOwnerCheckpoint(ctx, run, def, nil)
	if err != nil {
		t.Fatal(err)
	}
	if confirmed == nil || confirmed.CheckpointID != cp.CheckpointID || string(confirmed.Payload) != string(cp.Payload) {
		t.Fatal("Owner original checkpoint was not verified")
	}
	if store.run.CheckpointID == nil || *store.run.CheckpointID != cp.CheckpointID ||
		store.checkpoint == nil || store.checkpoint.Payload != nil || store.casCalls != 1 || beforeCommits != 0 {
		t.Fatal("local metadata was not projected exactly once without leaking private bytes")
	}
	if updated.Revision != store.run.Revision || updated.Generation != run.Generation || updated.ExecutionAttemptID != run.ExecutionAttemptID {
		t.Fatal("projection unexpectedly changed execution identity")
	}
	latest, err := svc.readTaskCheckpoint(ctx, updated)
	if err != nil || latest == nil || latest.PayloadHash != cp.PayloadHash {
		t.Fatalf("projected checkpoint could not be read: %v", err)
	}
	again, _, err := svc.reconcilePendingOwnerCheckpoint(ctx, updated, def, latest)
	if err != nil || again.Revision != updated.Revision || store.casCalls != 1 || beforeCommits != 0 {
		t.Fatalf("idempotent re-inspection changed state: %v", err)
	}
}

func TestRound16PendingOwnerCheckpointRejectsTamperingAndConcurrentCAS(t *testing.T) {
	for _, mode := range []string{"tampered_payload", "wrong_attempt", "wrong_owner", "concurrent"} {
		t.Run(mode, func(t *testing.T) {
			svc, ctx, data, store, run, def, cp := round16PendingCheckpointFixture(t)
			resource := data.resources["checkpoint/task/checkpoint/"+cp.CheckpointID]
			switch mode {
			case "tampered_payload":
				resource.Body = json.RawMessage(`{"checkpoint":{"checkpointId":"fake"}}`)
			case "wrong_attempt":
				var rec ownedTaskCheckpoint
				if err := json.Unmarshal(resource.Body, &rec); err != nil {
					t.Fatal(err)
				}
				rec.ExecutionAttemptID = "foreign-attempt"
				resource.Body, _ = json.Marshal(rec)
			case "wrong_owner":
				resource.OwnerID = "foreign-owner"
			case "concurrent":
				store.reject = true
			}
			_, _, err := svc.reconcilePendingOwnerCheckpoint(ctx, run, def, nil)
			if err == nil || store.checkpoint != nil || store.run.CheckpointID != nil {
				t.Fatalf("unsafe pending checkpoint was projected (%s): %v", mode, err)
			}
		})
	}
}

type round16CheckpointReadErrorStore struct {
	checkpointFenceStore
	checkpointReadErr error
}

func (s *round16CheckpointReadErrorStore) GetLatestCheckpoint(ctx context.Context, id string) (*TaskCheckpoint, error) {
	if s.checkpointReadErr != nil {
		return nil, s.checkpointReadErr
	}
	return s.checkpointFenceStore.GetLatestCheckpoint(ctx, id)
}

func TestRound16CrashCheckpointUntrustedDoesNotBecomeRecoverable(t *testing.T) {
	for _, mode := range []string{"io_error", "tampered", "missing_record", "valid"} {
		t.Run(mode, func(t *testing.T) {
			svc, _, _, run, def := taskAuthorityFixture(t)
			run.ScopeSnapshotID = ""
			run.Status, run.Generation, run.ExecutionAttemptID = RunStatusRunning, 1, "local-attempt"
			def.DefinitionHash = "def-16"
			def.Recoverability, def.Checkpoint = CheckpointRecoverable, true
			run.InputHash = hashBytes([]byte(`{}`))
			body := json.RawMessage(`{"cursor":1,"ok":true}`)
			cp := &TaskCheckpoint{CheckpointID: "local-checkpoint", TaskRunID: run.TaskRunID, Version: 1, Payload: body,
				PayloadHash: hashBytes(body), DefinitionHash: def.DefinitionHash, InputHash: run.InputHash}
			id := cp.CheckpointID
			run.CheckpointID = &id
			store := &round16CheckpointReadErrorStore{checkpointFenceStore: checkpointFenceStore{completionFenceStore: completionFenceStore{run: CloneTaskRun(run)}}}
			svc.store = store
			switch mode {
			case "io_error":
				store.checkpointReadErr = errors.New("disk read failed")
			case "tampered":
				cp.PayloadHash = "tampered"
				store.checkpoint = cp
			case "missing_record":
			case "valid":
				store.checkpoint = cp
			}
			svc.handleCrash(t.Context(), run, def, 1)
			expected := RunStatusManualIntervention
			if mode == "valid" {
				expected = RunStatusRecoveryRequired
			}
			if store.run.Status != expected {
				t.Fatalf("%s: got %s, want %s", mode, store.run.Status, expected)
			}
		})
	}
}

func TestRound16NextOwnerCheckpointAfterExistingProjection(t *testing.T) {
	svc, ctx, _, store, run, def, first := round16PendingCheckpointFixture(t)

	firstMetadata := *first
	firstMetadata.Payload = nil
	store.checkpoint = &firstMetadata
	id := first.CheckpointID
	run.CheckpointID = &id
	store.run.CheckpointID = &id
	nextBody := json.RawMessage(`{"cursor":2,"data":{"step":"next"}}`)
	second := &TaskCheckpoint{CheckpointID: ownedStableCheckpointID(run, 2), TaskRunID: run.TaskRunID,
		Version: 2, Payload: nextBody, PayloadHash: hashBytes(nextBody), DefinitionHash: def.DefinitionHash,
		InputHash: run.InputHash, CreatedAt: time.Now().UTC()}
	if err := svc.config.OwnedCheckpoints.SaveCheckpoint(ctx, run, second); err != nil {
		t.Fatal(err)
	}

	existing, err := svc.readTaskCheckpoint(ctx, run)
	if err != nil {
		t.Fatal(err)
	}
	next, recovered, err := svc.reconcilePendingOwnerCheckpoint(ctx, run, def, existing)
	if err != nil || recovered == nil || recovered.Version != 2 || next.CheckpointID == nil || *next.CheckpointID != second.CheckpointID {
		t.Fatalf("checkpoint N+1 was not reconciled: %v", err)
	}
	if store.checkpoint == nil || len(store.checkpoint.Payload) != 0 || store.checkpoint.Version != 2 || store.casCalls != 1 {
		t.Fatal("new Owner checkpoint was not safely projected")
	}
}

func TestRound16RecoverFirstOwnerCheckpointCommittedBeforeCoreCAS(t *testing.T) {
	svc, _, data, _, run, def, original := round16PendingCheckpointFixture(t)
	svc.config.OwnedOutcomes = AcknowledgedTaskOutcomePort{Data: data}
	svc.config.OwnedExecutionGuard = func(ctx context.Context, owner coordination.ExecutionScope, _ *TaskRun) (context.Context, func(), error) {
		return coordination.WithScope(ctx, owner), func() {}, nil
	}
	store := &pauseStore{run: CloneTaskRun(run), def: def}
	svc.store = store
	svc.queue = NewTaskQueue(store, "round16", time.Minute)
	atomic.StoreInt32(&svc.dispatching, 1)
	result, err := svc.Recover(t.Context(), run.TaskRunID)
	if err != nil {
		t.Fatalf("public Recover rejected committed Owner checkpoint: %v", err)
	}
	if result.Status != RunStatusQueued || result.Generation != run.Generation+1 ||
		result.CheckpointID == nil || *result.CheckpointID != original.CheckpointID ||
		store.cp == nil || store.cp.Version != original.Version || len(store.cp.Payload) != 0 || store.queued != 1 {
		t.Fatalf("public recovery failed to project private Owner checkpoint: run=%+v cp=%+v queued=%d", result, store.cp, store.queued)
	}
	if _, err := svc.Recover(t.Context(), run.TaskRunID); err == nil || store.queued != 1 {
		t.Fatal("repeated recovery dispatched the same task twice")
	}
}
