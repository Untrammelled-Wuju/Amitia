package workbenchtasks

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/interaction"
)

type round50TaskReader struct {
	records  []*interaction.InteractionRecord
	children map[string]*interaction.InteractionRecord
	reads    int
	err      error
}

func (r *round50TaskReader) ListByScope(context.Context, interaction.InteractionScope) ([]*interaction.InteractionRecord, error) {
	return r.records, nil
}

func (r *round50TaskReader) Get(_ context.Context, id string) (*interaction.InteractionRecord, bool, error) {
	r.reads++
	if r.err != nil {
		return nil, false, r.err
	}
	child, ok := r.children[id]
	return child, ok, nil
}

func round50OwnedTask(status, childStatus string) (*round50TaskReader, *interaction.InteractionRecord) {
	parent := interaction.NewInteractionRecord(interaction.InteractionScope{SpaceID: "space-a", CharacterID: "role-a", ConversationID: "conv-a", RequestID: "parent-a", Source: "web"})
	parent.Status = interaction.InteractionStatusProcessing
	parent.UpdatedAt = time.Now().UTC()
	checkpoint := &interaction.MultiAgentRecoveryRef{
		CoordinationID: "coord-a", Status: status,
		OwnedScope:        &coordination.ExecutionScope{SpaceID: "space-a", RoleID: "role-a"},
		ParentGoalSpaceID: "space-a", ParentGoalCharacterID: "role-a", ParentGoalConversationID: "conv-a",
		AssignmentRefs: []interaction.AssignmentRecoveryRef{{AssignmentID: "assignment-a", WorkerID: "worker-a", CharacterID: "role-a", Role: "coding_worker", ChildInteractionID: "child-a", Status: "succeeded", Objective: "verify worktree"}},
	}
	parent.RecoveryDescriptor = &interaction.RecoveryDescriptor{MultiAgent: checkpoint}
	reader := &round50TaskReader{records: []*interaction.InteractionRecord{parent}, children: map[string]*interaction.InteractionRecord{}}
	if childStatus != "" {
		child := interaction.NewInteractionRecord(interaction.InteractionScope{SpaceID: "space-a", CharacterID: "role-a", ConversationID: "multi-agent-worker-assignment-a", RequestID: "ma-assignment-a", Source: "multi_agent"})
		child.ID = "child-a"
		child.Status = interaction.InteractionStatus(childStatus)
		reader.children["child-a"] = child
	}
	return reader, parent
}

func TestRound50WorkbenchRejectsMissingOrForgedChildTerminalProof(t *testing.T) {
	for _, tc := range []struct {
		name, childStatus string
		corrupt           func(*interaction.InteractionRecord)
	}{
		{name: "missing"},
		{name: "not_terminal", childStatus: "processing"},
		{name: "wrong_outcome", childStatus: "failed"},
		{name: "wrong_role", childStatus: "completed", corrupt: func(c *interaction.InteractionRecord) { c.Scope.CharacterID = "role-b" }},
		{name: "wrong_space", childStatus: "completed", corrupt: func(c *interaction.InteractionRecord) { c.Scope.SpaceID = "space-b" }},
		{name: "wrong_request", childStatus: "completed", corrupt: func(c *interaction.InteractionRecord) { c.Scope.RequestID = "ma-other" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			reader, _ := round50OwnedTask("succeeded", tc.childStatus)
			if tc.corrupt != nil {
				tc.corrupt(reader.children["child-a"])
			}
			got, err := List(context.Background(), reader, "space-a", "conv-a")
			if err != nil || len(got.Tasks) != 1 {
				t.Fatalf("list: %+v, %v", got, err)
			}
			task := got.Tasks[0]
			if task.RecoveryState != "needs-reconciliation" || task.Status != "waiting" || task.CompletedCount != 0 || task.Assignments[0].Status != "waiting" || got.Summary.Succeeded != 0 {
				t.Fatalf("unproven completion exposed: %+v", task)
			}
		})
	}
}

func TestRound50WorkbenchAcceptsOnlyConfirmedChildTerminal(t *testing.T) {
	reader, _ := round50OwnedTask("succeeded", "completed")
	got, err := List(context.Background(), reader, "space-a", "conv-a")
	if err != nil || len(got.Tasks) != 1 {
		t.Fatalf("list: %+v %v", got, err)
	}
	task := got.Tasks[0]
	if task.Status != "succeeded" || task.CompletedCount != 1 || task.RecoveryState != "checkpoint-only" || got.Summary.Succeeded != 1 || reader.reads != 1 {
		t.Fatalf("valid completion not shown: %+v", task)
	}
}

func TestRound50WorkbenchFailsOnChildRecordReadError(t *testing.T) {
	reader, _ := round50OwnedTask("succeeded", "completed")
	reader.err = errors.New("tracker read failed")
	if _, err := List(context.Background(), reader, "space-a", "conv-a"); err == nil {
		t.Fatal("tracker read failure was suppressed")
	}
}

func TestRound50WorkbenchMissingTerminalDispatchStaysUnconfirmed(t *testing.T) {
	reader, parent := round50OwnedTask("succeeded", "")
	parent.RecoveryDescriptor.MultiAgent.AssignmentRefs[0].ChildInteractionID = ""
	got, err := List(context.Background(), reader, "space-a", "conv-a")
	if err != nil || got.Tasks[0].RecoveryState != "needs-reconciliation" || reader.reads != 0 {
		t.Fatalf("missing dispatch accepted: %+v %v", got, err)
	}
}

func TestRound50WorkbenchVerifiedDependentNonExecution(t *testing.T) {
	reader, parent := round50OwnedTask("failed", "")
	ref := &parent.RecoveryDescriptor.MultiAgent.AssignmentRefs[0]
	ref.ChildInteractionID = ""
	ref.Status = "failed"
	ref.Error = "dependency_failed: prerequisite failed"
	got, err := List(context.Background(), reader, "space-a", "conv-a")
	if err != nil || got.Tasks[0].RecoveryState == "needs-reconciliation" || got.Tasks[0].FailedCount != 1 {
		t.Fatalf("legitimate dependency failure lost: %+v %v", got, err)
	}
}

func TestRound50ScopedChildReaderRejectsOtherRoles(t *testing.T) {
	reader, _ := round50OwnedTask("succeeded", "completed")
	scoped := scopedRecordReader{reader: reader, role: "role-b"}
	if _, found, err := scoped.Get(context.Background(), "child-a"); err != nil || found {
		t.Fatalf("cross-role child became readable: %v %v", found, err)
	}
}

func TestRound50CheckpointHistoryRequiresTerminalProof(t *testing.T) {
	reader, _ := round50OwnedTask("succeeded", "")
	got, err := ListCheckpoints(context.Background(), reader, "space-a", "conv-a", "coord-a")
	if err != nil || len(got.Records) != 1 || got.Records[0].Consistent || got.Records[0].CompletedCount != 0 {
		t.Fatalf("checkpoint history claimed unproven result: %+v %v", got, err)
	}
	reader.children["child-a"] = interaction.NewInteractionRecord(interaction.InteractionScope{SpaceID: "space-a", CharacterID: "role-a", ConversationID: "multi-agent-worker-assignment-a", RequestID: "ma-assignment-a", Source: "multi_agent"})
	reader.children["child-a"].ID = "child-a"
	reader.children["child-a"].Status = interaction.InteractionStatusCompleted
	got, err = ListCheckpoints(context.Background(), reader, "space-a", "conv-a", "coord-a")
	if err != nil || len(got.Records) != 1 || !got.Records[0].Consistent || got.Records[0].CompletedCount != 1 {
		t.Fatalf("confirmed checkpoint hidden: %+v %v", got, err)
	}
}

func TestRound50RejectsForeignOwnedCheckpoint(t *testing.T) {
	reader, parent := round50OwnedTask("succeeded", "completed")
	parent.RecoveryDescriptor.MultiAgent.OwnedScope.RoleID = "role-other"
	got, err := List(context.Background(), reader, "space-a", "conv-a")
	if err != nil || len(got.Tasks) != 0 {
		t.Fatalf("foreign owner snapshot displayed: %+v %v", got, err)
	}
	history, err := ListCheckpoints(context.Background(), reader, "space-a", "conv-a", "coord-a")
	if err != nil || len(history.Records) != 0 {
		t.Fatalf("foreign owner checkpoint displayed: %+v %v", history, err)
	}
}
