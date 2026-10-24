package workbenchtasks

import (
	"context"
	"testing"

	"github.com/u-ai/backend/internal/interaction"
)

func TestRound71TerminalParentCannotOutrunNonTerminalChild(t *testing.T) {
	for _, status := range []string{"succeeded", "failed", "cancelled"} {
		t.Run(status, func(t *testing.T) {
			reader, parent := round50OwnedTask(status, "completed")
			parent.RecoveryDescriptor.MultiAgent.AssignmentRefs[0].Status = "waiting"
			got, err := List(context.Background(), reader, "space-a", "conv-a")
			if err != nil || len(got.Tasks) != 1 {
				t.Fatalf("task list: %+v %v", got, err)
			}
			task := got.Tasks[0]
			if task.RecoveryState != "needs-reconciliation" || task.RecoveryReason != "parent-terminal-child-pending" || got.Summary.Succeeded != 0 || got.Summary.Failed != 0 || got.Summary.NeedsReconciliation != 1 {
				t.Fatalf("unconfirmed terminal parent: %+v %+v", task, got.Summary)
			}
			history, err := ListCheckpoints(context.Background(), reader, "space-a", "conv-a", "coord-a")
			if err != nil || len(history.Records) != 1 || history.Records[0].Consistent {
				t.Fatalf("unconfirmed history: %+v %v", history, err)
			}
		})
	}
}

func TestRound71InterruptedChildCannotConfirmFailedAssignment(t *testing.T) {
	reader, parent := round50OwnedTask("failed", string(interaction.InteractionStatusInterrupted))
	parent.RecoveryDescriptor.MultiAgent.AssignmentRefs[0].Status = "failed"
	result, err := List(context.Background(), reader, "space-a", "conv-a")
	if err != nil || len(result.Tasks) != 1 {
		t.Fatalf("task list: %+v %v", result, err)
	}
	task := result.Tasks[0]
	if task.RecoveryState != "needs-reconciliation" || task.Assignments[0].Status != "waiting" || task.CompletedCount != 0 || result.Summary.Failed != 0 {
		t.Fatalf("interrupted worker counted as settled: %+v", task)
	}
	reader.children["child-a"].Status = interaction.InteractionStatusFailed
	result, err = List(context.Background(), reader, "space-a", "conv-a")
	if err != nil || result.Tasks[0].RecoveryState == "needs-reconciliation" || result.Tasks[0].FailedCount != 1 || result.Summary.Failed != 1 {
		t.Fatalf("confirmed failure became unobservable: %+v %v", result, err)
	}
}

func TestRound71OneChildReceiptCannotConfirmTwoAssignments(t *testing.T) {
	reader, parent := round50OwnedTask("succeeded", "completed")
	checkpoint := parent.RecoveryDescriptor.MultiAgent
	other := checkpoint.AssignmentRefs[0]
	other.AssignmentID = "assignment-b"
	checkpoint.AssignmentRefs = append(checkpoint.AssignmentRefs, other)
	if validateCheckpoint(checkpoint) {
		t.Fatal("the same child receipt cannot prove two independent assignments")
	}
	got, err := List(context.Background(), reader, "space-a", "conv-a")
	if err != nil || len(got.Tasks) != 0 {
		t.Fatalf("duplicated receipt became visible: %+v %v", got, err)
	}
}
