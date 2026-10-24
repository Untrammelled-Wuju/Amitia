package task_runtime

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

type round102ChangingOwnerData struct {
	*taskInputData
	targetID string
	onRead   func()
	fired    bool
}

func (p *round102ChangingOwnerData) Resource(ctx context.Context, scope coordination.ExecutionScope, kind, id string) (*coordination.Resource, error) {
	resource, err := p.taskInputData.Resource(ctx, scope, kind, id)
	if kind == "checkpoint" && id == p.targetID && !p.fired {
		p.fired = true
		p.onRead()
	}
	return resource, err
}

func TestRound102OwnerCheckpointWithoutCommitCursorCannotProject(t *testing.T) {
	svc, ctx, data, store, run, def, checkpoint := round16PendingCheckpointFixture(t)
	delete(data.resources, "checkpoint/"+taskOwnerCommitCursorID(run))
	if _, _, err := svc.reconcilePendingOwnerCheckpoint(ctx, run, def, nil); !IsTaskErrorCode(err, ErrTaskCheckpointIncompatible) {
		t.Fatalf("orphan checkpoint %s was projected without commit cursor: %v", checkpoint.CheckpointID, err)
	}
	if store.casCalls != 0 || store.checkpoint != nil || store.run.CheckpointID != nil {
		t.Fatal("an uncommitted Owner checkpoint changed the local TaskRun")
	}
}

func TestRound102OwnerCursorChangedDuringCheckpointReadCannotProject(t *testing.T) {
	for _, mode := range []string{"terminal", "revision_only", "body_only"} {
		t.Run(mode, func(t *testing.T) {
			svc, ctx, data, store, run, def, checkpoint := round16PendingCheckpointFixture(t)
			key := "checkpoint/" + taskOwnerCommitCursorID(run)
			wrapped := &round102ChangingOwnerData{taskInputData: data, targetID: "task/checkpoint/" + checkpoint.CheckpointID}
			wrapped.onRead = func() {
				stored := data.resources[key]
				var cursor ownedTaskCommitCursor
				if err := json.Unmarshal(stored.Body, &cursor); err != nil {
					t.Fatal(err)
				}
				switch mode {
				case "terminal":
					cursor.Terminal = true
					cursor.OutcomeID = taskOutcomeID(run)
					stored.Revision++
				case "revision_only":
					stored.Revision++
				case "body_only":
					cursor.InFlightEffects = map[string]string{"task/host/other-core": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}
				}
				stored.Body, _ = json.Marshal(cursor)
			}
			svc.config.OwnedCheckpoints = AcknowledgedTaskCheckpointPort{Data: wrapped}
			if _, _, err := svc.reconcilePendingOwnerCheckpoint(ctx, run, def, nil); !IsTaskErrorCode(err, ErrTaskCheckpointIncompatible) {
				t.Fatalf("changed Owner cursor (%s) allowed stale local projection: %v", mode, err)
			}
			if !wrapped.fired || store.casCalls != 0 || store.checkpoint != nil || store.run.CheckpointID != nil {
				t.Fatalf("changed Owner cursor (%s) updated the local run", mode)
			}
		})
	}
}

func TestRound102OwnerCursorAppearsDuringEmptyCheckpointQuery(t *testing.T) {
	svc, ctx, data, store, run, def, checkpoint := round16PendingCheckpointFixture(t)
	cursorKey := "checkpoint/" + taskOwnerCommitCursorID(run)
	cursorRecord := data.resources[cursorKey]
	delete(data.resources, cursorKey)
	delete(data.resources, "checkpoint/task/checkpoint/"+checkpoint.CheckpointID)
	wrapped := &round102ChangingOwnerData{taskInputData: data, targetID: "task/checkpoint/" + checkpoint.CheckpointID}
	wrapped.onRead = func() { data.resources[cursorKey] = cursorRecord }
	svc.config.OwnedCheckpoints = AcknowledgedTaskCheckpointPort{Data: wrapped}
	if _, _, err := svc.reconcilePendingOwnerCheckpoint(ctx, run, def, nil); !IsTaskErrorCode(err, ErrTaskCheckpointIncompatible) {
		t.Fatalf("new Owner commit cursor was ignored during empty candidate lookup: %v", err)
	}
	if !wrapped.fired || store.casCalls != 0 || store.run.CheckpointID != nil {
		t.Fatal("Owner changed during inspection but Core accepted the old empty state")
	}
}

func TestRound102OwnerCursorUnchangedStillProjectsCheckpoint(t *testing.T) {
	svc, ctx, _, store, run, def, checkpoint := round16PendingCheckpointFixture(t)
	updated, recovered, err := svc.reconcilePendingOwnerCheckpoint(ctx, run, def, nil)
	if err != nil || recovered == nil || recovered.CheckpointID != checkpoint.CheckpointID ||
		updated.CheckpointID == nil || *updated.CheckpointID != checkpoint.CheckpointID || store.casCalls != 1 {
		t.Fatalf("stable Owner checkpoint rejected: %v", err)
	}
}
