package interaction

import (
	"context"
	"strings"
	"testing"

	"github.com/u-ai/backend/internal/decision"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

func round48OwnedAggregate(t *testing.T) (*MultiAgentCoordinator, *InMemoryTracker, *activeCoordination) {
	t.Helper()
	ctx := context.Background()
	tracker := NewInMemoryTracker()
	parent := &InteractionRecord{ID: "round48-parent", Status: InteractionStatusProcessing,
		Scope: InteractionScope{SpaceID: "space", CharacterID: "role", ConversationID: "conversation"}}
	if err := tracker.Create(ctx, parent); err != nil {
		t.Fatal(err)
	}
	goals := decision.NewGoalRegistry()
	if err := goals.Register(decision.Goal{ID: "round48-goal", Revision: 1,
		SpaceID: "space", CharacterID: "role", ConversationID: "conversation", Status: decision.GoalStatusActive}); err != nil {
		t.Fatal(err)
	}
	c := NewMultiAgentCoordinator(tracker, goals, nil, &fakeWorkerRunner{}, nil, DefaultMultiAgentPolicy())
	owned := coordination.ExecutionScope{SpaceID: "space", RoleID: "role", CoreID: "core", ResourceOwnerID: "device"}
	ac := &activeCoordination{ownedScope: &owned, parentInteractionID: parent.ID, parentGoalID: "round48-goal",
		parentGoalRev: 1, id: "round48-coord", status: CoordinationAggregating,
		completionPlan: CoordinationRequireAll, assignments: []*AgentAssignment{{
			ID: "a1", WorkerRef: AgentWorkerRef{CharacterID: "role"},
			Status: AssignmentSucceeded, ChildInteractionID: "round48-child",
		}}}
	c.coordinations[string(ac.id)] = ac
	return c, tracker, ac
}

func round48Child(t *testing.T, tracker *InMemoryTracker, status InteractionStatus, scope InteractionScope) {
	t.Helper()
	child := &InteractionRecord{ID: "round48-child", Status: status, Scope: scope}
	if err := tracker.Create(context.Background(), child); err != nil {
		t.Fatal(err)
	}
}

func TestRound48OwnedAggregateRejectsMissingDurableChild(t *testing.T) {
	c, _, ac := round48OwnedAggregate(t)
	if err := c.aggregate(context.Background(), ac); err == nil || !strings.Contains(err.Error(), "no durable child receipt") {
		t.Fatalf("missing worker receipt was accepted: %v", err)
	}
	if ac.status != CoordinationAggregating {
		t.Fatal("parent was completed without child receipt")
	}
}

func TestRound48OwnedAggregateRejectsConflictingChildTerminalState(t *testing.T) {
	c, tracker, ac := round48OwnedAggregate(t)
	round48Child(t, tracker, InteractionStatusFailed, InteractionScope{SpaceID: "space", CharacterID: "role", ConversationID: "multi-agent-worker-a1", RequestID: "ma-a1"})
	if err := c.aggregate(context.Background(), ac); err == nil || !strings.Contains(err.Error(), "lacks matching terminal proof") {
		t.Fatalf("failed child was accepted as success: %v", err)
	}
}

func TestRound48OwnedAggregateRejectsCrossRoleReceipt(t *testing.T) {
	c, tracker, ac := round48OwnedAggregate(t)
	round48Child(t, tracker, InteractionStatusCompleted, InteractionScope{SpaceID: "space", CharacterID: "different-role", ConversationID: "multi-agent-worker-a1", RequestID: "ma-a1"})
	if err := c.aggregate(context.Background(), ac); err == nil || !strings.Contains(err.Error(), "scope differs") {
		t.Fatalf("cross-role child was accepted: %v", err)
	}
}

func TestRound48OwnedAggregateAcceptsVerifiedChild(t *testing.T) {
	c, tracker, ac := round48OwnedAggregate(t)
	round48Child(t, tracker, InteractionStatusCompleted, InteractionScope{SpaceID: "space", CharacterID: "role", ConversationID: "multi-agent-worker-a1", RequestID: "ma-a1"})
	if err := c.aggregate(context.Background(), ac); err != nil {
		t.Fatalf("durable child was not accepted: %v", err)
	}
	if ac.status != CoordinationSucceeded {
		t.Fatalf("parent status %s", ac.status)
	}
}

func TestRound48OwnedAggregateAllowsUnstartedDependencyFailure(t *testing.T) {
	c, _, ac := round48OwnedAggregate(t)
	ac.assignments[0] = &AgentAssignment{ID: "a1", Status: AssignmentFailed,
		Error: "dependency_failed: prerequisite a0 did not succeed"}
	if err := c.aggregate(context.Background(), ac); err != nil {
		t.Fatalf("unstarted dependency failure requires no execution receipt: %v", err)
	}
	if ac.status != CoordinationFailed {
		t.Fatalf("parent status %s", ac.status)
	}
}

func TestRound48OwnedAggregateRejectsUnfinishedFirstSuccess(t *testing.T) {
	c, tracker, ac := round48OwnedAggregate(t)
	ac.completionPlan = CoordinationFirstSuccess
	ac.assignments = append(ac.assignments, &AgentAssignment{ID: "a2", Status: AssignmentRunning})
	round48Child(t, tracker, InteractionStatusCompleted, InteractionScope{SpaceID: "space", CharacterID: "role", ConversationID: "multi-agent-worker-a1", RequestID: "ma-a1"})
	if err := c.aggregate(context.Background(), ac); err == nil || !strings.Contains(err.Error(), "remains running") {
		t.Fatalf("first success finalized parent while another child is executing: %v", err)
	}
}
