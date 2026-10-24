package task_runtime

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

func TestRound15LocalSequentialCheckpointsAndStrictReplay(t *testing.T) {
	svc, _, _, run, def := taskAuthorityFixture(t)
	run.Status, run.Generation, run.ExecutionAttemptID = RunStatusRunning, 2, "local-sequence"
	run.InputHash = hashBytes([]byte(`{}`))
	def.DefinitionHash = "definition-v1"
	store := &checkpointFenceStore{completionFenceStore: completionFenceStore{run: CloneTaskRun(run)}}
	svc.store = store
	ctx := t.Context()

	save := func(version int64, suffix string) error {
		payload := json.RawMessage(fmt.Sprintf(`{"cursor":%d,"data":%q}`, version, suffix))
		return svc.handleCheckpoint(ctx, run, def, payload, hashBytes(payload), version)
	}
	if err := save(4, "first"); err != nil {
		t.Fatal(err)
	}
	oldID := store.checkpoint.CheckpointID
	if err := save(5, "second"); err != nil {
		t.Fatalf("later checkpoint rejected: %v", err)
	}
	if store.checkpoint.Version != 5 || store.checkpoint.CheckpointID == oldID || store.casCalls != 2 {
		t.Fatal("second checkpoint was not committed as a distinct version")
	}
	secondID, revision := store.checkpoint.CheckpointID, store.run.Revision
	if err := save(5, "second"); err != nil {
		t.Fatalf("same bytes were not idempotent: %v", err)
	}
	if store.casCalls != 2 || store.run.Revision != revision || store.checkpoint.CheckpointID != secondID {
		t.Fatal("duplicate callback wrote another checkpoint")
	}
	for _, tc := range []struct {
		version int64
		suffix  string
	}{
		{4, "first"}, {5, "changed"}, {7, "skipped"},
	} {
		if err := save(tc.version, tc.suffix); err == nil {
			t.Fatalf("stale, conflicting or skipped checkpoint accepted: %+v", tc)
		}
	}
	mismatched := json.RawMessage(`{"cursor":999,"data":"invalid"}`)
	if err := svc.handleCheckpoint(ctx, run, def, mismatched, hashBytes(mismatched), 6); err == nil {
		t.Fatal("checkpoint version was not bound to the restored payload cursor")
	}
	if store.casCalls != 2 {
		t.Fatal("invalid checkpoint caused a state write")
	}

	store.checkpoint.PayloadHash = "tampered"
	if err := save(6, "third"); err == nil {
		t.Fatal("new checkpoint accepted on top of corrupted predecessor")
	}
	if store.casCalls != 2 {
		t.Fatal("corrupt predecessor caused a state write")
	}
}

func TestRound15OwnerSequentialCheckpointsAcrossRecoveryGenerations(t *testing.T) {
	svc, _, authority, run, def := taskAuthorityFixture(t)
	run.TaskDefinitionID = def.TaskID
	run.DefinitionFingerprint, _ = taskDefinitionFingerprint(def)
	run.Status, run.Generation, run.ExecutionAttemptID = RunStatusRunning, 1, "owner-first"
	run.Input = json.RawMessage(`{"secret":true}`)
	run.InputHash = hashBytes(run.Input)
	def.DefinitionHash = "definition-v1"
	data := &taskInputData{}
	ctx := coordination.WithScope(t.Context(), authority)
	if err := (AcknowledgedTaskInputPort{Data: data}).SaveInput(ctx, run); err != nil {
		t.Fatal(err)
	}
	run.Input = nil
	svc.config.OwnedCheckpoints = AcknowledgedTaskCheckpointPort{Data: data}
	store := &checkpointFenceStore{completionFenceStore: completionFenceStore{run: CloneTaskRun(run)}}
	svc.store = store

	write := func(version int64, phase string) error {
		payload := json.RawMessage(fmt.Sprintf(`{"cursor":%d,"data":{"phase":%q}}`, version, phase))
		return svc.handleCheckpoint(ctx, run, def, payload, hashBytes(payload), version)
	}
	if err := write(1, "first"); err != nil {
		t.Fatal(err)
	}
	firstID := store.checkpoint.CheckpointID

	run.Generation, run.ExecutionAttemptID = 2, "owner-second"
	store.run.Generation, store.run.ExecutionAttemptID = 2, "owner-second"
	if err := write(2, "second"); err != nil {
		t.Fatalf("checkpoint after recovery was rejected: %v", err)
	}
	if store.checkpoint.Version != 2 || store.checkpoint.CheckpointID == firstID || len(store.checkpoint.Payload) != 0 {
		t.Fatal("Owner checkpoint version did not advance or private content leaked")
	}
	secondID := store.checkpoint.CheckpointID
	run.Generation, run.ExecutionAttemptID = 3, "owner-third"
	store.run.Generation, store.run.ExecutionAttemptID = 3, "owner-third"
	if err := write(3, "third"); err != nil {
		t.Fatalf("third generation checkpoint was rejected: %v", err)
	}
	if store.checkpoint.Version != 3 || store.checkpoint.CheckpointID == secondID || store.casCalls != 3 {
		t.Fatal("third generation did not record a new Owner checkpoint")
	}
	if err := write(3, "third"); err != nil || store.casCalls != 3 {
		t.Fatalf("third generation exact callback was not idempotent: %v", err)
	}
	if err := write(5, "skipped"); err == nil {
		t.Fatal("skipped Owner version accepted")
	}
	if store.casCalls != 3 {
		t.Fatal("invalid callback updated the local task")
	}
}

func TestRound15OwnerRejectsTamperedPredecessorBeforeAdvancing(t *testing.T) {
	svc, _, authority, run, def := taskAuthorityFixture(t)
	run.TaskDefinitionID = def.TaskID
	run.DefinitionFingerprint, _ = taskDefinitionFingerprint(def)
	run.Status, run.Generation, run.ExecutionAttemptID = RunStatusRunning, 1, "original"
	run.Input = json.RawMessage(`{"secret":true}`)
	run.InputHash = hashBytes(run.Input)
	data := &taskInputData{}
	ctx := coordination.WithScope(t.Context(), authority)
	if err := (AcknowledgedTaskInputPort{Data: data}).SaveInput(ctx, run); err != nil {
		t.Fatal(err)
	}
	run.Input = nil
	svc.config.OwnedCheckpoints = AcknowledgedTaskCheckpointPort{Data: data}
	store := &checkpointFenceStore{completionFenceStore: completionFenceStore{run: CloneTaskRun(run)}}
	svc.store = store
	first := json.RawMessage(`{"cursor":1,"phase":"first"}`)
	if err := svc.handleCheckpoint(ctx, run, def, first, hashBytes(first), 1); err != nil {
		t.Fatal(err)
	}
	original := store.checkpoint.CheckpointID
	owner := data.resources["checkpoint/task/checkpoint/"+original]
	if owner == nil {
		t.Fatal("missing original Owner record")
	}
	owner.Body = json.RawMessage(`{"corrupted":true}`)
	next := json.RawMessage(`{"cursor":2,"phase":"second"}`)
	if err := svc.handleCheckpoint(ctx, run, def, next, hashBytes(next), 2); err == nil {
		t.Fatal("tampered original Owner record authorized a successor")
	}
	if store.casCalls != 1 || store.checkpoint.CheckpointID != original {
		t.Fatal("tampered predecessor changed the local task")
	}
}

func TestRound15LegacyOpaqueCheckpointPayloadStillAccepted(t *testing.T) {
	if err := validateCheckpointPayloadVersion(json.RawMessage(`{"data":{"version":5}}`), 2); err != nil {
		t.Fatalf("cursorless checkpoint was rejected: %v", err)
	}
	if err := validateCheckpointPayloadVersion(json.RawMessage(`[1,2,3]`), 2); err != nil {
		t.Fatalf("legacy array checkpoint was rejected: %v", err)
	}
	if err := validateCheckpointPayloadVersion(json.RawMessage(`{"cursor":"2"}`), 2); err == nil {
		t.Fatal("string cursor accepted")
	}
	if err := validateCheckpointPayloadVersion(json.RawMessage(`{"cursor":2.0}`), 2); err == nil {
		t.Fatal("non-integer cursor accepted")
	}
}

func TestRound15OwnerCheckpointProvenanceFencesForeignAttempts(t *testing.T) {
	_, _, authority, run, def := taskAuthorityFixture(t)
	run.TaskDefinitionID = def.TaskID
	run.DefinitionFingerprint, _ = taskDefinitionFingerprint(def)
	run.Generation, run.ExecutionAttemptID = 1, "original"
	run.Input = json.RawMessage(`{"secret":1}`)
	run.InputHash = hashBytes(run.Input)
	data := &taskInputData{}
	ctx := coordination.WithScope(t.Context(), authority)
	if err := (AcknowledgedTaskInputPort{Data: data}).SaveInput(ctx, run); err != nil {
		t.Fatal(err)
	}
	run.Input = nil
	port := AcknowledgedTaskCheckpointPort{Data: data}
	body := json.RawMessage(`{"cursor":1,"phase":"original"}`)
	cp := &TaskCheckpoint{CheckpointID: ownedStableCheckpointID(run, 1), TaskRunID: run.TaskRunID,
		Version: 1, Payload: body, PayloadHash: hashBytes(body), DefinitionHash: def.DefinitionHash,
		InputHash: run.InputHash}
	foreign := *cp
	foreign.CheckpointID = "manually-chosen-checkpoint"
	if err := port.SaveCheckpoint(ctx, run, &foreign); err == nil {
		t.Fatal("new Owner checkpoint accepted an arbitrary identity")
	}
	if err := port.SaveCheckpoint(ctx, run, cp); err != nil {
		t.Fatal(err)
	}
	id := "checkpoint/task/checkpoint/" + cp.CheckpointID
	resource := data.resources[id]
	if resource == nil {
		t.Fatal("missing Owner checkpoint")
	}
	original := append([]byte(nil), resource.Body...)
	var record ownedTaskCheckpoint
	if err := json.Unmarshal(original, &record); err != nil {
		t.Fatal(err)
	}

	mutated := record
	mutated.ExecutionAttemptID = "foreign"
	resource.Body, _ = json.Marshal(mutated)
	if _, err := port.Checkpoint(ctx, run, cp); err == nil {
		t.Fatal("foreign attempt checkpoint accepted")
	}
	mutated = record
	mutated.ExecutionGeneration = 9
	resource.Body, _ = json.Marshal(mutated)
	if _, err := port.Checkpoint(ctx, run, cp); err == nil {
		t.Fatal("future generation checkpoint accepted")
	}
	resource.Body = original
	run.Generation, run.ExecutionAttemptID = 2, "recovered"
	if got, err := port.Checkpoint(ctx, run, cp); err != nil || string(got.Payload) != string(body) {
		t.Fatalf("historical valid checkpoint was not readable after recovery: %v", err)
	}
}
