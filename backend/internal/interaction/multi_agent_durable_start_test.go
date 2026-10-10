package interaction

import (
	"context"
	"testing"

	"github.com/u-ai/backend/internal/decision"
)

type coordinationCheckpointCapture struct {
	*InMemoryTracker
	checkpoints []MultiAgentRecoveryRef
}

func (t *coordinationCheckpointCapture) UpdateMetadata(ctx context.Context, id string, update InteractionMetadataUpdate) (*InteractionRecord, error) {
	if update.RecoveryDescriptor != nil && update.RecoveryDescriptor.MultiAgent != nil {
		saved := *update.RecoveryDescriptor.MultiAgent
		t.checkpoints = append(t.checkpoints, saved)
	}
	return t.InMemoryTracker.UpdateMetadata(ctx, id, update)
}

func TestMultiAgentFirstDurableCheckpointContainsCompleteWorkerPlan(t *testing.T) {
	ctx := context.Background()
	tracker := &coordinationCheckpointCapture{InMemoryTracker: NewInMemoryTracker()}
	parent := NewInteractionRecord(InteractionScope{
		SpaceID: "space-a", CharacterID: "character-a", ConversationID: "parent-conversation",
		Channel: "web", RequestID: "checkpoint-parent",
	})
	if err := tracker.Create(ctx, parent); err != nil {
		t.Fatal(err)
	}
	goals := decision.NewGoalRegistry()
	if err := goals.Register(decision.Goal{ID: "checkpoint-goal", Revision: 1,
		SpaceID: "space-a", CharacterID: "character-a", ConversationID: "parent-conversation",
		Status: decision.GoalStatusActive}); err != nil {
		t.Fatal(err)
	}
	coordinator := NewMultiAgentCoordinator(tracker, goals, nil, &fakeWorkerRunner{}, nil, DefaultMultiAgentPolicy())
	_, err := coordinator.Start(ctx, StartCoordinationRequest{
		ParentInteractionID: parent.ID, ParentGoalID: "checkpoint-goal", ParentGoalRevision: 1,
		Strategy: CoordinationParallel, CompletionPlan: CoordinationRequireAll,
		WorkspaceID: "workspace-a", PermissionMode: "request_approval",
		WorkerRefs: []AgentWorkerRef{{WorkerID: "worker-1", CharacterID: "character-a"}, {WorkerID: "worker-2", CharacterID: "character-a"}},
		Objectives: []AssignmentObjective{{WorkerIndex: 0, Objective: "Inspect the backend runtime"}, {WorkerIndex: 1, Objective: "Review worker lifecycle"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(tracker.checkpoints) == 0 {
		t.Fatal("no durable checkpoint recorded before worker start")
	}
	for i, checkpoint := range tracker.checkpoints {
		if checkpoint.Strategy != CoordinationParallel || checkpoint.CompletionPlan != CoordinationRequireAll ||
			checkpoint.Status == "" || checkpoint.WorkspaceID != "workspace-a" ||
			checkpoint.PermissionMode != "request_approval" || len(checkpoint.AssignmentRefs) != 2 {
			t.Fatalf("checkpoint %d missing required recovery data: %+v", i, checkpoint)
		}
		for _, assignment := range checkpoint.AssignmentRefs {
			if assignment.AssignmentID == "" || assignment.WorkerID == "" || assignment.Objective == "" ||
				assignment.CharacterID == "" {
				t.Fatalf("checkpoint %d persisted incomplete assignment: %+v", i, assignment)
			}
		}
	}
}
