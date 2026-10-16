package workbenchtasks

import (
	"context"
	"testing"

	"github.com/u-ai/backend/internal/interaction"
)

func TestRejectsRepeatedAssignmentInCheckpoint(t *testing.T) {
	record := taskRecord("space", "conv", "coord", "running")
	record.RecoveryDescriptor.MultiAgent.AssignmentRefs[1].AssignmentID = "worker-1"
	result, err := List(context.Background(), taskRecordReader{records: []*interaction.InteractionRecord{record}}, "space", "conv")
	if err != nil || len(result.Tasks) != 0 {
		t.Fatalf("repeated worker accepted: %+v %v", result, err)
	}
}

func TestRejectsInvalidAssignmentStatus(t *testing.T) {
	record := taskRecord("space", "conv", "coord", "running")
	record.RecoveryDescriptor.MultiAgent.AssignmentRefs[1].Status = "unknown"
	result, err := List(context.Background(), taskRecordReader{records: []*interaction.InteractionRecord{record}}, "space", "conv")
	if err != nil || len(result.Tasks) != 0 {
		t.Fatalf("invalid worker accepted: %+v %v", result, err)
	}
}

func TestSummarizeProjectedTasks(t *testing.T) {
	a := taskRecord("space", "conv", "coord-a", "running")
	b := taskRecord("space", "conv", "coord-b", "succeeded")
	result, err := List(context.Background(), taskRecordReader{records: []*interaction.InteractionRecord{a, b}}, "space", "conv")
	if err != nil || result.Summary.Total != 2 || result.Summary.Active != 1 || result.Summary.Succeeded != 1 || result.Summary.CheckpointOnly != 2 || result.Summary.CodingWorkersRequiringReview != 4 {
		t.Fatalf("summary error: %+v %v", result, err)
	}
}

func TestRuntimeCannotDowngradeTerminalCheckpoint(t *testing.T) {
	record := taskRecord("space", "conv", "coord", "succeeded")
	snapshot := interaction.CoordinationSnapshot{CoordinationID: "coord", ParentInteractionID: record.ID, Status: interaction.CoordinationRunning, Assignments: []interaction.CoordinationAssignmentSnapshot{{AssignmentID: "worker-1", Status: interaction.AssignmentRunning}, {AssignmentID: "worker-2", Status: interaction.AssignmentRunning}}}
	result, err := List(context.Background(), taskRecordReader{records: []*interaction.InteractionRecord{record}}, "space", "conv", fakeSnapshot{snapshot: snapshot})
	if err != nil || len(result.Tasks) != 1 || result.Tasks[0].Status != "succeeded" || result.Tasks[0].RecoveryState != "needs-reconciliation" {
		t.Fatalf("terminal downgraded: %+v %v", result, err)
	}
}

func TestSnapshotCannotOverrideCheckpointReconciliation(t *testing.T) {
	record := taskRecord("space", "conv", "coord", "running")
	if err := record.Transition(interaction.InteractionStatusProcessing); err != nil {
		t.Fatal(err)
	}
	if err := record.Transition(interaction.InteractionStatusCompleted); err != nil {
		t.Fatal(err)
	}
	snap := interaction.CoordinationSnapshot{CoordinationID: "coord", ParentInteractionID: record.ID, Status: interaction.CoordinationSucceeded, Assignments: []interaction.CoordinationAssignmentSnapshot{{AssignmentID: "worker-1", Status: interaction.AssignmentSucceeded}, {AssignmentID: "worker-2", Status: interaction.AssignmentSucceeded}}}
	result, err := List(context.Background(), taskRecordReader{records: []*interaction.InteractionRecord{record}}, "space", "conv", fakeSnapshot{snapshot: snap})
	if err != nil || result.Tasks[0].RecoveryState != "needs-reconciliation" {
		t.Fatalf("reconciliation overwritten: %+v %v", result, err)
	}
}

func TestRejectsCrossScopeGoalCheckpoint(t *testing.T) {
	record := taskRecord("space-a", "conv", "coord", "running")
	record.RecoveryDescriptor.MultiAgent.ParentGoalSpaceID = "space-b"
	got, err := List(context.Background(), taskRecordReader{records: []*interaction.InteractionRecord{record}}, "space-a", "conv")
	if err != nil || len(got.Tasks) != 0 {
		t.Fatalf("cross-scope goal accepted: %+v %v", got, err)
	}
}

func TestRejectsRuntimeRoleMismatch(t *testing.T) {
	record := taskRecord("space", "conv", "coord", "running")
	snap := interaction.CoordinationSnapshot{CoordinationID: "coord", ParentInteractionID: record.ID, Status: interaction.CoordinationSucceeded,
		Assignments: []interaction.CoordinationAssignmentSnapshot{
			{AssignmentID: "worker-1", WorkerRole: "unrelated_role", Status: interaction.AssignmentSucceeded},
			{AssignmentID: "worker-2", WorkerRole: "coding_worker", Status: interaction.AssignmentSucceeded},
		}}
	got, err := List(context.Background(), taskRecordReader{records: []*interaction.InteractionRecord{record}}, "space", "conv", fakeSnapshot{snapshot: snap})
	if err != nil || got.Tasks[0].RecoveryState != "needs-reconciliation" || got.Tasks[0].RecoveryReason != "runtime-checkpoint-conflict" {
		t.Fatalf("unexpected runtime role substitution: %+v %v", got, err)
	}
}

func TestRuntimeStateConflictRequiresReconciliation(t *testing.T) {
	record := taskRecord("space", "conv", "coord", "succeeded")
	snapshot := interaction.CoordinationSnapshot{CoordinationID: "coord", ParentInteractionID: record.ID, Status: interaction.CoordinationRunning, Assignments: []interaction.CoordinationAssignmentSnapshot{
		{AssignmentID: "worker-1", Status: interaction.AssignmentRunning},
		{AssignmentID: "worker-2", Status: interaction.AssignmentRunning},
	}}
	result, err := List(context.Background(), taskRecordReader{records: []*interaction.InteractionRecord{record}}, "space", "conv", fakeSnapshot{snapshot: snapshot})
	if err != nil || result.Tasks[0].RecoveryState != "needs-reconciliation" || result.Tasks[0].RecoveryReason != "runtime-checkpoint-conflict" {
		t.Fatalf("runtime mismatch ignored: %+v %v", result, err)
	}
}

func TestTerminalParentPausedCheckpointRequiresReconciliation(t *testing.T) {
	record := taskRecord("space", "conv", "coord", "paused")
	record.Status = "completed"
	result, err := List(context.Background(), taskRecordReader{records: []*interaction.InteractionRecord{record}}, "space", "conv")
	if err != nil || result.Tasks[0].RecoveryReason != "parent-state-conflict" {
		t.Fatalf("terminal mismatch ignored: %+v %v", result, err)
	}
}
