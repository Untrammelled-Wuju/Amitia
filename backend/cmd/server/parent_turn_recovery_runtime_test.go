package main

import (
	"context"
	"errors"
	"testing"

	"github.com/u-ai/backend/internal/devicemesh/business"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/interaction"
)

type parentRecoveryProbe struct {
	calls  int
	result business.OwnedTurnReconciliation
	err    error
	scope  coordination.ExecutionScope
}

func (p *parentRecoveryProbe) ReconcileOwnedTurn(_ context.Context, scope coordination.ExecutionScope, conversation string) (business.OwnedTurnReconciliation, error) {
	p.calls++
	if p.scope != scope || conversation != "conversation" {
		return business.OwnedTurnReconciliation{}, coordination.ErrWrongOwner
	}
	return p.result, p.err
}

func ownedRecoveryParent(t *testing.T) (*interaction.InMemoryTracker, *interaction.InteractionRecord, coordination.ExecutionScope) {
	t.Helper()
	tracker := interaction.NewInMemoryTracker()
	scope := coordination.ExecutionScope{SpaceID: "space", CoreID: "core", RoleID: "role", ResourceOwnerID: "phone", InitiatorDeviceID: "phone", TargetDeviceID: "phone", RequestID: "raw", TurnID: "turn", ExecutionID: "execution"}
	parent := interaction.NewInteractionRecord(interaction.InteractionScope{SpaceID: "space", CharacterID: "role", ConversationID: "conversation", RequestID: "owned-parent/key", Channel: "web"})
	parent.RecoveryDescriptor = &interaction.RecoveryDescriptor{SchemaVersion: interaction.RecoveryDescriptorSchemaVersion, State: interaction.RecoveryDescriptorActive, Interaction: interaction.RecoveryInteractionRef{InteractionID: parent.ID}, Scope: interaction.RecoveryScopeRef{ConversationID: "conversation"}, OwnedTurn: &interaction.OwnedTurnRecoveryRef{Scope: scope, RequestID: "raw", InputFingerprint: "input"}}
	parent.RecoveryDescriptor.ComputeFingerprint()
	if err := tracker.Create(t.Context(), parent); err != nil {
		t.Fatal(err)
	}
	var err error
	parent, err = tracker.TransitionCAS(t.Context(), parent.ID, parent.StatusVersion, interaction.InteractionStatusProcessing)
	if err != nil {
		t.Fatal(err)
	}
	return tracker, parent, scope
}

func TestOwnedParentRecoveryCompletesOnlyOwnerConfirmedReply(t *testing.T) {
	tracker, parent, scope := ownedRecoveryParent(t)
	probe := &parentRecoveryProbe{scope: scope, result: business.OwnedTurnReconciliation{Status: "completed"}}
	if err := reconcileOwnedParents(t.Context(), tracker, probe); err != nil {
		t.Fatal(err)
	}
	saved, _, _ := tracker.Get(t.Context(), parent.ID)
	if saved.Status != interaction.InteractionStatusCompleted || saved.ResultRef != "owned-turn/raw" || probe.calls != 1 {
		t.Fatalf("parent=%+v calls=%d", saved, probe.calls)
	}
	if err := reconcileOwnedParents(t.Context(), tracker, probe); err != nil || probe.calls != 1 {
		t.Fatalf("terminal replay: calls=%d err=%v", probe.calls, err)
	}
}

func TestOwnedParentRecoveryPreservesUnknownSideEffectForReconciliation(t *testing.T) {
	tracker, parent, scope := ownedRecoveryParent(t)
	probe := &parentRecoveryProbe{scope: scope, result: business.OwnedTurnReconciliation{Status: "needs_reconciliation"}, err: business.ErrUncertainExecution}
	if err := reconcileOwnedParents(t.Context(), tracker, probe); err != nil {
		t.Fatal(err)
	}
	saved, _, _ := tracker.Get(t.Context(), parent.ID)
	if saved.IsTerminal() || saved.ResultRef != "needs_reconciliation/raw" || saved.RecoveryDescriptor.State != interaction.RecoveryDescriptorManualIntervention {
		t.Fatalf("parent=%+v", saved)
	}
}

func TestOwnedParentRecoveryRejectsTamperedDescriptorAndUnavailableOwner(t *testing.T) {
	tracker, parent, scope := ownedRecoveryParent(t)
	probe := &parentRecoveryProbe{scope: scope, err: coordination.ErrScopeExpired}
	if err := reconcileOwnedParents(t.Context(), tracker, probe); !errors.Is(err, coordination.ErrScopeExpired) {
		t.Fatalf("invalid Owner not rejected: %v", err)
	}
	saved, _, _ := tracker.Get(t.Context(), parent.ID)
	if saved.IsTerminal() || saved.ResultRef != "" {
		t.Fatal("unavailable Owner treated as confirmed")
	}
	saved.RecoveryDescriptor.OwnedTurn.Scope.ResourceOwnerID = "attacker"
	_, err := tracker.UpdateMetadata(t.Context(), saved.ID, interaction.InteractionMetadataUpdate{RecoveryDescriptor: saved.RecoveryDescriptor})
	if err != nil {
		t.Fatal(err)
	}
	probe.calls = 0
	if err := reconcileOwnedParents(t.Context(), tracker, probe); !errors.Is(err, coordination.ErrWrongOwner) || probe.calls != 0 {
		t.Fatalf("tampered descriptor reached Owner: calls=%d err=%v", probe.calls, err)
	}
}

func TestOwnedParentRecoveryDoesNotReconcileActiveGeneration(t *testing.T) {
	tracker, parent, scope := ownedRecoveryParent(t)
	probe := &parentRecoveryProbe{scope: scope, result: business.OwnedTurnReconciliation{Status: "running"}}
	if err := reconcileOwnedParents(t.Context(), tracker, probe); err != nil {
		t.Fatal(err)
	}
	saved, _, _ := tracker.Get(t.Context(), parent.ID)
	if saved.ResultRef != "" || saved.Status != interaction.InteractionStatusProcessing {
		t.Fatalf("active parent modified: %+v", saved)
	}
}
