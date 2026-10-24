package workbenchtasks

import (
	"context"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/interaction"
)

func TestRound52ProjectsDurableGoalAndExecutionLocation(t *testing.T) {
	record := taskRecord("space", "conv", "coord", "running")
	checkpoint := record.RecoveryDescriptor.MultiAgent
	checkpoint.ParentGoalID = "goal-7"
	checkpoint.ParentGoalRevision = 12
	checkpoint.ExecutionCoreID = "core-1"
	checkpoint.WorkspaceDeviceID = "device-2"
	checkpoint.CancelRequested = true
	deadline := time.Date(2026, 10, 23, 12, 0, 0, 0, time.UTC)
	checkpoint.DeadlineAt = &deadline
	result, err := List(context.Background(), taskRecordReader{records: []*interaction.InteractionRecord{record}}, "space", "conv")
	if err != nil || len(result.Tasks) != 1 {
		t.Fatalf("projection failed: %+v %v", result, err)
	}
	got := result.Tasks[0]
	if got.GoalID != "goal-7" || got.GoalRevision != 12 || got.ExecutionCoreID != "core-1" || got.DeviceID != "device-2" || !got.CancelRequested || got.DeadlineAt == nil || !got.DeadlineAt.Equal(deadline) {
		t.Fatalf("durable fields lost: %+v", got)
	}
}

func TestRound52ProjectionRejectsCrossSpaceGoalScope(t *testing.T) {
	record := taskRecord("space", "conv", "coord", "running")
	record.RecoveryDescriptor.MultiAgent.ParentGoalSpaceID = "other-space"
	result, err := List(context.Background(), taskRecordReader{records: []*interaction.InteractionRecord{record}}, "space", "conv")
	if err != nil || len(result.Tasks) != 0 {
		t.Fatalf("cross-space projection leaked: %+v %v", result, err)
	}
}
