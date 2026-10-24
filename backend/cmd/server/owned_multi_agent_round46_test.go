package main

import (
	"context"
	"testing"

	"github.com/u-ai/backend/internal/interaction"
)

func TestRound46OwnedWorkerRefusesPausedParent(t *testing.T) {
	for _, status := range []interaction.InteractionStatus{interaction.InteractionStatusPausing, interaction.InteractionStatusPaused} {
		t.Run(string(status), func(t *testing.T) {
			ctx := context.Background()
			tracker := interaction.NewInMemoryTracker()
			registerOwnedWorkerParent(t, tracker)
			parent, found, err := tracker.Get(ctx, "parent-a")
			if err != nil || !found {
				t.Fatalf("read parent: %v", err)
			}
			processing, err := tracker.TransitionCAS(ctx, parent.ID, parent.StatusVersion, interaction.InteractionStatusProcessing)
			if err != nil {
				t.Fatal(err)
			}
			pausing, err := tracker.TransitionCAS(ctx, parent.ID, processing.StatusVersion, interaction.InteractionStatusPausing)
			if err != nil {
				t.Fatal(err)
			}
			if status == interaction.InteractionStatusPaused {
				if _, err := tracker.TransitionCAS(ctx, parent.ID, pausing.StatusVersion, status); err != nil {
					t.Fatal(err)
				}
			}
			runner := newOwnedMultiAgentWorkerRunner(nil, tracker)
			if _, err := runner.verifyWorkerParent(ctx, ownedWorkerFixture()); err == nil {
				t.Fatal("owned worker accepted paused parent")
			}
		})
	}
}
