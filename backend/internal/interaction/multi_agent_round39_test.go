package interaction

import (
	"context"
	"errors"
	"testing"

	"github.com/u-ai/backend/internal/decision"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

func TestRound39CancelledChildRemainsIndeterminateUntilDurableTerminalProof(t *testing.T) {
	ctx := context.Background()
	c, tracker, workers, id := newReconciliationCoordinator(t, CoordinationSequential)
	ac := c.coordinations[id]
	childID := ac.assignments[0].ChildInteractionID
	if err := c.Cancel(ctx, id); err != nil {
		t.Fatal(err)
	}
	if ac.status != CoordinationWaiting || !ac.cancelRequested || ac.assignments[0].Status != AssignmentRunning || ac.assignments[1].Status != AssignmentCancelled {
		t.Fatalf("premature cancelled result: state=%s assignments=%+v", ac.status, ac.assignments)
	}
	parent := tracker.records[ac.parentInteractionID]
	if parent.RecoveryDescriptor.MultiAgent == nil || !parent.RecoveryDescriptor.MultiAgent.CancelRequested || parent.RecoveryDescriptor.MultiAgent.Status != string(CoordinationWaiting) {
		t.Fatal("cancellation intent was not checkpointed")
	}
	if len(workers.started) != 1 {
		t.Fatal("cancellation dispatched another worker")
	}
	restarted := NewMultiAgentCoordinator(tracker, c.goals, nil, &fakeWorkerRunner{}, nil, DefaultMultiAgentPolicy())
	if err := restarted.restoreCoordinationFromRecord(parent); err != nil {
		t.Fatal(err)
	}
	if err := restarted.Reconcile(ctx, id); err != nil {
		t.Fatal(err)
	}
	if restarted.coordinations[id].status != CoordinationWaiting {
		t.Fatal("restored parent invented cancellation acknowledgement")
	}
	tracker.records[childID] = &InteractionRecord{ID: childID, Status: InteractionStatusCompleted, ResultRef: "committed before cancellation"}
	if err := restarted.Reconcile(ctx, id); err != nil {
		t.Fatal(err)
	}
	latest := restarted.coordinations[id]
	if latest.status != CoordinationCancelled || latest.assignments[0].Status != AssignmentSucceeded || latest.assignments[1].Status != AssignmentCancelled {
		t.Fatalf("confirmed child was not reconciled: %+v", latest)
	}
	if got := tracker.records[ac.parentInteractionID].RecoveryDescriptor.MultiAgent.Status; got != string(CoordinationCancelled) {
		t.Fatalf("settled cancellation not durable: %q", got)
	}
}

func TestRound39RecoveredStaleCoordinatorCannotOverwriteFreshSnapshot(t *testing.T) {
	ctx := context.Background()
	tracker := NewInMemoryTracker()
	parent := &InteractionRecord{ID: "round39-parent", Status: InteractionStatusProcessing}
	if err := tracker.Create(ctx, parent); err != nil {
		t.Fatal(err)
	}
	goals := decision.NewGoalRegistry()
	if err := goals.Register(decision.Goal{ID: "round39-goal", Revision: 1, Status: decision.GoalStatusActive}); err != nil {
		t.Fatal(err)
	}
	first := NewMultiAgentCoordinator(tracker, goals, nil, &fakeWorkerRunner{}, nil, DefaultMultiAgentPolicy())
	started, err := first.Start(ctx, StartCoordinationRequest{
		ParentInteractionID: parent.ID, ParentGoalID: "round39-goal", ParentGoalRevision: 1,
		WorkerRefs: []AgentWorkerRef{{WorkerID: "one"}}, Objectives: []AssignmentObjective{{WorkerIndex: 0, Objective: "do the work"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	record, found, err := tracker.Get(ctx, parent.ID)
	if err != nil || !found {
		t.Fatalf("read parent: %v", err)
	}
	a := NewMultiAgentCoordinator(tracker, goals, nil, &fakeWorkerRunner{}, nil, DefaultMultiAgentPolicy())
	b := NewMultiAgentCoordinator(tracker, goals, nil, &fakeWorkerRunner{}, nil, DefaultMultiAgentPolicy())
	if err := a.restoreCoordinationFromRecord(record); err != nil {
		t.Fatal(err)
	}
	if err := b.restoreCoordinationFromRecord(record); err != nil {
		t.Fatal(err)
	}
	coordID := string(started.CoordinationID)
	a.coordinations[coordID].status = CoordinationWaiting
	if err := a.persistCoordinationSnapshot(ctx, a.coordinations[coordID]); err != nil {
		t.Fatal(err)
	}
	b.coordinations[coordID].status = CoordinationSucceeded
	if err := b.persistCoordinationSnapshot(ctx, b.coordinations[coordID]); !errors.Is(err, ErrInteractionCASConflict) {
		t.Fatalf("stale Core overwrote new parent: %v", err)
	}
	latest, _, err := tracker.Get(ctx, parent.ID)
	if err != nil || latest.RecoveryDescriptor.MultiAgent.Status != string(CoordinationWaiting) {
		t.Fatal("lost latest durable coordinator state")
	}
}

func TestRound39ParentOwnershipCheckedBeforeFirstDurableDispatch(t *testing.T) {
	ctx := context.Background()
	tracker := newFakeTracker()
	parent := &InteractionRecord{ID: "parent-owned", Status: InteractionStatusProcessing,
		Scope: InteractionScope{SpaceID: "space-a", CharacterID: "role-a", ConversationID: "conversation-a"}}
	if err := tracker.Create(ctx, parent); err != nil {
		t.Fatal(err)
	}
	goals := decision.NewGoalRegistry()
	if err := goals.Register(decision.Goal{ID: "goal-owned", Revision: 1, Status: decision.GoalStatusActive,
		SpaceID: "space-a", CharacterID: "role-a", ConversationID: "conversation-a"}); err != nil {
		t.Fatal(err)
	}
	worker := &fakeWorkerRunner{}
	c := NewMultiAgentCoordinator(tracker, goals, nil, worker, nil, DefaultMultiAgentPolicy())
	owned := coordination.ExecutionScope{SpaceID: "space-a", CoreID: "core-a", RoleID: "role-b", ResourceOwnerID: "device-a",
		InitiatorDeviceID: "device-a", TargetDeviceID: "device-a"}
	_, err := c.Start(ctx, StartCoordinationRequest{ParentInteractionID: "parent-owned", ParentGoalID: "goal-owned", ParentGoalRevision: 1,
		OwnedScope: &owned, WorkerRefs: []AgentWorkerRef{{WorkerID: "one", CharacterID: "role-b"}},
		Objectives: []AssignmentObjective{{WorkerIndex: 0, Objective: "must not run"}}})
	if !errors.Is(err, coordination.ErrWrongOwner) || len(worker.started) != 0 || parent.RecoveryDescriptor != nil {
		t.Fatalf("unowned parent was mutated: err=%v workers=%d descriptor=%+v", err, len(worker.started), parent.RecoveryDescriptor)
	}
}
