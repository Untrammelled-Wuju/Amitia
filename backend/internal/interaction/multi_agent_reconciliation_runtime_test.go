package interaction

import (
	"context"
	"errors"
	"testing"

	"github.com/u-ai/backend/internal/decision"
)

func newReconciliationCoordinator(t *testing.T, strategy CoordinationStrategy) (*MultiAgentCoordinator, *fakeTracker, *fakeWorkerRunner, string) {
	t.Helper()
	tracker := newFakeTracker()
	if err := tracker.Create(context.Background(), &InteractionRecord{ID: "parent-reconcile", Status: InteractionStatusProcessing}); err != nil {
		t.Fatal(err)
	}
	goals := decision.NewGoalRegistry()
	if err := goals.Register(decision.Goal{ID: "goal-reconcile", Revision: 1, Status: decision.GoalStatusActive}); err != nil {
		t.Fatal(err)
	}
	workers := &fakeWorkerRunner{}
	coord := NewMultiAgentCoordinator(tracker, goals, nil, workers, nil, DefaultMultiAgentPolicy())
	result, err := coord.Start(context.Background(), StartCoordinationRequest{
		ParentInteractionID: "parent-reconcile",
		ParentGoalID:        "goal-reconcile", ParentGoalRevision: 1,
		WorkerRefs: []AgentWorkerRef{{WorkerID: "w1", CharacterID: "c1"}, {WorkerID: "w2", CharacterID: "c2"}},
		Objectives: []AssignmentObjective{{WorkerIndex: 0, Objective: "first task"}, {WorkerIndex: 1, Objective: "second task"}},
		Strategy:   strategy, CompletionPlan: CoordinationRequireAll,
	})
	if err != nil {
		t.Fatal(err)
	}
	return coord, tracker, workers, string(result.CoordinationID)
}

func TestMultiAgentReconcileAdvancesSequentialWorkerAndAggregates(t *testing.T) {
	ctx := context.Background()
	coord, tracker, workers, id := newReconciliationCoordinator(t, CoordinationSequential)
	ac := coord.coordinations[id]
	if len(workers.started) != 1 || ac.assignments[0].Status != AssignmentRunning || ac.assignments[1].Status != AssignmentPending {
		t.Fatalf("sequential start invalid: %#v", ac.assignments)
	}
	first := ac.assignments[0].ChildInteractionID
	tracker.records[first] = &InteractionRecord{ID: first, Status: InteractionStatusCompleted, ResultRef: "verified result"}
	if err := coord.Reconcile(ctx, id); err != nil {
		t.Fatal(err)
	}
	if ac.assignments[0].Status != AssignmentSucceeded || ac.assignments[1].Status != AssignmentRunning || len(workers.started) != 2 {
		t.Fatalf("successful child did not advance sequential work: %#v starts=%d", ac.assignments, len(workers.started))
	}
	second := ac.assignments[1].ChildInteractionID
	tracker.records[second] = &InteractionRecord{ID: second, Status: InteractionStatusCompleted, ResultRef: "verified result"}
	if err := coord.Reconcile(ctx, id); err != nil {
		t.Fatal(err)
	}
	if ac.status != CoordinationSucceeded {
		t.Fatalf("all child completions must aggregate: %s", ac.status)
	}
	if err := coord.Reconcile(ctx, id); err != nil {
		t.Fatal(err)
	}
	if len(workers.started) != 2 {
		t.Fatal("repeated reconciliation duplicated a child execution")
	}
}

func TestMultiAgentReconcilePropagatesFailedChild(t *testing.T) {
	ctx := context.Background()
	coord, tracker, _, id := newReconciliationCoordinator(t, CoordinationParallel)
	ac := coord.coordinations[id]
	child := ac.assignments[0].ChildInteractionID
	tracker.records[child] = &InteractionRecord{ID: child, Status: InteractionStatusFailed, ErrorCode: "BUILD_FAILED", ErrorMessage: "compiler exit 1"}
	if err := coord.Reconcile(ctx, id); err != nil {
		t.Fatal(err)
	}
	if ac.assignments[0].Status != AssignmentFailed || ac.assignments[0].Error != "BUILD_FAILED: compiler exit 1" {
		t.Fatalf("worker failure not propagated: %+v", ac.assignments[0])
	}
	other := ac.assignments[1].ChildInteractionID
	tracker.records[other] = &InteractionRecord{ID: other, Status: InteractionStatusCompleted}
	if err := coord.Reconcile(ctx, id); err != nil {
		t.Fatal(err)
	}
	if ac.status != CoordinationFailed {
		t.Fatalf("require-all coordination must fail on child error: %s", ac.status)
	}
}

func TestMultiAgentReconcilePendingFromParentDescriptor(t *testing.T) {
	ctx := context.Background()
	coord, tracker, _, id := newReconciliationCoordinator(t, CoordinationParallel)
	ac := coord.coordinations[id]
	first := ac.assignments[0].ChildInteractionID
	tracker.records[first] = &InteractionRecord{ID: first, Status: InteractionStatusCompleted}
	if err := coord.ReconcilePendingCoordinations(ctx); err != nil {
		t.Fatal(err)
	}
	if ac.assignments[0].Status != AssignmentSucceeded {
		t.Fatalf("parent recovery descriptor did not trigger child reconciliation: %s", ac.assignments[0].Status)
	}
}

type workerReturnedKnownInteractionError struct{ childID string }

func (w workerReturnedKnownInteractionError) StartWorker(context.Context, WorkerRunRequest) (string, error) {
	return w.childID, errors.New("connection interrupted after creating child interaction")
}

func TestMultiAgentPreservesCreatedChildIDWhenWorkerReturnsError(t *testing.T) {
	ctx := context.Background()
	tracker := newFakeTracker()
	_ = tracker.Create(ctx, &InteractionRecord{ID: "parent-known", Status: InteractionStatusProcessing})
	_ = tracker.Create(ctx, &InteractionRecord{
		ID: "child-known", Status: InteractionStatusFailed, ErrorCode: "REMOTE_WORKER_FAILED",
		ErrorMessage: "tool exited after workspace edit",
	})
	goals := decision.NewGoalRegistry()
	_ = goals.Register(decision.Goal{ID: "goal-known", Revision: 1, Status: decision.GoalStatusActive})
	coord := NewMultiAgentCoordinator(tracker, goals, nil, workerReturnedKnownInteractionError{childID: "child-known"}, nil, DefaultMultiAgentPolicy())
	started, err := coord.Start(ctx, StartCoordinationRequest{
		ParentInteractionID: "parent-known", ParentGoalID: "goal-known", ParentGoalRevision: 1,
		WorkerRefs: []AgentWorkerRef{{WorkerID: "worker", CharacterID: "character"}},
		Objectives: []AssignmentObjective{{WorkerIndex: 0, Objective: "Run a tool and record failure"}},
	})
	if err != nil {
		t.Fatalf("known child execution was discarded: %v", err)
	}
	parent := tracker.records["parent-known"]
	if parent.RecoveryDescriptor == nil || parent.RecoveryDescriptor.MultiAgent == nil ||
		parent.RecoveryDescriptor.MultiAgent.AssignmentRefs[0].ChildInteractionID != "child-known" {
		t.Fatal("known side-effecting child ID is missing from durable recovery")
	}
	if err := coord.Reconcile(ctx, string(started.CoordinationID)); err != nil {
		t.Fatal(err)
	}
	snapshot, err := coord.Snapshot(ctx, string(started.CoordinationID))
	if err != nil || snapshot.Status != CoordinationFailed || snapshot.Assignments[0].Error == "" {
		t.Fatalf("child failure was not reconciled safely: %+v err=%v", snapshot, err)
	}
}

func TestUnifiedWorkerRunnerRejectsMissingEntryWithoutPanic(t *testing.T) {
	r := NewUnifiedEntryWorkerRunner(nil)
	id, err := r.StartWorker(context.Background(), WorkerRunRequest{AssignmentID: "some-id"})
	if err == nil || id != "" {
		t.Fatalf("nil worker entry should fail: id=%q err=%v", id, err)
	}
}
