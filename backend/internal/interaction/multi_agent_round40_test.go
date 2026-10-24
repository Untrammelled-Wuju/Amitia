package interaction

import (
	"context"
	"strings"
	"testing"

	"github.com/u-ai/backend/internal/decision"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

type round40WorkerRunner struct {
	cancellations []string
}

func (w *round40WorkerRunner) StartWorker(_ context.Context, req WorkerRunRequest) (string, error) {
	return w.WorkerInteractionID(req), nil
}

func (w *round40WorkerRunner) WorkerInteractionID(req WorkerRunRequest) string {
	return "child-" + req.AssignmentID
}

func (w *round40WorkerRunner) CancelWorker(_ context.Context, childID string) error {
	w.cancellations = append(w.cancellations, childID)
	return nil
}

type round40CancelProbe struct {
	InteractionTracker
	requests []string
}

func (t *round40CancelProbe) RequestCancel(ctx context.Context, id, reason string) error {
	t.requests = append(t.requests, id)
	return t.InteractionTracker.RequestCancel(ctx, id, reason)
}

func TestRound40OwnedTerminalRequiresMatchingDurableChildProof(t *testing.T) {
	ctx := context.Background()
	c, tracker, workers, id := newReconciliationCoordinator(t, CoordinationParallel)
	ac := c.coordinations[id]
	ac.ownedScope = &coordination.ExecutionScope{SpaceID: "space-40", CoreID: "core-40", RoleID: "role-40", ResourceOwnerID: "device-40", InitiatorDeviceID: "device-40", TargetDeviceID: "device-40"}
	tracker.records[ac.parentInteractionID].Scope = InteractionScope{SpaceID: "space-40", CharacterID: "role-40", ConversationID: "conversation-40"}
	if err := c.goals.Register(decision.Goal{ID: ac.parentGoalID, Revision: ac.parentGoalRev, Status: decision.GoalStatusActive, SpaceID: "space-40", CharacterID: "role-40", ConversationID: "conversation-40"}); err != nil {
		t.Fatal(err)
	}
	ac.assignments[0].WorkerRef.CharacterID = "role-40"
	first := ac.assignments[0]
	result := AgentAssignmentResult{AssignmentID: first.ID, ChildInteractionID: first.ChildInteractionID, Status: AssignmentSucceeded}
	if err := c.OnAssignmentTerminal(ctx, first.ID, result); err == nil || !strings.Contains(err.Error(), "proof is missing") {
		t.Fatalf("unproven success was accepted: %v", err)
	}
	if first.Status != AssignmentRunning || len(workers.started) != 2 {
		t.Fatal("missing proof advanced the coordinator")
	}
	tracker.records[first.ChildInteractionID] = &InteractionRecord{ID: first.ChildInteractionID, Status: InteractionStatusCompleted, Scope: InteractionScope{SpaceID: "other-space", CharacterID: "role-40", ConversationID: "multi-agent-worker-" + first.ID, RequestID: "ma-" + first.ID}}
	if err := c.OnAssignmentTerminal(ctx, first.ID, result); err == nil || !strings.Contains(err.Error(), "scope differs") {
		t.Fatalf("foreign child completion was accepted: %v", err)
	}
	tracker.records[first.ChildInteractionID].Scope.SpaceID = "space-40"
	tracker.records[first.ChildInteractionID].Status = InteractionStatusProcessing
	if err := c.OnAssignmentTerminal(ctx, first.ID, result); err == nil || !strings.Contains(err.Error(), "terminal") {
		t.Fatalf("nonterminal child completion was accepted: %v", err)
	}
	tracker.records[first.ChildInteractionID].Status = InteractionStatusCompleted
	if err := c.OnAssignmentTerminal(ctx, first.ID, result); err != nil {
		t.Fatalf("confirmed child result was rejected: %v", err)
	}
	if first.Status != AssignmentSucceeded {
		t.Fatalf("committed completion was lost: %s", first.Status)
	}
}

func TestRound40CancellationRefusesForgedOwnedChildID(t *testing.T) {
	ctx := context.Background()
	c, tracker, _, id := newReconciliationCoordinator(t, CoordinationSequential)
	ac := c.coordinations[id]
	ac.ownedScope = &coordination.ExecutionScope{SpaceID: "space-40", CoreID: "core-40", RoleID: "role-40", ResourceOwnerID: "device-40", InitiatorDeviceID: "device-40", TargetDeviceID: "device-40"}
	ac.assignments[0].ChildInteractionID = "foreign-child-interaction"
	worker := &round40WorkerRunner{}
	probe := &round40CancelProbe{InteractionTracker: tracker}
	c.starter = worker
	c.tracker = probe
	if err := c.Cancel(ctx, id); err == nil || !strings.Contains(err.Error(), "identity cannot be verified") {
		t.Fatalf("forged child cancellation should be rejected: %v", err)
	}
	if len(worker.cancellations) != 0 {
		t.Fatal("foreign child stop was dispatched")
	}
	for _, requested := range probe.requests {
		if requested == "foreign-child-interaction" {
			t.Fatal("foreign child cancellation was persisted")
		}
	}
	if ac.status != CoordinationWaiting || ac.assignments[0].Status != AssignmentRunning {
		t.Fatal("rejected cancellation fabricated a child terminal result")
	}
}

func TestRound40ResumeStaleGoalRequestsCancellationUntilChildReceipt(t *testing.T) {
	ctx := context.Background()
	c, tracker, workers, id := newReconciliationCoordinator(t, CoordinationSequential)
	ac := c.coordinations[id]
	ac.status = CoordinationPaused
	if err := c.persistCoordinationSnapshot(ctx, ac); err != nil {
		t.Fatal(err)
	}
	if err := c.goals.Register(decision.Goal{ID: ac.parentGoalID, Revision: ac.parentGoalRev + 1, Status: decision.GoalStatusActive}); err != nil {
		t.Fatal(err)
	}
	if err := c.Resume(ctx, id); err != nil {
		t.Fatal(err)
	}
	if ac.status != CoordinationWaiting || !ac.cancelRequested || ac.assignments[0].Status != AssignmentRunning || len(workers.started) != 1 {
		t.Fatal("stale goal resumed or fabricated cancellation acknowledgement")
	}
	if !tracker.records[ac.parentInteractionID].RecoveryDescriptor.MultiAgent.CancelRequested {
		t.Fatal("parent cancellation was not persisted")
	}
}

func TestRound40OwnedCancellationUsesDurableParentAfterGoalRevisionChange(t *testing.T) {
	ctx := context.Background()
	c, tracker, _, id := newReconciliationCoordinator(t, CoordinationSequential)
	ac := c.coordinations[id]
	ac.status = CoordinationPaused
	ac.ownedScope = &coordination.ExecutionScope{SpaceID: "space-40", CoreID: "core-40", RoleID: "role-40", ResourceOwnerID: "device-40", InitiatorDeviceID: "device-40", TargetDeviceID: "device-40"}
	tracker.records[ac.parentInteractionID].Scope = InteractionScope{SpaceID: "space-40", CharacterID: "role-40", ConversationID: "conversation-40"}
	worker := &round40WorkerRunner{}
	c.starter = worker
	child := ac.assignments[0]
	child.WorkerRef.CharacterID = "role-40"
	tracker.records[child.ChildInteractionID] = &InteractionRecord{ID: child.ChildInteractionID, Status: InteractionStatusProcessing, Scope: InteractionScope{
		SpaceID: "space-40", CharacterID: "role-40", ConversationID: "multi-agent-worker-" + child.ID, RequestID: "ma-" + child.ID,
	}}
	if err := c.persistCoordinationSnapshot(ctx, ac); err != nil {
		t.Fatal(err)
	}
	if err := c.goals.Register(decision.Goal{ID: ac.parentGoalID, Revision: ac.parentGoalRev + 1, Status: decision.GoalStatusActive}); err != nil {
		t.Fatal(err)
	}
	if err := c.Resume(ctx, id); err != nil {
		t.Fatalf("revoked goal prevented cancellation delivery: %v", err)
	}
	if len(worker.cancellations) != 1 || worker.cancellations[0] != child.ChildInteractionID {
		t.Fatal("cancel request did not reach the correct reserved worker")
	}
	if ac.status != CoordinationWaiting || child.Status != AssignmentRunning || !ac.cancelRequested {
		t.Fatal("unacknowledged cancellation incorrectly settled")
	}
}
