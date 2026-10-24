package task_runtime

import "testing"

func TestRound101OwnerCannotStartCursorAtSkippedVersion(t *testing.T) {
	ctx, data, run, _ := round17OwnerFixture(t)
	port := AcknowledgedTaskCheckpointPort{Data: data}
	for _, version := range []int64{2, 7} {
		checkpoint := round18Checkpoint(run, version)
		if err := port.SaveCheckpoint(ctx, run, checkpoint); !IsTaskErrorCode(err, ErrTaskCheckpointIncompatible) {
			t.Fatalf("first Owner checkpoint with skipped version %d was accepted: %v", version, err)
		}
		if data.resources["checkpoint/task/checkpoint/"+checkpoint.CheckpointID] != nil || data.resources["checkpoint/"+taskOwnerCommitCursorID(run)] != nil {
			t.Fatal("invalid initial checkpoint wrote Owner records")
		}
	}
	first := round18Checkpoint(run, 1)
	if err := port.SaveCheckpoint(ctx, run, first); err != nil {
		t.Fatalf("valid initial Owner checkpoint refused: %v", err)
	}
}

func TestRound101LegacyLocalCheckpointCannotCreateUnconfirmedOwnerCursor(t *testing.T) {
	ctx, data, run, _ := round17OwnerFixture(t)
	legacyID := "legacy-checkpoint"
	run.CheckpointID = &legacyID
	cp := round18Checkpoint(run, 2)
	if err := (AcknowledgedTaskCheckpointPort{Data: data}).SaveCheckpoint(ctx, run, cp); !IsTaskErrorCode(err, ErrTaskCheckpointIncompatible) {
		t.Fatalf("checkpoint without prior Owner cursor was accepted: %v", err)
	}
	if data.resources["checkpoint/"+taskOwnerCommitCursorID(run)] != nil {
		t.Fatal("legacy local reference minted an Owner commit cursor")
	}
}
