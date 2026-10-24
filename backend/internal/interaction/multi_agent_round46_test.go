package interaction

import (
	"context"
	"errors"
	"testing"

	"github.com/u-ai/backend/internal/decision"
)

type round46ReservedRunner struct {
	starts int
}

func (r *round46ReservedRunner) WorkerInteractionID(req WorkerRunRequest) string {
	return "round46-child-" + req.AssignmentID
}

func (r *round46ReservedRunner) StartWorker(_ context.Context, req WorkerRunRequest) (string, error) {
	r.starts++
	return r.WorkerInteractionID(req), nil
}

func round46RecoverySetup(t *testing.T) (*MultiAgentCoordinator, *InMemoryTracker, *round46ReservedRunner, string) {
	t.Helper()
	ctx := context.Background()
	tracker := NewInMemoryTracker()
	if err := tracker.Create(ctx, &InteractionRecord{ID: "round46-parent", Status: InteractionStatusProcessing}); err != nil {
		t.Fatal(err)
	}
	goals := decision.NewGoalRegistry()
	if err := goals.Register(decision.Goal{ID: "round46-goal", Revision: 1, Status: decision.GoalStatusActive}); err != nil {
		t.Fatal(err)
	}
	runner := &round46ReservedRunner{}
	original := NewMultiAgentCoordinator(tracker, goals, nil, runner, nil, DefaultMultiAgentPolicy())
	started, err := original.Start(ctx, StartCoordinationRequest{
		ParentInteractionID: "round46-parent", ParentGoalID: "round46-goal", ParentGoalRevision: 1,
		WorkerRefs: []AgentWorkerRef{{WorkerID: "worker", CharacterID: "character"}},
		Objectives: []AssignmentObjective{{WorkerIndex: 0, Objective: "should not duplicate a recovered worker"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if runner.starts != 1 {
		t.Fatalf("initial dispatch count = %d", runner.starts)
	}
	return original, tracker, runner, string(started.CoordinationID)
}

func TestRound46RecoveredReservationRefusesStaleCrossCoreCheckpoint(t *testing.T) {
	ctx := context.Background()
	original, tracker, runner, id := round46RecoverySetup(t)
	checkpoint, found, err := tracker.Get(ctx, "round46-parent")
	if err != nil || !found {
		t.Fatalf("load checkpoint: %v", err)
	}
	recovered := NewMultiAgentCoordinator(tracker, original.goals, nil, runner, nil, DefaultMultiAgentPolicy())
	if err := recovered.restoreCoordinationFromRecord(checkpoint); err != nil {
		t.Fatal(err)
	}
	stale := recovered.coordinations[id]
	assigned := *stale.assignments[0]
	original.coordinations[id].status = CoordinationWaiting
	if err := original.persistCoordinationSnapshot(ctx, original.coordinations[id]); err != nil {
		t.Fatal(err)
	}
	if err := recovered.resumeReservedWorker(ctx, stale, assigned); !errors.Is(err, ErrInteractionCASConflict) {
		t.Fatalf("stale checkpoint resumed a reserved child: %v", err)
	}
	if runner.starts != 1 {
		t.Fatalf("duplicate dispatch on stale recovered checkpoint: %d", runner.starts)
	}
}

func TestRound46TerminalParentCannotResumeUnstartedReservedChild(t *testing.T) {
	for _, terminal := range []InteractionStatus{InteractionStatusCompleted, InteractionStatusFailed} {
		t.Run(string(terminal), func(t *testing.T) {
			ctx := context.Background()
			original, tracker, runner, id := round46RecoverySetup(t)
			checkpoint, found, err := tracker.Get(ctx, "round46-parent")
			if err != nil || !found {
				t.Fatalf("load checkpoint: %v", err)
			}
			recovered := NewMultiAgentCoordinator(tracker, original.goals, nil, runner, nil, DefaultMultiAgentPolicy())
			if err := recovered.restoreCoordinationFromRecord(checkpoint); err != nil {
				t.Fatal(err)
			}
			parent, found, err := tracker.Get(ctx, "round46-parent")
			if err != nil || !found {
				t.Fatalf("load parent: %v", err)
			}
			if _, err := tracker.TransitionCAS(ctx, parent.ID, parent.StatusVersion, terminal); err != nil {
				t.Fatal(err)
			}
			ac := recovered.coordinations[id]
			if err := recovered.resumeReservedWorker(ctx, ac, *ac.assignments[0]); err == nil {
				t.Fatal("terminal parent was allowed to resume a reserved child")
			}
			if runner.starts != 1 {
				t.Fatalf("worker started after terminal parent: %d", runner.starts)
			}
		})
	}
}

func TestRound46ActiveParentStillPermitsAuthorizedReservationRecovery(t *testing.T) {
	ctx := context.Background()
	original, tracker, runner, id := round46RecoverySetup(t)
	checkpoint, found, err := tracker.Get(ctx, "round46-parent")
	if err != nil || !found {
		t.Fatalf("load checkpoint: %v", err)
	}
	recovered := NewMultiAgentCoordinator(tracker, original.goals, nil, runner, nil, DefaultMultiAgentPolicy())
	if err := recovered.restoreCoordinationFromRecord(checkpoint); err != nil {
		t.Fatal(err)
	}
	ac := recovered.coordinations[id]
	childID := ac.assignments[0].ChildInteractionID
	if err := tracker.Create(ctx, &InteractionRecord{ID: childID, Status: InteractionStatusReceived}); err != nil {
		t.Fatal(err)
	}
	if err := recovered.resumeReservedWorker(ctx, ac, *ac.assignments[0]); err != nil {
		t.Fatalf("active reserved child cannot be recovered: %v", err)
	}
	if runner.starts != 2 {
		t.Fatalf("authorized reservation was not delegated: %d", runner.starts)
	}
}

func TestRound46PausedParentDoesNotAuthorizeNewWorkerDispatch(t *testing.T) {
	for _, paused := range []InteractionStatus{InteractionStatusPausing, InteractionStatusPaused} {
		t.Run(string(paused), func(t *testing.T) {
			ctx := context.Background()
			original, tracker, runner, id := round46RecoverySetup(t)
			checkpoint, found, err := tracker.Get(ctx, "round46-parent")
			if err != nil || !found {
				t.Fatalf("load checkpoint: %v", err)
			}
			recovered := NewMultiAgentCoordinator(tracker, original.goals, nil, runner, nil, DefaultMultiAgentPolicy())
			if err := recovered.restoreCoordinationFromRecord(checkpoint); err != nil {
				t.Fatal(err)
			}
			parent, found, err := tracker.Get(ctx, "round46-parent")
			if err != nil || !found {
				t.Fatalf("load parent: %v", err)
			}
			pausing, err := tracker.TransitionCAS(ctx, parent.ID, parent.StatusVersion, InteractionStatusPausing)
			if err != nil {
				t.Fatal(err)
			}
			if paused == InteractionStatusPaused {
				if _, err := tracker.TransitionCAS(ctx, parent.ID, pausing.StatusVersion, paused); err != nil {
					t.Fatal(err)
				}
			}
			ac := recovered.coordinations[id]
			if err := recovered.resumeReservedWorker(ctx, ac, *ac.assignments[0]); err == nil {
				t.Fatal("paused parent was allowed to resume a worker")
			}
			if runner.starts != 1 {
				t.Fatalf("paused parent started a new worker: %d", runner.starts)
			}
		})
	}
}

func TestRound46MissingChildReservationIsNeverReplayed(t *testing.T) {
	ctx := context.Background()
	original, tracker, runner, id := round46RecoverySetup(t)
	checkpoint, found, err := tracker.Get(ctx, "round46-parent")
	if err != nil || !found {
		t.Fatalf("read checkpoint: %v", err)
	}
	recovered := NewMultiAgentCoordinator(tracker, original.goals, nil, runner, nil, DefaultMultiAgentPolicy())
	if err := recovered.restoreCoordinationFromRecord(checkpoint); err != nil {
		t.Fatal(err)
	}
	if err := recovered.Reconcile(ctx, id); err != nil {
		t.Fatal(err)
	}
	ac := recovered.coordinations[id]
	if ac.status != CoordinationWaiting || ac.assignments[0].Status != AssignmentWaiting || runner.starts != 1 {
		t.Fatalf("missing reservation caused replay or incorrect completion: state=%s child=%s starts=%d", ac.status, ac.assignments[0].Status, runner.starts)
	}
	parent, found, err := tracker.Get(ctx, "round46-parent")
	if err != nil || !found || parent.RecoveryDescriptor == nil || parent.RecoveryDescriptor.MultiAgent == nil {
		t.Fatalf("missing durable waiting checkpoint: %v", err)
	}
	if got := parent.RecoveryDescriptor.MultiAgent.AssignmentRefs[0].Status; got != string(AssignmentWaiting) {
		t.Fatalf("unknown child status was not durably marked waiting: %s", got)
	}
	if err := recovered.Reconcile(ctx, id); err != nil {
		t.Fatal(err)
	}
	if runner.starts != 1 {
		t.Fatal("second reconciliation replayed unknown child")
	}
}

func TestRound46DirectResumeRefusesMissingDurableChild(t *testing.T) {
	ctx := context.Background()
	original, tracker, runner, id := round46RecoverySetup(t)
	checkpoint, found, err := tracker.Get(ctx, "round46-parent")
	if err != nil || !found {
		t.Fatal(err)
	}
	recovered := NewMultiAgentCoordinator(tracker, original.goals, nil, runner, nil, DefaultMultiAgentPolicy())
	if err := recovered.restoreCoordinationFromRecord(checkpoint); err != nil {
		t.Fatal(err)
	}
	ac := recovered.coordinations[id]
	if err := recovered.resumeReservedWorker(ctx, ac, *ac.assignments[0]); err == nil {
		t.Fatal("replayed worker without durable reservation")
	}
	if runner.starts != 1 {
		t.Fatalf("unsafe replay: %d", runner.starts)
	}
}
