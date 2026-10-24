package task_runtime

import (
	"context"
	"encoding/json"
	"sync/atomic"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/devicemesh/executionjournal"
	"github.com/u-ai/backend/internal/extension/kernel/capability"
	"github.com/u-ai/backend/internal/extension/kernel/scope"
)

func round10CoordinatedSnapshot(t *testing.T, snapshots *scope.MemoryScopeStore, authority *coordination.ExecutionScope, run *TaskRun) {
	t.Helper()
	authority.Coordinated = true
	original, err := snapshots.GetSnapshot(t.Context(), run.ScopeSnapshotID)
	if err != nil {
		t.Fatal(err)
	}
	original.SnapshotID = "round10-coordinated"
	original.OwnedExecutionScope, err = json.Marshal(authority)
	if err != nil {
		t.Fatal(err)
	}
	if err := snapshots.SaveSnapshot(t.Context(), original); err != nil {
		t.Fatal(err)
	}
	run.ScopeSnapshotID = original.SnapshotID
}

type round10AbsentOutcomePort struct{ OwnedTaskOutcomePort }

func (round10AbsentOutcomePort) InspectOutcome(context.Context, *TaskRun) (*CommittedTaskOutcome, error) {
	return nil, nil
}

type round10CheckpointPort struct {
	cp    *TaskCheckpoint
	reads int
}

func (p *round10CheckpointPort) SaveCheckpoint(context.Context, *TaskRun, *TaskCheckpoint) error {
	return nil
}
func (p *round10CheckpointPort) Checkpoint(_ context.Context, _ *TaskRun, _ *TaskCheckpoint) (*TaskCheckpoint, error) {
	p.reads++
	return p.cp, nil
}

func (p *round10CheckpointPort) InspectPendingCheckpoint(context.Context, *TaskRun, *TaskDefinition, *TaskCheckpoint) (*TaskCheckpoint, error) {
	return nil, nil
}

func TestDurableRecoveryStopRequiresOwnerVerifiedIdempotentCheckpoint(t *testing.T) {
	run := &TaskRun{TaskRunID: "run", TaskDefinitionID: "task.action", Generation: 3, Status: RunStatusRecoveryRequired,
		ScopeSnapshotID: "snapshot", ExecutionAttemptID: "attempt", LeaseID: "lease", ExecutionPlacement: TaskExecutionPlacementDevice,
		ExecutionTarget: TaskExecutionTarget{SpaceID: "core", DeviceID: "target", RuntimeID: "runtime", RuntimeSessionID: "old-session", ConnectionGeneration: 7}}
	pin := "cp-id"
	run.CheckpointID = &pin
	cp := &TaskCheckpoint{CheckpointID: pin}
	def := &TaskDefinition{Checkpoint: true, Recoverability: CheckpointRecoverable, Idempotency: Idempotent}
	owner := coordination.ExecutionScope{SpaceID: "core", TargetDeviceID: "target", Coordinated: true}
	ctx := coordination.WithScope(t.Context(), owner)
	port := &testJournalInvocationPort{reply: capability.UnifiedToolResult{
		Status: capability.ToolResultStatusSuccess, DeviceID: "target", RuntimeID: "runtime", RuntimeSessionID: "new-session", Generation: 8,
		Structured: json.RawMessage(`{"state":"stopped_performed"}`),
	}}
	executor := NewMeshRemoteTaskExecutor(nil, nil)
	executor.SetInspectionPort(port)
	svc := NewTaskRuntimeService(nil, DefaultTaskRuntimeConfig())
	svc.SetRemoteExecutor(executor)
	if err := svc.confirmRecoveryStop(ctx, run, def, cp, true); err != nil {
		t.Fatalf("durable owner stop refused: %v", err)
	}
	for _, tc := range []struct {
		name   string
		change func(*TaskDefinition, *TaskCheckpoint, *TaskRun, *bool)
	}{
		{"not-owned", func(_ *TaskDefinition, _ *TaskCheckpoint, _ *TaskRun, owned *bool) { *owned = false }},
		{"non-idempotent", func(d *TaskDefinition, _ *TaskCheckpoint, _ *TaskRun, _ *bool) { d.Idempotency = NonIdempotent }},
		{"not-checkpointable", func(d *TaskDefinition, _ *TaskCheckpoint, _ *TaskRun, _ *bool) { d.Checkpoint = false }},
		{"wrong-recovery-mode", func(d *TaskDefinition, _ *TaskCheckpoint, _ *TaskRun, _ *bool) {
			d.Recoverability = RestartableFromBeginning
		}},
		{"missing-checkpoint", func(_ *TaskDefinition, c *TaskCheckpoint, _ *TaskRun, _ *bool) { *c = TaskCheckpoint{} }},
		{"checkpoint-not-pinned", func(_ *TaskDefinition, _ *TaskCheckpoint, r *TaskRun, _ *bool) { r.CheckpointID = nil }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			changedDef, changedCP, changedRun, owned := *def, *cp, *run, true
			tc.change(&changedDef, &changedCP, &changedRun, &owned)
			if err := svc.confirmRecoveryStop(ctx, &changedRun, &changedDef, &changedCP, owned); err == nil {
				t.Fatal("untrusted replay authorized")
			}
		})
	}
	for _, state := range []string{"started", "performed", "completed", "stopped_unknown", "absent", "invented"} {
		t.Run(state, func(t *testing.T) {
			port.reply.Structured = json.RawMessage(`{"state":"` + state + `"}`)
			if err := svc.confirmRecoveryStop(ctx, run, def, cp, true); err == nil {
				t.Fatal("unknown effect upgraded to recovery permission")
			}
		})
	}
	port.reply.Structured = json.RawMessage(`{"state":"stopped_performed"}`)
	if err := svc.confirmRecoveryStop(t.Context(), run, def, cp, true); err == nil {
		t.Fatal("unowned journal inspection accepted")
	}
	if port.calls < 1 {
		t.Fatal("journal was not consulted")
	}
}

func TestRecoverDeviceWithDurableStopAfterOwnerCheckpointValidation(t *testing.T) {
	service, snapshots, authority, run, def := taskAuthorityFixture(t)
	round10CoordinatedSnapshot(t, snapshots, &authority, run)
	run.TaskDefinitionID = def.TaskID
	run.InputHash = hashBytes([]byte(`{"input":1}`))
	run.Generation, run.Revision = 3, 7
	run.Status = RunStatusRecoveryRequired
	run.ExecutionPlacement = TaskExecutionPlacementDevice
	run.ExecutionAttemptID, run.LeaseID = "attempt-3", "lease-3"
	run.ExecutionTarget = TaskExecutionTarget{SpaceID: "core", DeviceID: "target", RuntimeID: "runtime", RuntimeSessionID: "old-session", ConnectionGeneration: 5}
	def.Checkpoint, def.Recoverability, def.Idempotency = true, CheckpointRecoverable, Idempotent
	def.DefinitionHash = "definition-hash"
	run.DefinitionFingerprint, _ = taskDefinitionFingerprint(def)
	pin, payload := "checkpoint-1", json.RawMessage(`{"cursor":2}`)
	run.CheckpointID = &pin
	cp := &TaskCheckpoint{TaskRunID: run.TaskRunID, CheckpointID: pin, Version: 2, Payload: payload, PayloadHash: hashBytes(payload), InputHash: run.InputHash, DefinitionHash: def.DefinitionHash}
	store := &pauseStore{run: CloneTaskRun(run), def: def, cp: cp}
	service.store = store
	service.queue = NewTaskQueue(store, "round10-recovery", time.Minute)
	atomic.StoreInt32(&service.dispatching, 1)
	checked := &round10CheckpointPort{cp: cp}
	service.config.OwnedCheckpoints = checked
	service.config.OwnedOutcomes = round10AbsentOutcomePort{}
	service.config.OwnedExecutionGuard = func(ctx context.Context, owner coordination.ExecutionScope, _ *TaskRun) (context.Context, func(), error) {
		return coordination.WithScope(ctx, owner), func() {}, nil
	}
	port := &testJournalInvocationPort{reply: capability.UnifiedToolResult{Status: capability.ToolResultStatusSuccess, DeviceID: "target", RuntimeID: "runtime", RuntimeSessionID: "new-session", Generation: 9, Structured: json.RawMessage(`{"state":"stopped_performed"}`)}}
	executor := NewMeshRemoteTaskExecutor(nil, nil)
	executor.SetInspectionPort(port)
	service.SetRemoteExecutor(executor)
	got, err := service.Recover(t.Context(), run.TaskRunID)
	if err != nil {
		t.Fatalf("verified recovery failed: %v", err)
	}
	if got.Status != RunStatusQueued || got.TaskRunID != run.TaskRunID || got.Generation != 4 || got.Revision != 8 || got.ExecutionAttemptID != "" || got.LeaseID != "" || got.ExecutionTarget.RuntimeSessionID != "" || store.queued != 1 || checked.reads != 1 || port.calls != 1 {
		t.Fatalf("recovery did not preserve identity and clear stale bindings: run=%+v queue=%d cp=%d inspect=%d", got, store.queued, checked.reads, port.calls)
	}

	if _, err := service.Recover(t.Context(), run.TaskRunID); err == nil || store.queued != 1 {
		t.Fatalf("duplicate recovery accepted: %v queued=%d", err, store.queued)
	}
}

func TestRecoverDeviceRejectsUncertainStopWithoutTaskMutation(t *testing.T) {
	for _, state := range []string{"started", "performed", "stopped_unknown", "absent"} {
		t.Run(state, func(t *testing.T) {
			service, snapshots, authority, run, def := taskAuthorityFixture(t)
			round10CoordinatedSnapshot(t, snapshots, &authority, run)
			def.Checkpoint = true
			def.Recoverability = CheckpointRecoverable
			def.Idempotency = Idempotent
			def.DefinitionHash = "def"
			run.TaskDefinitionID = def.TaskID
			run.DefinitionFingerprint, _ = taskDefinitionFingerprint(def)
			run.InputHash = "input"
			run.ExecutionPlacement = TaskExecutionPlacementDevice
			run.ExecutionTarget = TaskExecutionTarget{SpaceID: "core", DeviceID: "target", RuntimeID: "runtime", RuntimeSessionID: "old", ConnectionGeneration: 4}
			run.Generation = 2
			run.Revision = 4
			run.ExecutionAttemptID = "attempt"
			run.LeaseID = "lease"
			run.Status = RunStatusRecoveryRequired
			payload := json.RawMessage(`{"cursor":1}`)
			pin := "cp"
			run.CheckpointID = &pin
			cp := &TaskCheckpoint{TaskRunID: run.TaskRunID, CheckpointID: pin, Payload: payload, PayloadHash: hashBytes(payload), Version: 1, DefinitionHash: def.DefinitionHash, InputHash: run.InputHash}
			store := &pauseStore{run: run, def: def, cp: cp}
			service.store = store
			service.queue = NewTaskQueue(store, "round10", time.Minute)
			service.config.OwnedCheckpoints = &round10CheckpointPort{cp: cp}
			service.config.OwnedExecutionGuard = func(ctx context.Context, a coordination.ExecutionScope, _ *TaskRun) (context.Context, func(), error) {
				return coordination.WithScope(ctx, a), func() {}, nil
			}
			port := &testJournalInvocationPort{reply: capability.UnifiedToolResult{Status: capability.ToolResultStatusSuccess, DeviceID: "target", RuntimeID: "runtime", RuntimeSessionID: "new", Generation: 5, Structured: json.RawMessage(`{"state":"` + state + `"}`)}}
			executor := NewMeshRemoteTaskExecutor(nil, nil)
			executor.SetInspectionPort(port)
			service.SetRemoteExecutor(executor)
			if _, err := service.Recover(t.Context(), run.TaskRunID); err == nil || store.queued != 0 || store.run.Generation != 2 || store.run.Status != RunStatusRecoveryRequired {
				t.Fatalf("unsafe %s replayed: %v run=%+v queued=%d", state, err, store.run, store.queued)
			}
		})
	}
}

var _ executionjournal.Inspection

type round10MemoryStopExecutor struct {
	UnavailableRemoteTaskExecutor
	accepted bool
}

func (e round10MemoryStopExecutor) ConfirmStoppedAttempt(context.Context, *TaskRun) bool {
	return e.accepted
}

func TestRecoveryMemoryStopCannotBypassOwnerCheckpointOrIdempotency(t *testing.T) {
	pin := "checkpoint"
	run := &TaskRun{TaskRunID: "run", ExecutionPlacement: TaskExecutionPlacementDevice, CheckpointID: &pin,
		ExecutionTarget: TaskExecutionTarget{SpaceID: "core", DeviceID: "target"}}
	ctx := coordination.WithScope(t.Context(), coordination.ExecutionScope{SpaceID: "core", TargetDeviceID: "target", Coordinated: true})
	cp := &TaskCheckpoint{CheckpointID: pin}
	def := &TaskDefinition{Checkpoint: true, Recoverability: CheckpointRecoverable, Idempotency: Idempotent}
	svc := NewTaskRuntimeService(nil, DefaultTaskRuntimeConfig())
	svc.SetRemoteExecutor(round10MemoryStopExecutor{accepted: true})
	if err := svc.confirmRecoveryStop(ctx, run, def, cp, true); err != nil {
		t.Fatalf("valid in-memory stop rejected: %v", err)
	}
	def.Idempotency = NonIdempotent
	if err := svc.confirmRecoveryStop(ctx, run, def, cp, true); err == nil {
		t.Fatal("non-idempotent device step restarted using a live receipt")
	}
	def.Idempotency = Idempotent
	if err := svc.confirmRecoveryStop(ctx, run, def, cp, false); err == nil {
		t.Fatal("non-owned remote step restarted")
	}
	if err := svc.confirmRecoveryStop(ctx, run, def, nil, true); err == nil {
		t.Fatal("uncheckpointed remote step restarted")
	}
}

func TestResumeCheckpointValidationRejectsForgedMetadata(t *testing.T) {
	for _, kind := range []string{"wrong-run", "bad-json", "bad-version", "wrong-pin", "bad-hash"} {
		t.Run(kind, func(t *testing.T) {
			pin := "saved"
			payload := json.RawMessage(`{"cursor":2}`)
			run := &TaskRun{TaskRunID: "run", TaskDefinitionID: "task", Status: RunStatusPaused, Generation: 2, Revision: 3, InputHash: "input", CheckpointID: &pin, ExecutionPlacement: TaskExecutionPlacementLocal}
			def := &TaskDefinition{TaskID: "task", Checkpoint: true, DefinitionHash: "definition"}
			cp := &TaskCheckpoint{TaskRunID: run.TaskRunID, CheckpointID: pin, Version: 2, InputHash: run.InputHash, DefinitionHash: def.DefinitionHash, Payload: payload, PayloadHash: hashBytes(payload)}
			switch kind {
			case "wrong-run":
				cp.TaskRunID = "foreign"
			case "bad-json":
				cp.Payload = json.RawMessage(`{`)
				cp.PayloadHash = hashBytes(cp.Payload)
			case "bad-version":
				cp.Version = 0
			case "wrong-pin":
				cp.CheckpointID = "foreign"
			case "bad-hash":
				cp.PayloadHash = "invalid"
			}
			store := &pauseStore{run: run, def: def, cp: cp}
			service := NewTaskRuntimeService(store, DefaultTaskRuntimeConfig())
			service.queue = NewTaskQueue(store, "round10-invalid-checkpoint", time.Minute)
			if err := service.ResumeTask(t.Context(), ResumeTaskRequest{TaskRunID: run.TaskRunID, Generation: run.Generation}); err == nil || store.queued != 0 || store.run.Status != RunStatusPaused {
				t.Fatalf("invalid checkpoint %s bypassed single validator: err=%v run=%+v queued=%d", kind, err, store.run, store.queued)
			}
		})
	}
}

func TestRecoverCannotRequeueUnboundDeviceOrUnsupportedCloud(t *testing.T) {
	for _, place := range []TaskExecutionPlacement{TaskExecutionPlacementDevice, TaskExecutionPlacementCloud} {
		t.Run(string(place), func(t *testing.T) {
			run := &TaskRun{TaskRunID: "remote", TaskDefinitionID: "task", Status: RunStatusRecoveryRequired, Generation: 2, Revision: 2, ExecutionPlacement: place}
			def := &TaskDefinition{TaskID: "task", Recoverability: RestartableFromBeginning, Idempotency: Idempotent}
			store := &pauseStore{run: run, def: def}
			svc := NewTaskRuntimeService(store, DefaultTaskRuntimeConfig())
			svc.queue = NewTaskQueue(store, "round10-unbound", time.Minute)
			if _, err := svc.Recover(t.Context(), run.TaskRunID); err == nil || store.queued != 0 || store.run.Status != RunStatusRecoveryRequired {
				t.Fatalf("remote run without stop binding restarted: %s err=%v queued=%d", place, err, store.queued)
			}
		})
	}
}
