package workbenchtasks

import (
	"context"
	"testing"

	"github.com/u-ai/backend/internal/interaction"
)

func TestRound57RejectsOwnerAndCoreDriftAcrossDuplicatedCheckpointScope(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(*interaction.MultiAgentRecoveryRef)
	}{
		{"owner_mismatch", func(c *interaction.MultiAgentRecoveryRef) { c.ResourceOwnerID = "owner-forged" }},
		{"core_mismatch", func(c *interaction.MultiAgentRecoveryRef) { c.ExecutionCoreID = "core-forged" }},
		{"goal_space_mismatch", func(c *interaction.MultiAgentRecoveryRef) { c.ParentGoalSpaceID = "other-space" }},
		{"goal_role_mismatch", func(c *interaction.MultiAgentRecoveryRef) { c.ParentGoalCharacterID = "other-role" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			reader, parent := round50OwnedTask("succeeded", "completed")
			scope := parent.RecoveryDescriptor.MultiAgent.OwnedScope
			scope.CoreID = "core-a"
			scope.ResourceOwnerID = "owner-a"
			checkpoint := parent.RecoveryDescriptor.MultiAgent
			checkpoint.ExecutionCoreID = "core-a"
			checkpoint.ResourceOwnerID = "owner-a"
			tc.mutate(checkpoint)
			if validateCheckpoint(checkpoint) {
				t.Fatal("conflicting owned metadata was trusted")
			}
			got, err := List(context.Background(), reader, "space-a", "conv-a")
			if err != nil || len(got.Tasks) != 0 {
				t.Fatalf("forged task projected: %+v %v", got, err)
			}
			history, err := ListCheckpoints(context.Background(), reader, "space-a", "conv-a", "coord-a")
			if err != nil || len(history.Records) != 0 {
				t.Fatalf("forged history projected: %+v %v", history, err)
			}
		})
	}
}

func TestRound57ValidOwnedScopeStillProjectsProof(t *testing.T) {
	reader, parent := round50OwnedTask("succeeded", "completed")
	scope := parent.RecoveryDescriptor.MultiAgent.OwnedScope
	scope.CoreID = "core-a"
	scope.ResourceOwnerID = "owner-a"
	ref := parent.RecoveryDescriptor.MultiAgent
	ref.ExecutionCoreID = scope.CoreID
	ref.ResourceOwnerID = scope.ResourceOwnerID
	if !validateCheckpoint(ref) {
		t.Fatal("valid scope rejected")
	}
	got, err := List(context.Background(), reader, "space-a", "conv-a")
	if err != nil || len(got.Tasks) != 1 || got.Tasks[0].Status != "succeeded" {
		t.Fatalf("legitimate task lost: %+v %v", got, err)
	}
}
