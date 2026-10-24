package workbenchtasks

import (
	"context"
	"testing"
	"time"
)

func TestRound72WorkbenchDeadlineRequiresTimedChildSuccess(t *testing.T) {
	deadline := time.Now().UTC().Add(-time.Minute)
	for _, tc := range []struct {
		name        string
		completedAt time.Time
		confirmed   bool
	}{
		{"before_deadline", deadline.Add(-time.Minute), true},
		{"at_deadline", deadline, true},
		{"after_deadline", deadline.Add(time.Second), false},
		{"missing_completion_timestamp", time.Time{}, false},
		{"completed_before_creation", deadline.Add(-2 * time.Hour), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			reader, parent := round50OwnedTask("succeeded", "completed")
			parent.RecoveryDescriptor.MultiAgent.DeadlineAt = &deadline
			child := reader.children["child-a"]
			child.CreatedAt = deadline.Add(-time.Hour)
			child.CompletedAt = tc.completedAt
			got, err := List(context.Background(), reader, "space-a", "conv-a")
			if err != nil || len(got.Tasks) != 1 {
				t.Fatalf("list: %+v %v", got, err)
			}
			if tc.confirmed {
				if got.Tasks[0].RecoveryState == "needs-reconciliation" || got.Summary.Succeeded != 1 {
					t.Fatalf("proved on-time completion should remain visible: %+v", got)
				}
			} else {
				if got.Tasks[0].RecoveryState != "needs-reconciliation" || got.Tasks[0].RecoveryReason != "terminal-child-deadline-unverified" || got.Summary.Succeeded != 0 || got.Tasks[0].CompletedCount != 0 {
					t.Fatalf("late or unproved success shown as complete: %+v", got)
				}
			}
			history, err := ListCheckpoints(context.Background(), reader, "space-a", "conv-a", "coord-a")
			if err != nil || len(history.Records) != 1 || history.Records[0].Consistent != tc.confirmed {
				t.Fatalf("history missed deadline evidence: %+v %v", history, err)
			}
		})
	}
}

func TestRound72WorkbenchNoDeadlineRetainsHistoricalProof(t *testing.T) {
	reader, _ := round50OwnedTask("succeeded", "completed")
	got, err := List(context.Background(), reader, "space-a", "conv-a")
	if err != nil || len(got.Tasks) != 1 || got.Summary.Succeeded != 1 {
		t.Fatalf("older checkpoints must remain readable: %+v %v", got, err)
	}
}

func TestRound72WorkbenchDeadlineDoesNotHideConfirmedFailure(t *testing.T) {
	reader, parent := round50OwnedTask("failed", "failed")
	deadline := time.Now().UTC().Add(-time.Minute)
	parent.RecoveryDescriptor.MultiAgent.DeadlineAt = &deadline
	parent.RecoveryDescriptor.MultiAgent.AssignmentRefs[0].Status = "failed"
	got, err := List(context.Background(), reader, "space-a", "conv-a")
	if err != nil || len(got.Tasks) != 1 || got.Summary.Failed != 1 || got.Tasks[0].RecoveryState == "needs-reconciliation" {
		t.Fatalf("confirmed failure hidden by success deadline rule: %+v %v", got, err)
	}
}
