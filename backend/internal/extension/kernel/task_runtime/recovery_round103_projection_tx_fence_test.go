package task_runtime

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

func TestRound103OwnerCursorChangedAfterInspectionBeforeLocalTransaction(t *testing.T) {
	for _, mode := range []string{"revision", "terminal", "native_effect", "checkpoint_body"} {
		t.Run(mode, func(t *testing.T) {
			svc, ctx, data, store, run, def, checkpoint := round16PendingCheckpointFixture(t)
			store.beforeTx = func() {
				if mode == "checkpoint_body" {
					key := "checkpoint/task/checkpoint/" + checkpoint.CheckpointID
					resource := data.resources[key]
					var record ownedTaskCheckpoint
					if err := json.Unmarshal(resource.Body, &record); err != nil {
						t.Fatal(err)
					}
					record.Checkpoint.CreatedAt = record.Checkpoint.CreatedAt.Add(1)
					resource.Body, _ = json.Marshal(record)
					return
				}
				key := "checkpoint/" + taskOwnerCommitCursorID(run)
				resource := data.resources[key]
				var cursor ownedTaskCommitCursor
				if err := json.Unmarshal(resource.Body, &cursor); err != nil {
					t.Fatal(err)
				}
				switch mode {
				case "revision":
					resource.Revision++
				case "terminal":
					cursor.Terminal, cursor.OutcomeID = true, taskOutcomeID(run)
					resource.Revision++
				case "native_effect":
					cursor.InFlightEffects = map[string]string{"task/host/round103": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}
					resource.Revision++
				}
				resource.Body, _ = json.Marshal(cursor)
			}
			_, _, err := svc.reconcilePendingOwnerCheckpoint(ctx, run, def, nil)
			if !IsTaskErrorCode(err, ErrTaskCheckpointIncompatible) || store.casCalls != 0 || store.checkpoint != nil || store.run.CheckpointID != nil {
				t.Fatalf("mode %s allowed stale projection: error=%v cas=%d", mode, err, store.casCalls)
			}
		})
	}
}

func TestRound103OwnerProjectionStableInsideTransaction(t *testing.T) {
	svc, ctx, _, store, run, def, checkpoint := round16PendingCheckpointFixture(t)
	next, recovered, err := svc.reconcilePendingOwnerCheckpoint(ctx, run, def, nil)
	if err != nil || recovered == nil || recovered.CheckpointID != checkpoint.CheckpointID || next.CheckpointID == nil || *next.CheckpointID != checkpoint.CheckpointID || store.casCalls != 1 {
		t.Fatalf("stable checkpoint cannot be projected: %v", err)
	}
}

func TestRound103ProjectionRequiresTrustedCurrentOwnerContext(t *testing.T) {
	svc, _, _, store, run, def, _ := round16PendingCheckpointFixture(t)
	_, _, err := svc.reconcilePendingOwnerCheckpoint(context.Background(), run, def, nil)
	if err == nil || store.casCalls != 0 {
		t.Fatalf("projection did not enforce current owner authority: %v", err)
	}
}

var _ coordination.DataPort = (*taskInputData)(nil)
