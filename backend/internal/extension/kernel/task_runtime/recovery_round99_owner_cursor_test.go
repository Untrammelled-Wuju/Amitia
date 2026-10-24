package task_runtime

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

func TestRound99CommittedOwnerCheckpointMissingCannotFallBackToOldCursor(t *testing.T) {
	svc, ctx, data, store, run, def, checkpoint := round16PendingCheckpointFixture(t)
	delete(data.resources, "checkpoint/task/checkpoint/"+checkpoint.CheckpointID)
	if _, _, err := svc.reconcilePendingOwnerCheckpoint(ctx, run, def, nil); !IsTaskErrorCode(err, ErrTaskCheckpointIncompatible) {
		t.Fatalf("missing committed first checkpoint must not appear absent: %v", err)
	}
	if store.run.CheckpointID != nil || store.casCalls != 0 || store.checkpoint != nil {
		t.Fatal("missing Owner checkpoint changed local task projection")
	}
}

func TestRound99MissingCommittedNextCheckpointBlocksStaleRecovery(t *testing.T) {
	svc, ctx, data, store, run, def, first := round16PendingCheckpointFixture(t)
	firstMetadata := *first
	firstMetadata.Payload = nil
	store.checkpoint = &firstMetadata
	id := first.CheckpointID
	run.CheckpointID = &id
	store.run.CheckpointID = &id
	body := json.RawMessage(`{"cursor":2,"step":"pending"}`)
	second := &TaskCheckpoint{CheckpointID: ownedStableCheckpointID(run, 2), TaskRunID: run.TaskRunID,
		Version: 2, Payload: body, PayloadHash: hashBytes(body), DefinitionHash: def.DefinitionHash,
		InputHash: run.InputHash, CreatedAt: time.Now().UTC()}
	if err := svc.config.OwnedCheckpoints.SaveCheckpoint(ctx, run, second); err != nil {
		t.Fatal(err)
	}
	delete(data.resources, "checkpoint/task/checkpoint/"+second.CheckpointID)
	previous, err := svc.readTaskCheckpoint(ctx, run)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := svc.reconcilePendingOwnerCheckpoint(ctx, run, def, previous); !IsTaskErrorCode(err, ErrTaskCheckpointIncompatible) {
		t.Fatalf("missing committed next checkpoint must not resume old cursor: %v", err)
	}
	if store.casCalls != 0 || store.run.CheckpointID == nil || *store.run.CheckpointID != first.CheckpointID {
		t.Fatal("reconciliation mutated local task despite missing Owner record")
	}
}

func TestRound99UncommittedOrphanOwnerCheckpointCannotBeProjected(t *testing.T) {
	svc, ctx, data, store, run, def, first := round16PendingCheckpointFixture(t)
	id := first.CheckpointID
	run.CheckpointID = &id
	store.run.CheckpointID = &id
	firstMetadata := *first
	firstMetadata.Payload = nil
	store.checkpoint = &firstMetadata
	body := json.RawMessage(`{"cursor":2}`)
	second := &TaskCheckpoint{CheckpointID: ownedStableCheckpointID(run, 2), TaskRunID: run.TaskRunID,
		Version: 2, Payload: body, PayloadHash: hashBytes(body), DefinitionHash: def.DefinitionHash,
		InputHash: run.InputHash, CreatedAt: time.Now().UTC()}
	scope, ok := coordination.FromContext(ctx)
	if !ok {
		t.Fatal("missing scope")
	}
	metadata := *second
	metadata.Payload = nil
	encoded, err := json.Marshal(ownedTaskCheckpoint{Scope: scope, DefinitionFingerprint: run.DefinitionFingerprint,
		ExecutionGeneration: run.Generation, ExecutionAttemptID: run.ExecutionAttemptID.String(), Checkpoint: metadata, Payload: body})
	if err != nil {
		t.Fatal(err)
	}
	path := "task/checkpoint/" + second.CheckpointID
	data.resources["checkpoint/"+path] = &coordination.Resource{ID: path, Kind: "checkpoint", RoleID: scope.RoleID, OwnerID: scope.ResourceOwnerID, Revision: 1, Body: encoded}
	previous, err := svc.readTaskCheckpoint(ctx, run)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := svc.reconcilePendingOwnerCheckpoint(ctx, run, def, previous); !IsTaskErrorCode(err, ErrTaskCheckpointIncompatible) {
		t.Fatalf("uncommitted orphan record incorrectly projected: %v", err)
	}
	if store.casCalls != 0 || store.run.CheckpointID == nil || *store.run.CheckpointID != id {
		t.Fatal("orphan Owner checkpoint changed local state")
	}
}
