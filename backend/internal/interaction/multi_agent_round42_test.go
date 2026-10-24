package interaction

import (
	"context"
	"strings"
	"testing"

	"github.com/u-ai/backend/internal/decision"
)

func TestRound42UnknownDispatchCannotBeAcknowledgedAsCancelled(t *testing.T) {
	ctx := context.Background()
	tracker := NewInMemoryTracker()
	if err := tracker.Create(ctx, &InteractionRecord{ID: "round42-parent", Status: InteractionStatusProcessing}); err != nil {
		t.Fatal(err)
	}
	goals := decision.NewGoalRegistry()
	if err := goals.Register(decision.Goal{ID: "round42-goal", Revision: 1, Status: decision.GoalStatusActive}); err != nil {
		t.Fatal(err)
	}
	runner := &round41UncertainRunner{}
	c := NewMultiAgentCoordinator(tracker, goals, nil, runner, nil, DefaultMultiAgentPolicy())
	_, err := c.Start(ctx, StartCoordinationRequest{
		ParentInteractionID: "round42-parent", ParentGoalID: "round42-goal", ParentGoalRevision: 1,
		WorkerRefs: []AgentWorkerRef{{WorkerID: "round42-worker"}},
		Objectives: []AssignmentObjective{{WorkerIndex: 0, Objective: "effect possibly started without acknowledgement"}},
	})
	if err == nil {
		t.Fatal("expected indeterminate dispatch")
	}
	parent, found, err := tracker.Get(ctx, "round42-parent")
	if err != nil || !found || parent.RecoveryDescriptor == nil || parent.RecoveryDescriptor.MultiAgent == nil {
		t.Fatalf("missing durable dispatch: %v", err)
	}
	id := parent.RecoveryDescriptor.MultiAgent.CoordinationID
	if err := c.Cancel(ctx, id); err != nil {
		t.Fatal(err)
	}
	parent, _, err = tracker.Get(ctx, "round42-parent")
	if err != nil {
		t.Fatal(err)
	}
	ref := parent.RecoveryDescriptor.MultiAgent
	if !ref.CancelRequested || ref.Status != string(CoordinationWaiting) || len(ref.AssignmentRefs) != 1 ||
		ref.AssignmentRefs[0].Status != string(AssignmentWaiting) || !strings.Contains(ref.AssignmentRefs[0].Error, "indeterminate") {
		t.Fatalf("unknown effects incorrectly acknowledged as cancelled: %+v", ref)
	}
	recovered := NewMultiAgentCoordinator(tracker, goals, nil, runner, nil, DefaultMultiAgentPolicy())
	if err := recovered.restoreCoordinationFromRecord(parent); err != nil {
		t.Fatal(err)
	}
	if err := recovered.Reconcile(ctx, id); err != nil {
		t.Fatal(err)
	}
	if runner.attempts != 1 {
		t.Fatalf("indeterminate child was replayed: %d", runner.attempts)
	}
	if recovered.coordinations[id].status != CoordinationWaiting {
		t.Fatal("restored cancellation invented an acknowledgement")
	}
}

func TestRound42InterruptedChildCannotConfirmCancelOrReleaseDependent(t *testing.T) {
	ctx := context.Background()
	c, tracker, workers, id := newReconciliationCoordinator(t, CoordinationSequential)
	ac := c.coordinations[id]
	child := ac.assignments[0]
	tracker.records[child.ChildInteractionID] = &InteractionRecord{
		ID: child.ChildInteractionID, Status: InteractionStatusInterrupted,
	}
	if err := c.Cancel(ctx, id); err != nil {
		t.Fatal(err)
	}
	if ac.status != CoordinationWaiting || !ac.cancelRequested || child.Status != AssignmentWaiting ||
		!strings.Contains(child.Error, "external effects") || ac.assignments[1].Status != AssignmentCancelled {
		t.Fatalf("interrupted effects falsely confirmed: %+v", ac)
	}
	parent := tracker.records[ac.parentInteractionID]
	recovered := NewMultiAgentCoordinator(tracker, c.goals, nil, workers, nil, DefaultMultiAgentPolicy())
	if err := recovered.restoreCoordinationFromRecord(parent); err != nil {
		t.Fatal(err)
	}
	if err := recovered.Reconcile(ctx, id); err != nil {
		t.Fatal(err)
	}
	if recovered.coordinations[id].status != CoordinationWaiting || len(workers.started) != 1 {
		t.Fatal("interrupted child was replayed or cancellation confirmed after restart")
	}
}

func TestRound42InterruptedChildReconcileDoesNotInventFailureOrRestart(t *testing.T) {
	ctx := context.Background()
	c, tracker, workers, id := newReconciliationCoordinator(t, CoordinationSequential)
	ac := c.coordinations[id]
	child := ac.assignments[0]
	tracker.records[child.ChildInteractionID] = &InteractionRecord{ID: child.ChildInteractionID, Status: InteractionStatusInterrupted}
	if err := c.Reconcile(ctx, id); err != nil {
		t.Fatal(err)
	}
	if child.Status != AssignmentWaiting || ac.status != CoordinationWaiting || len(workers.started) != 1 {
		t.Fatalf("unreconciled effects moved the task: %+v", ac)
	}
	if err := c.Reconcile(ctx, id); err != nil {
		t.Fatal(err)
	}
	if len(workers.started) != 1 || ac.assignments[1].Status != AssignmentPending {
		t.Fatal("repeated reconciliation launched a dependent worker")
	}
}
