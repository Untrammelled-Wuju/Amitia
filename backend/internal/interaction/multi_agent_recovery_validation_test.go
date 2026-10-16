package interaction

import (
	"strings"
	"testing"

	"github.com/u-ai/backend/internal/decision"
)

func TestRecoveredParentMustMatchSpaceConversationAndGoalRevision(t *testing.T) {
	rec := &InteractionRecord{ID: "parent", Scope: InteractionScope{SpaceID: "space-a", ConversationID: "conversation-a"}}
	ref := &MultiAgentRecoveryRef{ParentGoalID: "goal", ParentGoalRevision: 4, ParentGoalSpaceID: "space-a", ParentGoalConversationID: "conversation-a", ParentGoalCharacterID: "character-a"}
	goal := decision.Goal{ID: "goal", Revision: 4, SpaceID: "space-a", ConversationID: "conversation-a", CharacterID: "character-a"}
	if err := validateRecoveredParent(rec, ref, goal); err != nil {
		t.Fatalf("valid matching scope refused: %v", err)
	}
	cases := []struct {
		name   string
		mutate func(*InteractionRecord, *MultiAgentRecoveryRef, *decision.Goal)
	}{
		{"wrong owner space", func(r *InteractionRecord, ref *MultiAgentRecoveryRef, g *decision.Goal) { g.SpaceID = "space-b" }},
		{"wrong parent conversation", func(r *InteractionRecord, ref *MultiAgentRecoveryRef, g *decision.Goal) {
			r.Scope.ConversationID = "conversation-b"
		}},
		{"wrong goal revision", func(r *InteractionRecord, ref *MultiAgentRecoveryRef, g *decision.Goal) { g.Revision++ }},
		{"wrong worker identity", func(r *InteractionRecord, ref *MultiAgentRecoveryRef, g *decision.Goal) {
			g.CharacterID = "character-b"
		}},
		{"removed persisted scope", func(r *InteractionRecord, ref *MultiAgentRecoveryRef, g *decision.Goal) { ref.ParentGoalSpaceID = "" }},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			nextRec := InteractionRecord{ID: rec.ID, Scope: rec.Scope}
			nextRef := *ref
			nextGoal := goal
			c.mutate(&nextRec, &nextRef, &nextGoal)
			if err := validateRecoveredParent(&nextRec, &nextRef, nextGoal); err == nil {
				t.Fatal("invalid recovered identity allowed")
			}
		})
	}
}

func TestRecoveredWorkerPlanRejectsDuplicatesCyclesAndOverLimit(t *testing.T) {
	base := &MultiAgentRecoveryRef{AssignmentRefs: []AssignmentRecoveryRef{
		{AssignmentID: "a", WorkerID: "worker-a", Objective: "verify", Status: "pending"},
		{AssignmentID: "b", WorkerID: "worker-b", Objective: "build", Status: "pending", Dependencies: []string{"a"}},
	}}
	if err := validateRecoveredWorkerPlan(base, 8, 4); err != nil {
		t.Fatalf("valid worker dependency graph rejected: %v", err)
	}
	cases := []struct {
		name     string
		expected string
		change   func(*MultiAgentRecoveryRef)
	}{
		{"duplicate assignment", "duplicate", func(ref *MultiAgentRecoveryRef) { ref.AssignmentRefs[1].AssignmentID = "a" }},
		{"missing predecessor", "unknown", func(ref *MultiAgentRecoveryRef) { ref.AssignmentRefs[1].Dependencies = []string{"missing"} }},
		{"cyclic dependencies", "cyclic", func(ref *MultiAgentRecoveryRef) { ref.AssignmentRefs[0].Dependencies = []string{"b"} }},
		{"duplicate edge", "duplicate", func(ref *MultiAgentRecoveryRef) { ref.AssignmentRefs[1].Dependencies = []string{"a", "a"} }},
		{"exceeds assignment budget", "bounds", func(ref *MultiAgentRecoveryRef) {
			ref.AssignmentRefs = append(ref.AssignmentRefs, AssignmentRecoveryRef{AssignmentID: "c", WorkerID: "worker-c", Objective: "review"})
		}},
		{"exceeds recursion budget", "bounds", func(ref *MultiAgentRecoveryRef) { ref.Depth = 9 }},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			copied := *base
			copied.AssignmentRefs = make([]AssignmentRecoveryRef, len(base.AssignmentRefs))
			for i, a := range base.AssignmentRefs {
				copied.AssignmentRefs[i] = a
				copied.AssignmentRefs[i].Dependencies = append([]string(nil), a.Dependencies...)
			}
			c.change(&copied)
			err := validateRecoveredWorkerPlan(&copied, 2, 4)
			if err == nil || !strings.Contains(err.Error(), c.expected) {
				t.Fatalf("invalid worker recovery plan accepted: %v", err)
			}
		})
	}
}
