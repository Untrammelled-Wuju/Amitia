package interaction

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/decision"
	"github.com/u-ai/backend/internal/mindruntime"
)

type fakeGoalReader struct {
	goals    map[string]decision.Goal
	scopeRes []decision.Goal
}

func (f *fakeGoalReader) GetGoal(ctx context.Context, goalID string) (decision.Goal, bool) {
	g, ok := f.goals[goalID]
	return g, ok
}

func (f *fakeGoalReader) ActiveForScope(ctx context.Context, userID, characterID, conversationID string) []decision.Goal {
	out := make([]decision.Goal, 0, len(f.scopeRes))
	for _, g := range f.scopeRes {
		if g.UserID != userID {
			continue
		}
		if characterID != "" && g.CharacterID != "" && g.CharacterID != characterID {
			continue
		}
		if conversationID != "" && g.ConversationID != "" && g.ConversationID != conversationID {
			continue
		}
		out = append(out, g)
	}
	return out
}

type fakeObsReader struct {
	byInteraction map[string][]decision.Observation
}

func (f *fakeObsReader) ListObservationsByInteraction(ctx context.Context, interactionID string) []decision.Observation {
	return f.byInteraction[interactionID]
}

type fakeTaskReader struct{}

func (fakeTaskReader) GetTaskRun(ctx context.Context, taskRunID string) (AgentTaskRef, bool) {
	return AgentTaskRef{}, false
}

func (fakeTaskReader) ListTaskRunsByInteraction(ctx context.Context, invocationID string) []AgentTaskRef {
	return nil
}

type fakeWorkflowReader struct{}

func (fakeWorkflowReader) GetWorkflowRun(ctx context.Context, executionID string) (AgentWorkflowRef, bool) {
	return AgentWorkflowRef{}, false
}

func (fakeWorkflowReader) ListWorkflowRunsByInteraction(ctx context.Context, invocationID string) []AgentWorkflowRef {
	return nil
}

type fakeInvocationReader struct{}

func (fakeInvocationReader) GetInvocation(ctx context.Context, invocationID string) (AgentInvocationRef, bool) {
	return AgentInvocationRef{}, false
}

func (fakeInvocationReader) ListInvocationsByInteraction(ctx context.Context, invocationID string) []AgentInvocationRef {
	return nil
}

func newTestProcessor(goals GoalReconciliationReader, obs AgentObservationReader) AgentReconciliationProcessor {
	return NewAgentReconciliationProcessor(goals, obs, fakeTaskReader{}, fakeWorkflowReader{}, fakeInvocationReader{})
}

func TestGoalActionChecker_ActiveGoalWithinSettleWindow_NotFlagged(t *testing.T) {
	now := time.Now().UTC()
	goals := &fakeGoalReader{
		scopeRes: []decision.Goal{
			{
				ID:             "goal-1",
				UserID:         "u1",
				CharacterID:    "c1",
				ConversationID: "conv1",
				Status:         decision.GoalStatusActive,
				Revision:       1,
				CreatedAt:      now.Add(-1 * time.Second),
				Trigger:        decision.GoalTrigger{InteractionID: "i1"},
			},
		},
	}
	obs := &fakeObsReader{byInteraction: map[string][]decision.Observation{
		"i1": {{ID: "o1", InteractionID: "i1", ActionID: "a1", GoalIDs: []string{"goal-1"}, Outcome: decision.ObservationOutcomeSucceeded, ObservedAt: now}},
	}}
	processor := newTestProcessor(goals, obs)
	checker := NewGoalActionChecker(processor, mindruntime.DefaultAgentFactSettleDelay())
	diffs, err := checker.CheckReconciliation(context.Background(), mindruntime.ReconciliationCheckRequest{
		ScanID: "s1",
		Target: mindruntime.ReconciliationAgentGoalAction,
		Scope:  &mindruntime.ReconciliationScope{UserID: "u1", CharacterID: "c1", ConversationID: "conv1", InteractionID: "i1"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(diffs) != 0 {
		t.Fatalf("expected 0 diffs, got %d: %+v", len(diffs), diffs)
	}
}

func TestGoalActionChecker_ActiveGoalPastSettle_Flagged(t *testing.T) {
	now := time.Now().UTC()
	goals := &fakeGoalReader{
		scopeRes: []decision.Goal{
			{
				ID:             "goal-1",
				UserID:         "u1",
				CharacterID:    "c1",
				ConversationID: "conv1",
				Status:         decision.GoalStatusActive,
				Revision:       1,
				CreatedAt:      now.Add(-10 * time.Second),
				Trigger:        decision.GoalTrigger{InteractionID: "i1"},
			},
		},
	}
	obs := &fakeObsReader{}
	processor := newTestProcessor(goals, obs)
	checker := NewGoalActionChecker(processor, 3*time.Second)
	diffs, err := checker.CheckReconciliation(context.Background(), mindruntime.ReconciliationCheckRequest{
		ScanID: "s1",
		Target: mindruntime.ReconciliationAgentGoalAction,
		Scope:  &mindruntime.ReconciliationScope{UserID: "u1", CharacterID: "c1", ConversationID: "conv1", InteractionID: "i1"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(diffs) != 1 {
		t.Fatalf("expected 1 diff, got %d", len(diffs))
	}
	if diffs[0].AutoRepairable {
		t.Fatalf("agent diffs must default AutoRepairable=false, got true")
	}
	if diffs[0].SourceKey != "goal-1" {
		t.Fatalf("expected source key goal-1, got %s", diffs[0].SourceKey)
	}
}

func TestGoalActionChecker_DifferentCharacter_ScopeIsolated(t *testing.T) {
	now := time.Now().UTC()
	goals := &fakeGoalReader{
		scopeRes: []decision.Goal{
			{
				ID:             "goal-1",
				UserID:         "u1",
				CharacterID:    "OTHER",
				ConversationID: "conv1",
				Status:         decision.GoalStatusActive,
				Trigger:        decision.GoalTrigger{InteractionID: "i1"},
				CreatedAt:      now.Add(-10 * time.Second),
			},
		},
	}
	obs := &fakeObsReader{}
	processor := newTestProcessor(goals, obs)
	checker := NewGoalActionChecker(processor, 3*time.Second)
	diffs, err := checker.CheckReconciliation(context.Background(), mindruntime.ReconciliationCheckRequest{
		ScanID: "s1",
		Target: mindruntime.ReconciliationAgentGoalAction,
		Scope:  &mindruntime.ReconciliationScope{UserID: "u1", CharacterID: "c1", ConversationID: "conv1", InteractionID: "i1"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(diffs) != 0 {
		t.Fatalf("expected cross-character isolation (0 diffs), got %d", len(diffs))
	}
}

func TestAgentSnapshotEntities_StableHash(t *testing.T) {
	snap := &AgentReconciliationSnapshot{
		UserID:         "u1",
		CharacterID:    "c1",
		ConversationID: "conv1",
		InteractionID:  "i1",
		Goals: []decision.Goal{
			{ID: "g1", UserID: "u1", Status: decision.GoalStatusActive, Revision: 2, Priority: decision.GoalPriorityNormal},
		},
	}
	entities := AgentSnapshotEntities(snap)
	if len(entities) != 1 {
		t.Fatalf("expected 1 entity, got %d", len(entities))
	}
	e := entities[0]
	if e.Store != "agent_goal" {
		t.Fatalf("expected store agent_goal, got %s", e.Store)
	}
	if e.References["userId"] != "u1" {
		t.Fatalf("expected userId reference u1, got %s", e.References["userId"])
	}
	if !strings.HasPrefix(e.Hash, "sha256:") {
		t.Fatalf("expected sha256: prefix on hash, got %s", e.Hash)
	}
	snap2 := &AgentReconciliationSnapshot{
		UserID: "u1",
		Goals:  []decision.Goal{{ID: "g1", UserID: "u1", Status: decision.GoalStatusActive, Revision: 2, Priority: decision.GoalPriorityNormal}},
	}
	if AgentSnapshotEntities(snap2)[0].Hash != e.Hash {
		t.Fatalf("hash must be deterministic across snapshot instances")
	}
}

func TestAgentSnapshotEntities_AllDiffsDefaultNonRepairable(t *testing.T) {
	now := time.Now().UTC()
	goals := &fakeGoalReader{
		scopeRes: []decision.Goal{
			{ID: "g1", Status: decision.GoalStatusActive, Revision: 1, CreatedAt: now.Add(-10 * time.Second), Trigger: decision.GoalTrigger{InteractionID: "i1"}},
		},
	}
	obs := &fakeObsReader{byInteraction: map[string][]decision.Observation{
		"i1": {{ID: "o1", InteractionID: "i1", ActionID: "a1", Outcome: "", ObservedAt: now}},
	}}
	processor := newTestProcessor(goals, obs)
	settleDelay := 3 * time.Second

	goalChecker := NewGoalActionChecker(processor, settleDelay)
	actionChecker := NewActionObservationChecker(processor, settleDelay)

	req := mindruntime.ReconciliationCheckRequest{
		ScanID: "s1",
		Target: mindruntime.ReconciliationAgentGoalAction,
		Scope:  &mindruntime.ReconciliationScope{UserID: "u1", InteractionID: "i1"},
	}
	if d, err := goalChecker.CheckReconciliation(context.Background(), req); err != nil {
		t.Fatalf("goal checker err: %v", err)
	} else if len(d) > 0 && d[0].AutoRepairable {
		t.Fatalf("goal diff must default AutoRepairable=false")
	}
	req.Target = mindruntime.ReconciliationAgentActionObservation
	if d, err := actionChecker.CheckReconciliation(context.Background(), req); err != nil {
		t.Fatalf("action checker err: %v", err)
	} else if len(d) > 0 && d[0].AutoRepairable {
		t.Fatalf("action diff must default AutoRepairable=false")
	}
}
