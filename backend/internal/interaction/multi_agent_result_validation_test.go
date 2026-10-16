package interaction

import "testing"

func TestTerminalSnapshotRejectsMismatchedChildState(t *testing.T) {
	cases := []struct {
		assignment AgentAssignmentStatus
		child      InteractionStatus
		valid      bool
	}{
		{AssignmentSucceeded, InteractionStatusCompleted, true},
		{AssignmentSucceeded, InteractionStatusFailed, false},
		{AssignmentSucceeded, InteractionStatusProcessing, false},
		{AssignmentFailed, InteractionStatusFailed, true},
		{AssignmentFailed, InteractionStatusInterrupted, true},
		{AssignmentFailed, InteractionStatusArchived, false},
		{AssignmentFailed, InteractionStatusCompleted, false},
		{AssignmentCancelled, InteractionStatusCancelled, true},
		{AssignmentCancelled, InteractionStatusSuperseded, true},
		{AssignmentCancelled, InteractionStatusCompleted, false},
	}
	for _, tc := range cases {
		child := &InteractionRecord{ID: "child-1", Status: tc.child}
		assignment := CoordinationAssignmentSnapshot{AssignmentID: "a", ChildInteractionID: "child-1", Status: tc.assignment}
		err := validateTerminalChildSnapshot(child, assignment)
		if (err == nil) != tc.valid {
			t.Fatalf("incorrect validation for assignment %s with child %s: %v", tc.assignment, tc.child, err)
		}
	}
	if err := validateTerminalChildSnapshot(&InteractionRecord{ID: "other", Status: InteractionStatusCompleted}, CoordinationAssignmentSnapshot{ChildInteractionID: "child-1", Status: AssignmentSucceeded}); err == nil {
		t.Fatal("child identity mismatch was ignored")
	}
}
