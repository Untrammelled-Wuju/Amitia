package interaction

import (
	"context"
	"testing"
)

func TestInteractionMemoryTrackerPersistsRecoveryDescriptorAndSnapshot(t *testing.T) {
	ctx := context.Background()
	tracker := NewInMemoryTracker()
	parent := NewInteractionRecord(InteractionScope{SpaceID: "space", ConversationID: "conversation", CharacterID: "character", RequestID: "req"})
	if err := tracker.Create(ctx, parent); err != nil {
		t.Fatal(err)
	}
	desc := &RecoveryDescriptor{MultiAgent: &MultiAgentRecoveryRef{
		CoordinationID: "coord", ParentGoalID: "goal", ParentGoalRevision: 1,
		Status: string(CoordinationRunning), AssignmentRefs: []AssignmentRecoveryRef{{AssignmentID: "a", WorkerID: "worker", Objective: "review source", Status: string(AssignmentRunning)}},
	}}
	desc.ComputeFingerprint()
	if _, err := tracker.UpdateMetadata(ctx, parent.ID, InteractionMetadataUpdate{RecoveryDescriptor: desc}); err != nil {
		t.Fatal(err)
	}
	got, found, err := tracker.GetByRequestID(ctx, "space", "req")
	if err != nil || !found || got.RecoveryDescriptor == nil || got.RecoveryDescriptor.MultiAgent == nil {
		t.Fatalf("recovery metadata disappeared on snapshot: found=%v err=%v record=%+v", found, err, got)
	}
	got.RecoveryDescriptor.MultiAgent.AssignmentRefs[0].Objective = "tampered"
	got2, found, err := tracker.Get(ctx, parent.ID)
	if err != nil || !found {
		t.Fatal(err)
	}
	if got2.RecoveryDescriptor.MultiAgent.AssignmentRefs[0].Objective != "review source" {
		t.Fatal("returned snapshot unexpectedly mutated stored recovery metadata")
	}
}
