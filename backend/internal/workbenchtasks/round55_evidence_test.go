package workbenchtasks

import (
	"context"
	"testing"

	"github.com/u-ai/backend/internal/interaction"
)

func TestRound55LiveSnapshotDoesNotUpgradeUnprovedChild(t *testing.T) {
	reader, parent := round50OwnedTask("running", "completed")
	checkpoint := parent.RecoveryDescriptor.MultiAgent
	checkpoint.AssignmentRefs[0].Status = "running"
	live := interaction.CoordinationSnapshot{CoordinationID: "coord-a", ParentInteractionID: parent.ID, Status: interaction.CoordinationSucceeded, Assignments: []interaction.CoordinationAssignmentSnapshot{{AssignmentID: "assignment-a", WorkerRole: "coding_worker", Status: interaction.AssignmentSucceeded}}}
	got, err := List(context.Background(), reader, "space-a", "conv-a", fakeSnapshot{snapshot: live})
	if err != nil || len(got.Tasks) != 1 || got.Tasks[0].Status != "running" || got.Tasks[0].CompletedCount != 0 || got.Tasks[0].RecoveryState != "needs-reconciliation" || got.Summary.Succeeded != 0 {
		t.Fatalf("live terminal bypassed committed checkpoint: %+v %v", got, err)
	}
}

func TestRound55LiveSnapshotRetainsVerifiedTerminal(t *testing.T) {
	reader, parent := round50OwnedTask("succeeded", "completed")
	live := interaction.CoordinationSnapshot{CoordinationID: "coord-a", ParentInteractionID: parent.ID, Status: interaction.CoordinationSucceeded, Assignments: []interaction.CoordinationAssignmentSnapshot{{AssignmentID: "assignment-a", WorkerRole: "coding_worker", Status: interaction.AssignmentSucceeded}}}
	got, err := List(context.Background(), reader, "space-a", "conv-a", fakeSnapshot{snapshot: live})
	if err != nil || len(got.Tasks) != 1 || got.Tasks[0].Status != "succeeded" || got.Tasks[0].CompletedCount != 1 || got.Tasks[0].RecoveryState != "runtime-observed" {
		t.Fatalf("verified terminal lost: %+v %v", got, err)
	}
}

func TestRound55ScopedChildReaderUsesVerifiedParentScope(t *testing.T) {
	reader, _ := round50OwnedTask("succeeded", "completed")
	scoped := scopedRecordReader{reader: reader, role: "role-a", space: "space-a"}
	if child, ok, err := scoped.Get(context.Background(), "child-a"); err != nil || !ok || child == nil {
		t.Fatalf("authorized child proof unavailable: %v %v", ok, err)
	}
	for _, tc := range []struct{ name, role, space string }{
		{"other_role", "role-b", "space-a"},
		{"other_space", "role-a", "space-b"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			scoped := scopedRecordReader{reader: reader, role: tc.role, space: tc.space}
			if _, ok, err := scoped.Get(context.Background(), "child-a"); err != nil || ok {
				t.Fatalf("foreign proof readable: %v %v", ok, err)
			}
		})
	}
	scoped.canRead = func(context.Context, *interaction.InteractionRecord) (bool, error) { return false, nil }
	got, err := List(context.Background(), scoped, "space-a", "conv-a")
	if err != nil || len(got.Tasks) != 0 {
		t.Fatalf("parent callback bypassed: %+v %v", got, err)
	}
}

func TestRound55AuthorizedParentCanVerifyChildInSeparateConversation(t *testing.T) {
	reader, _ := round50OwnedTask("succeeded", "completed")
	scoped := scopedRecordReader{reader: reader, role: "role-a", space: "space-a", canRead: func(_ context.Context, parent *interaction.InteractionRecord) (bool, error) {
		return parent.Scope.ConversationID == "conv-a" && parent.Scope.Source != "multi_agent", nil
	}}
	result, err := List(context.Background(), scoped, "space-a", "conv-a")
	if err != nil || len(result.Tasks) != 1 || result.Tasks[0].Status != "succeeded" || result.Tasks[0].CompletedCount != 1 || result.Tasks[0].RecoveryState == "needs-reconciliation" {
		t.Fatalf("legitimate owned worker proof lost: %+v %v", result, err)
	}
	resultCheckpoint, err := ListCheckpoints(context.Background(), scoped, "space-a", "conv-a", "coord-a")
	if err != nil || len(resultCheckpoint.Records) != 1 || !resultCheckpoint.Records[0].Consistent || resultCheckpoint.Records[0].CompletedCount != 1 {
		t.Fatalf("legitimate owned checkpoint proof lost: %+v %v", resultCheckpoint, err)
	}
}
