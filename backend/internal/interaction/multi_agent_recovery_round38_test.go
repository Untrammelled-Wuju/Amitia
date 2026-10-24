package interaction

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/u-ai/backend/internal/decision"
)

func TestRound38StaleTerminalParentConvergesAfterRestart(t *testing.T) {
	ctx := context.Background()
	c, tracker, _, id := newReconciliationCoordinator(t, CoordinationParallel)
	ac := c.coordinations[id]
	for _, a := range ac.assignments {
		a.Status = AssignmentSucceeded
	}
	ac.status = CoordinationRunning
	if err := c.persistCoordinationSnapshot(ctx, ac); err != nil {
		t.Fatal(err)
	}

	restarted := NewMultiAgentCoordinator(tracker, c.goals, nil, &fakeWorkerRunner{}, nil, DefaultMultiAgentPolicy())
	if err := restarted.restoreCoordinationFromRecord(tracker.records[ac.parentInteractionID]); err != nil {
		t.Fatal(err)
	}
	if err := restarted.Reconcile(ctx, id); err != nil {
		t.Fatal(err)
	}
	got := restarted.coordinations[id]
	if got == nil || got.status != CoordinationSucceeded || tracker.records[ac.parentInteractionID].RecoveryDescriptor.MultiAgent.Status != string(CoordinationSucceeded) {
		t.Fatalf("terminal worker proofs left restored parent stale: %+v", got)
	}
}

func TestRound38WaitingParentResumesOnlyAfterManualWaiterConverges(t *testing.T) {
	ctx := context.Background()
	c, tracker, workers, id := newReconciliationCoordinator(t, CoordinationSequential)
	ac := c.coordinations[id]
	ac.status = CoordinationWaiting
	ac.assignments[0].Status = AssignmentWaiting
	if err := c.persistCoordinationSnapshot(ctx, ac); err != nil {
		t.Fatal(err)
	}
	if err := c.Reconcile(ctx, id); err != nil {
		t.Fatal(err)
	}
	if ac.status != CoordinationWaiting || len(workers.started) != 1 {
		t.Fatal("unknown child was retried or subsequent worker started")
	}
	child := ac.assignments[0].ChildInteractionID
	tracker.records[child] = &InteractionRecord{ID: child, Status: InteractionStatusCompleted}
	if err := c.Reconcile(ctx, id); err != nil {
		t.Fatal(err)
	}
	if ac.status != CoordinationRunning || ac.assignments[0].Status != AssignmentSucceeded || ac.assignments[1].Status != AssignmentRunning || len(workers.started) != 2 {
		t.Fatalf("waiting graph failed to converge after verified terminal child: state=%s assignments=%+v starts=%d", ac.status, ac.assignments, len(workers.started))
	}
}

type round38CheckpointWorker struct {
	tracker       *fakeTracker
	parentID      string
	predecessorID string
	observed      bool
}

func (w *round38CheckpointWorker) StartWorker(_ context.Context, req WorkerRunRequest) (string, error) {
	parent := w.tracker.records[w.parentID]
	if parent != nil && parent.RecoveryDescriptor != nil && parent.RecoveryDescriptor.MultiAgent != nil {
		for _, saved := range parent.RecoveryDescriptor.MultiAgent.AssignmentRefs {
			if saved.AssignmentID == w.predecessorID && saved.Status == string(AssignmentSucceeded) {
				w.observed = true
			}
		}
	}
	return "child-" + req.AssignmentID, nil
}

func TestRound38PredecessorMustBeCheckpointedBeforeDependentWorkerStarts(t *testing.T) {
	ctx := context.Background()
	c, tracker, _, id := newReconciliationCoordinator(t, CoordinationSequential)
	ac := c.coordinations[id]
	worker := &round38CheckpointWorker{tracker: tracker, parentID: ac.parentInteractionID, predecessorID: ac.assignments[0].ID}
	c.starter = worker
	first := ac.assignments[0]
	if err := c.OnAssignmentTerminal(ctx, first.ID, AgentAssignmentResult{
		AssignmentID: first.ID, ChildInteractionID: first.ChildInteractionID, Status: AssignmentSucceeded,
	}); err != nil {
		t.Fatal(err)
	}
	if !worker.observed {
		t.Fatal("dependent worker ran before predecessor terminal state reached durable parent checkpoint")
	}
}

func TestRound38SuspendedParentCannotStartOrResumeWorker(t *testing.T) {
	ctx := context.Background()
	for _, status := range []decision.GoalStatus{decision.GoalStatusSuspended, decision.GoalStatusAchieved, decision.GoalStatusAbandoned, decision.GoalStatusWish} {
		c := newTestCoordinator()
		if err := c.goals.Register(decision.Goal{ID: "goal-1", Revision: 1, Status: status}); err != nil {
			t.Fatal(err)
		}
		_, err := c.Start(ctx, StartCoordinationRequest{ParentGoalID: "goal-1", ParentGoalRevision: 1,
			WorkerRefs: []AgentWorkerRef{{WorkerID: "worker"}}, Objectives: []AssignmentObjective{{WorkerIndex: 0, Objective: "unsafe dispatch"}},
		})
		if err == nil || !strings.Contains(err.Error(), "not active") {
			t.Fatalf("parent status %s accepted unauthorized initial dispatch: %v", status, err)
		}
	}
	c, tracker, workers, id := newReconciliationCoordinator(t, CoordinationSequential)
	ac := c.coordinations[id]
	if err := c.goals.Register(decision.Goal{ID: "goal-reconcile", Revision: 1, Status: decision.GoalStatusSuspended}); err != nil {
		t.Fatal(err)
	}
	first := ac.assignments[0]
	err := c.OnAssignmentTerminal(ctx, first.ID, AgentAssignmentResult{AssignmentID: first.ID,
		ChildInteractionID: first.ChildInteractionID, Status: AssignmentSucceeded})
	if err == nil || !strings.Contains(err.Error(), "not active") || len(workers.started) != 1 {
		t.Fatalf("suspended parent dispatched dependent worker: %v starts=%d", err, len(workers.started))
	}
	if got := tracker.records[ac.parentInteractionID].RecoveryDescriptor.MultiAgent.AssignmentRefs[0].Status; got != string(AssignmentSucceeded) {
		t.Fatalf("finished child checkpoint lost on blocked dependent dispatch: %s", got)
	}
}

type round38RejectCheckpointTracker struct {
	*fakeTracker
	reject bool
}

func (t *round38RejectCheckpointTracker) UpdateMetadata(ctx context.Context, id string, update InteractionMetadataUpdate) (*InteractionRecord, error) {
	if t.reject {
		return nil, errors.New("checkpoint store unavailable")
	}
	return t.fakeTracker.UpdateMetadata(ctx, id, update)
}

func TestRound38CheckpointFailurePreventsDependentDispatch(t *testing.T) {
	ctx := context.Background()
	c, tracker, workers, id := newReconciliationCoordinator(t, CoordinationSequential)
	ac := c.coordinations[id]
	wrapped := &round38RejectCheckpointTracker{fakeTracker: tracker, reject: true}
	c.tracker = wrapped
	first := ac.assignments[0]
	err := c.OnAssignmentTerminal(ctx, first.ID, AgentAssignmentResult{
		AssignmentID: first.ID, ChildInteractionID: first.ChildInteractionID, Status: AssignmentSucceeded,
	})
	if err == nil || !strings.Contains(err.Error(), "persist terminal worker before dispatch") {
		t.Fatalf("checkpoint failure was not propagated: %v", err)
	}
	if len(workers.started) != 1 || ac.assignments[1].Status != AssignmentPending {
		t.Fatalf("dependent worker dispatched despite failed predecessor checkpoint: starts=%d next=%s", len(workers.started), ac.assignments[1].Status)
	}
	if tracker.records[ac.parentInteractionID].RecoveryDescriptor.MultiAgent.AssignmentRefs[0].Status == string(AssignmentSucceeded) {
		t.Fatal("failed checkpoint was incorrectly reported as durable")
	}
}
