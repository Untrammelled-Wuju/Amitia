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

func TestRecoveryCheckpointRequiresExactRunAndOwnerEvidence(t *testing.T) {
	run := &TaskRun{TaskRunID: "run-one", InputHash: hashBytes([]byte(`{"input":true}`))}
	def := &TaskDefinition{DefinitionHash: "def-pinned"}
	payload := json.RawMessage(`{"cursor":3,"phase":"validated"}`)
	cp := &TaskCheckpoint{CheckpointID: "checkpoint-three", TaskRunID: run.TaskRunID, Version: 3, InputHash: run.InputHash, DefinitionHash: def.DefinitionHash, Payload: payload, PayloadHash: hashBytes(payload)}
	if err := validateRecoveryCheckpoint(run, def, nil, false); err != nil {
		t.Fatalf("uncheckpointed local task: %v", err)
	}
	if err := validateRecoveryCheckpoint(run, def, nil, true); !IsTaskErrorCode(err, ErrTaskCheckpointIncompatible) {
		t.Fatalf("owner task resumed without checkpoint: %v", err)
	}
	if err := validateRecoveryCheckpoint(run, def, cp, false); err != nil {
		t.Fatalf("local checkpoint: %v", err)
	}
	if err := validateRecoveryCheckpoint(run, def, cp, true); !IsTaskErrorCode(err, ErrTaskCheckpointIncompatible) {
		t.Fatalf("owner task accepted unpinned checkpoint: %v", err)
	}
	current := cp.CheckpointID
	run.CheckpointID = &current
	if err := validateRecoveryCheckpoint(run, def, cp, true); err != nil {
		t.Fatalf("owner checkpoint pin rejected: %v", err)
	}
	for name, mutate := range map[string]func(*TaskCheckpoint){
		"different-run": func(p *TaskCheckpoint) { p.TaskRunID = "foreign" },
		"different-id":  func(p *TaskCheckpoint) { p.CheckpointID = "older" },
		"input":         func(p *TaskCheckpoint) { p.InputHash = "other" },
		"definition":    func(p *TaskCheckpoint) { p.DefinitionHash = "changed" },
		"version":       func(p *TaskCheckpoint) { p.Version = 0 },
		"payload":       func(p *TaskCheckpoint) { p.Payload = json.RawMessage(`{"cursor":4}`) },
		"invalid-json":  func(p *TaskCheckpoint) { p.Payload = json.RawMessage(`{`) },
	} {
		t.Run(name, func(t *testing.T) {
			changed := *cp
			mutate(&changed)
			if err := validateRecoveryCheckpoint(run, def, &changed, true); !IsTaskErrorCode(err, ErrTaskCheckpointIncompatible) {
				t.Fatalf("invalid checkpoint accepted: %v", err)
			}
		})
	}
}

func TestRecoverRefusesActiveWorkerOrExpiredDeadline(t *testing.T) {
	for _, situation := range []string{"active-local", "active-device", "expired", "live-device-lease"} {
		t.Run(situation, func(t *testing.T) {
			run := &TaskRun{TaskRunID: "run", TaskDefinitionID: "task", Status: RunStatusRecoveryRequired, Generation: 3, Revision: 5, InputHash: "input"}
			store := &pauseStore{run: run, def: &TaskDefinition{TaskID: "task"}}
			service := NewTaskRuntimeService(store, DefaultTaskRuntimeConfig())
			switch situation {
			case "active-local":
				service.activeHosts[run.TaskRunID] = &TaskProcessHost{}
			case "active-device":
				service.sourceHosts.Store(run.TaskRunID, &sourceTaskProcess{})
			case "expired":
				old := time.Now().Add(-time.Minute)
				run.DeadlineAt = &old
			case "live-device-lease":
				active := time.Now().Add(time.Minute)
				run.ExecutionPlacement = TaskExecutionPlacementDevice
				run.ExecutionAttemptID = "old-attempt"
				run.LeaseExpiresAt = &active
			}
			if _, err := service.Recover(t.Context(), run.TaskRunID); err == nil {
				t.Fatal("unsafe recovery was accepted")
			}
			if store.run.Generation != 3 || store.run.Status != RunStatusRecoveryRequired || store.queued != 0 {
				t.Fatalf("unsafe recovery mutated task: %+v queued=%d", store.run, store.queued)
			}
		})
	}
}

func TestOwnerRetryCannotLoseStableEffectIdentity(t *testing.T) {
	service, _, _, run, def := taskAuthorityFixture(t)
	def.Idempotent = true
	def.PermissionRequirementStrings = []string{"service.tool.execute"}
	run.TaskDefinitionID = def.TaskID
	run.DefinitionFingerprint, _ = taskDefinitionFingerprint(def)
	run.Status, run.Attempt, run.MaxAttempts = RunStatusFailed, 1, 3
	store := &ownedCallbackStore{ownedOutcomeStore: ownedOutcomeStore{completionFenceStore: completionFenceStore{run: CloneTaskRun(run)}}, definition: def}
	service.store = store
	service.config.OwnedExecutionGuard = func(ctx context.Context, scope coordination.ExecutionScope, _ *TaskRun) (context.Context, func(), error) {
		return coordination.WithScope(ctx, scope), func() {}, nil
	}
	if next, err := service.Retry(t.Context(), run.TaskRunID); next != nil || !IsTaskErrorCode(err, ErrTaskNotRetryable) {
		t.Fatalf("effectful owner task got new TaskRunID: %+v %v", next, err)
	}
	if store.run.TaskRunID != run.TaskRunID {
		t.Fatal("original task was overwritten")
	}
	def.PermissionRequirementStrings = []string{"event.emit"}
	run.DefinitionFingerprint, _ = taskDefinitionFingerprint(def)
	store.definition = def
	store.run.DefinitionFingerprint = run.DefinitionFingerprint
	if _, err := service.Retry(t.Context(), run.TaskRunID); !IsTaskErrorCode(err, ErrTaskNotRetryable) {
		t.Fatalf("event emitting retry was not blocked: %v", err)
	}
}

func TestRecoverStopsOnCheckpointReadFailure(t *testing.T) {
	service, _, _, run, def := taskAuthorityFixture(t)
	run.TaskDefinitionID = def.TaskID
	run.DefinitionFingerprint, _ = taskDefinitionFingerprint(def)
	run.Status = RunStatusRecoveryRequired
	run.Generation = 1
	run.Revision = 1
	store := &checkpointReadFailureStore{pauseStore: pauseStore{run: run, def: def}}
	service.store = store
	service.config.OwnedExecutionGuard = func(ctx context.Context, scope coordination.ExecutionScope, _ *TaskRun) (context.Context, func(), error) {
		return coordination.WithScope(ctx, scope), func() {}, nil
	}
	if _, err := service.Recover(t.Context(), run.TaskRunID); err == nil || store.queued != 0 {
		t.Fatalf("missing owner checkpoint was bypassed: %v", err)
	}
}

type checkpointReadFailureStore struct{ pauseStore }

func (s *checkpointReadFailureStore) GetLatestCheckpoint(context.Context, string) (*TaskCheckpoint, error) {
	return nil, errors.New("checkpoint storage unavailable")
}

func TestRecoverLocalKeepsTaskIdentityAndDropsOldLease(t *testing.T) {
	payload := json.RawMessage(`{"cursor":2}`)
	id := "cp-verified"
	now := time.Now().UTC()
	future := now.Add(time.Hour)
	run := &TaskRun{
		TaskRunID: "recover-in-place", TaskDefinitionID: "task", Status: RunStatusRecoveryRequired,
		Generation: 3, Revision: 6, Attempt: 2, MaxAttempts: 5,
		InputHash: "input", CheckpointID: &id, ExecutionAttemptID: "old-attempt",
		LeaseID: "old-lease", LeaseExpiresAt: &future, LastHeartbeatAt: &now,
	}
	def := &TaskDefinition{TaskID: "task", DefinitionHash: "def-hash"}
	store := &pauseStore{run: run, def: def, cp: &TaskCheckpoint{
		CheckpointID: id, TaskRunID: run.TaskRunID, Version: 2, Payload: payload,
		PayloadHash: hashBytes(payload), DefinitionHash: def.DefinitionHash, InputHash: run.InputHash,
	}}
	service := NewTaskRuntimeService(store, DefaultTaskRuntimeConfig())
	service.queue = NewTaskQueue(store, "test-recovery", time.Minute)
	atomic.StoreInt32(&service.dispatching, 1)
	resumed, err := service.Recover(t.Context(), run.TaskRunID)
	if err != nil {
		t.Fatal(err)
	}
	if resumed.TaskRunID != run.TaskRunID || resumed.Generation != 4 || resumed.Revision != 7 || resumed.Status != RunStatusQueued || resumed.ExecutionAttemptID != "" || resumed.LeaseID != "" || resumed.LeaseExpiresAt != nil || resumed.LastHeartbeatAt != nil || store.queued != 1 {
		t.Fatalf("recovery did not reset only transient state: %+v queued=%d", resumed, store.queued)
	}
}

func TestRecoverWithoutCheckpointRequiresExplicitIdempotentRestart(t *testing.T) {
	for _, condition := range []string{"unrecoverable", "non-idempotent", "allowed"} {
		t.Run(condition, func(t *testing.T) {
			run := &TaskRun{TaskRunID: "run-local", TaskDefinitionID: "task", Status: RunStatusRecoveryRequired, Generation: 1, Revision: 1}
			def := &TaskDefinition{TaskID: "task", Recoverability: RestartableFromBeginning, Idempotency: Idempotent}
			if condition == "unrecoverable" {
				def.Recoverability = NotRecoverable
			}
			if condition == "non-idempotent" {
				def.Idempotency = NonIdempotent
			}
			store := &pauseStore{run: run, def: def}
			service := NewTaskRuntimeService(store, DefaultTaskRuntimeConfig())
			service.queue = NewTaskQueue(store, "recovery", time.Minute)
			atomic.StoreInt32(&service.dispatching, 1)
			got, err := service.Recover(t.Context(), run.TaskRunID)
			if condition == "allowed" {
				if err != nil || got == nil || got.Status != RunStatusQueued || store.queued != 1 {
					t.Fatalf("explicit idempotent restart refused: %v %+v", err, got)
				}
			} else if err == nil || store.run.Status != RunStatusRecoveryRequired || store.queued != 0 {
				t.Fatalf("unsafe restart accepted: %v %+v", err, got)
			}
		})
	}
}
