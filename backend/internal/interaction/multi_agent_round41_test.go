package interaction

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/u-ai/backend/internal/decision"
)

func newRound41Coordination(t *testing.T, runner AgentWorkerRunner) (*MultiAgentCoordinator, *InMemoryTracker, string) {
	t.Helper()
	ctx := context.Background()
	tracker := NewInMemoryTracker()
	if err := tracker.Create(ctx, &InteractionRecord{ID: "round41-parent", Status: InteractionStatusProcessing}); err != nil {
		t.Fatal(err)
	}
	goals := decision.NewGoalRegistry()
	if err := goals.Register(decision.Goal{ID: "round41-goal", Revision: 1, Status: decision.GoalStatusActive}); err != nil {
		t.Fatal(err)
	}
	coord := NewMultiAgentCoordinator(tracker, goals, nil, runner, nil, DefaultMultiAgentPolicy())
	started, err := coord.Start(ctx, StartCoordinationRequest{
		ParentInteractionID: "round41-parent", ParentGoalID: "round41-goal", ParentGoalRevision: 1,
		WorkerRefs: []AgentWorkerRef{{WorkerID: "worker", CharacterID: "character"}},
		Objectives: []AssignmentObjective{{WorkerIndex: 0, Objective: "durable child"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	return coord, tracker, string(started.CoordinationID)
}

func TestRound41ReloadsCurrentParentAfterCrossCoreCASConflict(t *testing.T) {
	ctx := context.Background()
	first, tracker, id := newRound41Coordination(t, &fakeWorkerRunner{})
	checkpoint, found, err := tracker.Get(ctx, "round41-parent")
	if err != nil || !found {
		t.Fatalf("read initial checkpoint: %v", err)
	}
	second := NewMultiAgentCoordinator(tracker, first.goals, nil, &fakeWorkerRunner{}, nil, DefaultMultiAgentPolicy())
	if err := second.restoreCoordinationFromRecord(checkpoint); err != nil {
		t.Fatal(err)
	}
	stale := second.coordinations[id]
	first.coordinations[id].assignments[0].Status = AssignmentWaiting
	first.coordinations[id].status = CoordinationWaiting
	if err := first.persistCoordinationSnapshot(ctx, first.coordinations[id]); err != nil {
		t.Fatal(err)
	}
	if err := second.requireCurrentCoordinationSnapshot(ctx, stale); !errors.Is(err, ErrInteractionCASConflict) {
		t.Fatalf("stale core was allowed to dispatch: %v", err)
	}
	if err := second.persistCoordinationSnapshot(ctx, stale); !errors.Is(err, ErrInteractionCASConflict) {
		t.Fatalf("stale core was allowed to overwrite: %v", err)
	}
	fresh, err := second.refreshCoordinationIfStale(ctx, stale)
	if err != nil {
		t.Fatal(err)
	}
	if fresh == stale || second.coordinations[id] != fresh || fresh.status != CoordinationWaiting || fresh.assignments[0].Status != AssignmentWaiting {
		t.Fatalf("authoritative snapshot not restored: %+v", fresh)
	}
}

func TestRound41StaleTerminalCallbackMustBeReconciledAgainstFreshSnapshot(t *testing.T) {
	ctx := context.Background()
	first, tracker, id := newRound41Coordination(t, &fakeWorkerRunner{})
	record, _, err := tracker.Get(ctx, "round41-parent")
	if err != nil {
		t.Fatal(err)
	}
	second := NewMultiAgentCoordinator(tracker, first.goals, nil, &fakeWorkerRunner{}, nil, DefaultMultiAgentPolicy())
	if err := second.restoreCoordinationFromRecord(record); err != nil {
		t.Fatal(err)
	}
	a := first.coordinations[id].assignments[0]
	first.coordinations[id].status = CoordinationWaiting
	if err := first.persistCoordinationSnapshot(ctx, first.coordinations[id]); err != nil {
		t.Fatal(err)
	}
	result := AgentAssignmentResult{AssignmentID: a.ID, ChildInteractionID: a.ChildInteractionID, Status: AssignmentFailed, Error: &AssignmentError{Code: "WORKER_FAILED", Message: "verified externally"}}
	if err := second.OnAssignmentTerminal(ctx, a.ID, result); !errors.Is(err, ErrInteractionCASConflict) {
		t.Fatalf("stale callback must be fenced: %v", err)
	}
	latest, _, err := tracker.Get(ctx, "round41-parent")
	if err != nil || latest.RecoveryDescriptor.MultiAgent.AssignmentRefs[0].Status != string(AssignmentRunning) {
		t.Fatalf("failed CAS corrupted durable child outcome: %v", err)
	}
	fresh, err := second.refreshCoordinationIfStale(ctx, second.coordinations[id])
	if err != nil {
		t.Fatal(err)
	}
	if fresh.assignments[0].Status != AssignmentRunning {
		t.Fatal("uncommitted local terminal state leaked into recovered view")
	}
	if err := second.OnAssignmentTerminal(ctx, a.ID, result); err != nil {
		t.Fatal(err)
	}
	latest, _, err = tracker.Get(ctx, "round41-parent")
	if err != nil || latest.RecoveryDescriptor.MultiAgent.AssignmentRefs[0].Status != string(AssignmentFailed) {
		t.Fatalf("fresh callback not persisted: %v", err)
	}
}

type round41UncertainRunner struct{ attempts int }

func (r *round41UncertainRunner) StartWorker(context.Context, WorkerRunRequest) (string, error) {
	r.attempts++
	return "", errors.New("transport closed before dispatch acknowledgement")
}

func TestRound41AmbiguousDispatchRemainsWaitingWithoutAutomaticReplay(t *testing.T) {
	ctx := context.Background()
	tracker := NewInMemoryTracker()
	if err := tracker.Create(ctx, &InteractionRecord{ID: "uncertain-parent", Status: InteractionStatusProcessing}); err != nil {
		t.Fatal(err)
	}
	goals := decision.NewGoalRegistry()
	if err := goals.Register(decision.Goal{ID: "uncertain-goal", Revision: 1, Status: decision.GoalStatusActive}); err != nil {
		t.Fatal(err)
	}
	runner := &round41UncertainRunner{}
	coord := NewMultiAgentCoordinator(tracker, goals, nil, runner, nil, DefaultMultiAgentPolicy())
	_, err := coord.Start(ctx, StartCoordinationRequest{
		ParentInteractionID: "uncertain-parent", ParentGoalID: "uncertain-goal", ParentGoalRevision: 1,
		WorkerRefs: []AgentWorkerRef{{WorkerID: "worker"}},
		Objectives: []AssignmentObjective{{WorkerIndex: 0, Objective: "could have side effects"}},
	})
	if err == nil || !strings.Contains(err.Error(), "requires reconciliation") {
		t.Fatalf("unknown dispatch was not reported: %v", err)
	}
	record, found, err := tracker.Get(ctx, "uncertain-parent")
	if err != nil || !found {
		t.Fatal(err)
	}
	ref := record.RecoveryDescriptor.MultiAgent
	if ref.Status != string(CoordinationWaiting) || ref.AssignmentRefs[0].Status != string(AssignmentWaiting) {
		t.Fatalf("unknown dispatch was declared terminal: %+v", ref)
	}
	recovered := NewMultiAgentCoordinator(tracker, goals, nil, runner, nil, DefaultMultiAgentPolicy())
	if err := recovered.restoreCoordinationFromRecord(record); err != nil {
		t.Fatal(err)
	}
	if err := recovered.Reconcile(ctx, ref.CoordinationID); err != nil {
		t.Fatal(err)
	}
	if runner.attempts != 1 {
		t.Fatalf("ambiguous worker was dispatched twice: %d", runner.attempts)
	}
}

func TestRound41StalePauseResumeCannotReopenTerminalParent(t *testing.T) {
	ctx := context.Background()
	first, tracker, id := newRound41Coordination(t, &fakeWorkerRunner{})
	record, _, err := tracker.Get(ctx, "round41-parent")
	if err != nil {
		t.Fatal(err)
	}
	other := NewMultiAgentCoordinator(tracker, first.goals, nil, &fakeWorkerRunner{}, nil, DefaultMultiAgentPolicy())
	if err := other.restoreCoordinationFromRecord(record); err != nil {
		t.Fatal(err)
	}
	first.coordinations[id].assignments[0].Status = AssignmentCancelled
	first.coordinations[id].status = CoordinationCancelled
	if err := first.persistCoordinationSnapshot(ctx, first.coordinations[id]); err != nil {
		t.Fatal(err)
	}
	if err := other.Pause(ctx, id); err != nil {
		t.Fatal(err)
	}
	if err := other.Resume(ctx, id); err == nil {
		t.Fatal("cancelled coordination unexpectedly accepted resume")
	}
	latest, _, err := tracker.Get(ctx, "round41-parent")
	if err != nil || latest.RecoveryDescriptor.MultiAgent.Status != string(CoordinationCancelled) {
		t.Fatalf("stale control reopened cancelled parent: %v", err)
	}
}
