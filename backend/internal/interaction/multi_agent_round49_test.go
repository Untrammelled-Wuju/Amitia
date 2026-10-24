package interaction

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/decision"
)

func TestRound49OwnedAggregateRejectsUnprovedCancellation(t *testing.T) {
	c, _, ac := round48OwnedAggregate(t)
	ac.cancelRequested = true
	ac.assignments[0] = &AgentAssignment{ID: "a1", Status: AssignmentCancelled, Error: "worker dispatch outcome indeterminate"}
	if err := c.aggregate(context.Background(), ac); err == nil || !strings.Contains(err.Error(), "no durable terminal child") {
		t.Fatalf("unproved cancellation accepted: %v", err)
	}
	if ac.status != CoordinationAggregating {
		t.Fatalf("unproved cancellation committed: %s", ac.status)
	}
}

func TestRound49OwnedAggregateAcceptsUnstartedCancelledWorker(t *testing.T) {
	c, _, ac := round48OwnedAggregate(t)
	ac.cancelRequested = true
	ac.assignments[0] = &AgentAssignment{ID: "a1", Status: AssignmentCancelled, Error: "cancelled before worker dispatch"}
	if err := c.verifyOwnedAggregateEvidence(context.Background(), ac); err != nil {
		t.Fatalf("confirmed never-started cancellation rejected: %v", err)
	}
}

func TestRound49TimeBudgetBoundaries(t *testing.T) {
	now := time.Date(2026, 10, 22, 12, 0, 0, 0, time.UTC)
	before := now.Add(-time.Second)
	after := now.Add(time.Second)
	for _, tc := range []struct {
		name     string
		expires  time.Time
		deadline *time.Time
		reject   bool
	}{
		{"unbounded", time.Time{}, nil, false},
		{"future_goal", after, nil, false},
		{"future_deadline", time.Time{}, &after, false},
		{"expired_goal", before, nil, true},
		{"goal_at_boundary", now, nil, true},
		{"past_deadline", time.Time{}, &before, true},
		{"deadline_at_boundary", time.Time{}, &now, true},
		{"both_limits_active", after, &after, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := validateCoordinationTimeBudget(now, tc.expires, tc.deadline)
			if (err != nil) != tc.reject {
				t.Fatalf("reject=%v, err=%v", tc.reject, err)
			}
		})
	}
}

func TestRound49StartRejectsExpiredGoalBeforeWorker(t *testing.T) {
	now := time.Now().UTC()
	goals := decision.NewGoalRegistry()
	if err := goals.Register(decision.Goal{ID: "round49-expired", Revision: 1, Status: decision.GoalStatusActive, ExpiresAt: now.Add(-time.Minute)}); err != nil {
		t.Fatal(err)
	}
	worker := &fakeWorkerRunner{}
	c := NewMultiAgentCoordinator(newFakeTracker(), goals, nil, worker, nil, DefaultMultiAgentPolicy())
	_, err := c.Start(context.Background(), StartCoordinationRequest{
		ParentGoalID: "round49-expired", ParentGoalRevision: 1,
		WorkerRefs: []AgentWorkerRef{{WorkerID: "worker"}},
		Objectives: []AssignmentObjective{{WorkerIndex: 0, Objective: "cannot run expired work"}},
	})
	if err == nil || !strings.Contains(err.Error(), "expired") || len(worker.started) != 0 {
		t.Fatalf("expired goal started work: err=%v workers=%d", err, len(worker.started))
	}
}

func TestRound49StartRejectsExpiredDeadlineBeforeWorker(t *testing.T) {
	goals := decision.NewGoalRegistry()
	if err := goals.Register(decision.Goal{ID: "round49-deadline", Revision: 1, Status: decision.GoalStatusActive}); err != nil {
		t.Fatal(err)
	}
	worker := &fakeWorkerRunner{}
	c := NewMultiAgentCoordinator(newFakeTracker(), goals, nil, worker, nil, DefaultMultiAgentPolicy())
	past := time.Now().UTC().Add(-time.Minute)
	_, err := c.Start(context.Background(), StartCoordinationRequest{
		ParentGoalID: "round49-deadline", ParentGoalRevision: 1, DeadlineAt: &past,
		WorkerRefs: []AgentWorkerRef{{WorkerID: "worker"}},
		Objectives: []AssignmentObjective{{WorkerIndex: 0, Objective: "cannot run past deadline"}},
	})
	if err == nil || !strings.Contains(err.Error(), "deadline") || len(worker.started) != 0 {
		t.Fatalf("expired deadline started work: err=%v workers=%d", err, len(worker.started))
	}
}

func TestRound49RecoveredParentRejectsCharacterDrift(t *testing.T) {
	parent := &InteractionRecord{ID: "round49-parent", Scope: InteractionScope{SpaceID: "s", ConversationID: "c", CharacterID: "other-role"}}
	ref := &MultiAgentRecoveryRef{ParentGoalID: "goal", ParentGoalRevision: 1, ParentGoalSpaceID: "s", ParentGoalConversationID: "c", ParentGoalCharacterID: "role"}
	goal := decision.Goal{ID: "goal", Revision: 1, SpaceID: "s", ConversationID: "c", CharacterID: "role"}
	if err := validateRecoveredParent(parent, ref, goal); err == nil {
		t.Fatal("mismatched durable parent role was accepted")
	}
	parent.Scope.CharacterID = "role"
	if err := validateRecoveredParent(parent, ref, goal); err != nil {
		t.Fatalf("matching role rejected: %v", err)
	}
}
