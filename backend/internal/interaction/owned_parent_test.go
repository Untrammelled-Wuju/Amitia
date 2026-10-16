package interaction

import (
	"errors"
	"strings"
	"testing"
)

func TestOwnedParentFinalizesOnlyAfterReplyAndDurableWorkerTerminal(t *testing.T) {
	for _, storage := range []string{"memory", "sqlite"} {
		for _, status := range []string{"running", "succeeded", "failed", "cancelled"} {
			t.Run(storage+"/"+status, func(t *testing.T) {
				var tracker InteractionTracker = NewInMemoryTracker()
				if storage == "sqlite" {
					tracker = newTestSQLiteInteractionTracker(t)
				}
				parent := NewInteractionRecord(InteractionScope{SpaceID: "core", ConversationID: "conv", CharacterID: "role", RequestID: "owned-parent/key", Channel: "device-mesh"})
				parent.RecoveryDescriptor = &RecoveryDescriptor{OwnedTurn: &OwnedTurnRecoveryRef{RequestID: "raw"}, MultiAgent: &MultiAgentRecoveryRef{CoordinationID: "task", Status: status}}
				if err := tracker.Create(t.Context(), parent); err != nil {
					t.Fatal(err)
				}
				parent, err := tracker.TransitionCAS(t.Context(), parent.ID, parent.StatusVersion, InteractionStatusProcessing)
				if err != nil {
					t.Fatal(err)
				}
				if err := FinalizeOwnedParent(t.Context(), tracker, parent.ID); err != nil {
					t.Fatal(err)
				}
				before, _, _ := tracker.Get(t.Context(), parent.ID)
				if before.IsTerminal() {
					t.Fatal("worker ended a still-generating parent reply")
				}
				marker := OwnedReplyFinishedPrefix + "raw"
				if _, err := tracker.UpdateMetadata(t.Context(), parent.ID, InteractionMetadataUpdate{ResultRef: &marker, ExpectedStatusVersion: &before.StatusVersion}); err != nil {
					t.Fatal(err)
				}
				if err := FinalizeOwnedParent(t.Context(), tracker, parent.ID); err != nil {
					t.Fatal(err)
				}
				after, _, _ := tracker.Get(t.Context(), parent.ID)
				expected := map[string]InteractionStatus{"running": InteractionStatusProcessing, "succeeded": InteractionStatusCompleted, "failed": InteractionStatusFailed, "cancelled": InteractionStatusCancelled}[status]
				if after.Status != expected {
					t.Fatalf("status=%s want=%s", after.Status, expected)
				}
				if status == "succeeded" && !strings.HasSuffix(after.ResultRef, "/requires-review") {
					t.Fatal("worker result reported as accepted code")
				}
				if err := FinalizeOwnedParent(t.Context(), tracker, parent.ID); err != nil {
					t.Fatal(err)
				}
				wrong := before.StatusVersion - 1
				if _, err := tracker.UpdateMetadata(t.Context(), parent.ID, InteractionMetadataUpdate{ResultRef: &marker, ExpectedStatusVersion: &wrong}); !errors.Is(err, ErrInteractionCASConflict) {
					t.Fatalf("stale marker overwrite accepted: %v", err)
				}
			})
		}
	}
}

func TestOwnedRegistrationRejectsCrossRoleRequestCollision(t *testing.T) {
	tracker := NewInMemoryTracker()
	first := NewInteractionRecord(InteractionScope{SpaceID: "core", CharacterID: "first", RequestID: "shared"})
	if _, created, err := RegisterInteraction(t.Context(), tracker, first); err != nil || !created {
		t.Fatalf("register=%v %v", created, err)
	}
	if _, created, err := RegisterInteraction(t.Context(), tracker, first); err != nil || created {
		t.Fatalf("retry=%v %v", created, err)
	}
	other := NewInteractionRecord(InteractionScope{SpaceID: "core", CharacterID: "second", RequestID: "shared"})
	if _, _, err := RegisterInteraction(t.Context(), tracker, other); !errors.Is(err, ErrDuplicateRequest) {
		t.Fatalf("cross-role collision=%v", err)
	}
}

func TestOwnedWorkerSuccessCannotHideUncommittedParentReply(t *testing.T) {
	tracker := newTestSQLiteInteractionTracker(t)
	parent := NewInteractionRecord(InteractionScope{SpaceID: "core", RequestID: "owned-parent/failure"})
	parent.RecoveryDescriptor = &RecoveryDescriptor{OwnedTurn: &OwnedTurnRecoveryRef{RequestID: "raw"}, MultiAgent: &MultiAgentRecoveryRef{CoordinationID: "task", Status: "running"}}
	if err := tracker.Create(t.Context(), parent); err != nil {
		t.Fatal(err)
	}
	parent, err := tracker.TransitionCAS(t.Context(), parent.ID, parent.StatusVersion, InteractionStatusProcessing)
	if err != nil {
		t.Fatal(err)
	}
	marker := OwnedReplyFailedPrefix + "raw"
	if _, err := tracker.UpdateMetadata(t.Context(), parent.ID, InteractionMetadataUpdate{ResultRef: &marker}); err != nil {
		t.Fatal(err)
	}
	if err := FinalizeOwnedParent(t.Context(), tracker, parent.ID); err != nil {
		t.Fatal(err)
	}
	stillRunning, _, _ := tracker.Get(t.Context(), parent.ID)
	if stillRunning.IsTerminal() {
		t.Fatal("failed reply cancelled background work")
	}
	stillRunning.RecoveryDescriptor.MultiAgent.Status = "succeeded"
	stillRunning.RecoveryDescriptor.ComputeFingerprint()
	if _, err := tracker.UpdateMetadata(t.Context(), parent.ID, InteractionMetadataUpdate{RecoveryDescriptor: stillRunning.RecoveryDescriptor}); err != nil {
		t.Fatal(err)
	}
	if err := FinalizeOwnedParent(t.Context(), tracker, parent.ID); err != nil {
		t.Fatal(err)
	}
	finished, _, _ := tracker.Get(t.Context(), parent.ID)
	if finished.Status != InteractionStatusFailed || finished.ErrorCode != "owned_turn_failed" {
		t.Fatalf("worker hid parent save failure: %+v", finished)
	}
}
