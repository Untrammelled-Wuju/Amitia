package decision

import (
	"testing"
	"time"
)

func TestEvaluateContinuation_RespondPlan(t *testing.T) {
	plan := &BehaviorPlan{
		ID:             "plan-1",
		InteractionID:  "interaction-1",
		ConversationID: "conv-1",
		Selected: BehaviorCandidate{
			ID:         "candidate-1",
			ActionType: CandidateActionChat,
		},
	}
	observation := &Observation{
		ID:             "obs-1",
		PlanID:         "plan-1",
		InteractionID:  "interaction-1",
		ConversationID: "conv-1",
		Kind:           ObservationKindNoAction,
		Outcome:        ObservationOutcomeSkipped,
		ObservedAt:     time.Now(),
	}
	input := ContinuationInput{
		Plan:         plan,
		Observation:  observation,
		GoalProgress: GoalProgressBatchResult{},
		Goals:        []Goal{},
		Iteration:    1,
		ReplanCount:  0,
		Policy:       DefaultContinuationPolicy(),
	}
	result, err := EvaluateContinuation(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Disposition != ContinuationContinue {
		t.Errorf("expected CONTINUE, got %s", result.Disposition)
	}
}

func TestEvaluateContinuation_WaitPlan(t *testing.T) {
	plan := &BehaviorPlan{
		ID:             "plan-1",
		InteractionID:  "interaction-1",
		ConversationID: "conv-1",
		Selected: BehaviorCandidate{
			ID:         "candidate-1",
			ActionType: CandidateActionWait,
		},
	}
	observation := &Observation{
		ID:             "obs-1",
		PlanID:         "plan-1",
		InteractionID:  "interaction-1",
		ConversationID: "conv-1",
		Kind:           ObservationKindNoAction,
		Outcome:        ObservationOutcomeSkipped,
		ObservedAt:     time.Now(),
	}
	input := ContinuationInput{
		Plan:         plan,
		Observation:  observation,
		GoalProgress: GoalProgressBatchResult{},
		Goals:        []Goal{},
		Iteration:    1,
		ReplanCount:  0,
		Policy:       DefaultContinuationPolicy(),
	}
	result, err := EvaluateContinuation(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Disposition != ContinuationWait {
		t.Errorf("expected WAIT, got %s", result.Disposition)
	}
}

func TestEvaluateContinuation_SafetyBlocked(t *testing.T) {
	plan := &BehaviorPlan{
		ID:             "plan-1",
		InteractionID:  "interaction-1",
		ConversationID: "conv-1",
		SafetyLevel:    BehaviorSafetyLevelBlocked,
		Selected: BehaviorCandidate{
			ID:         "candidate-1",
			ActionType: CandidateActionToolCall,
		},
	}
	observation := &Observation{
		ID:             "obs-1",
		PlanID:         "plan-1",
		InteractionID:  "interaction-1",
		ConversationID: "conv-1",
		Kind:           ObservationKindToolResult,
		Outcome:        ObservationOutcomeSucceeded,
		ObservedAt:     time.Now(),
	}
	input := ContinuationInput{
		Plan:         plan,
		Observation:  observation,
		GoalProgress: GoalProgressBatchResult{},
		Goals:        []Goal{},
		Iteration:    1,
		ReplanCount:  0,
		Policy:       DefaultContinuationPolicy(),
	}
	result, err := EvaluateContinuation(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Disposition != ContinuationStop {
		t.Errorf("expected STOP, got %s", result.Disposition)
	}
}

func TestEvaluateContinuation_ToolSuccessGoalAchieved(t *testing.T) {
	goalID := "goal-1"
	plan := &BehaviorPlan{
		ID:             "plan-1",
		InteractionID:  "interaction-1",
		ConversationID: "conv-1",
		Selected: BehaviorCandidate{
			ID:         "candidate-1",
			ActionType: CandidateActionToolCall,
		},
		GoalRefs: []GoalRef{{ID: goalID, Revision: 1}},
		GoalProgress: []GoalProgressExpectation{
			{Goal: GoalRef{ID: goalID, Revision: 1}, Mode: GoalProgressAchieve},
		},
	}
	observation := &Observation{
		ID:             "obs-1",
		PlanID:         "plan-1",
		InteractionID:  "interaction-1",
		ConversationID: "conv-1",
		Kind:           ObservationKindToolResult,
		Outcome:        ObservationOutcomeSucceeded,
		GoalRefs:      []GoalRef{{ID: goalID, Revision: 1}},
		ObservedAt:    time.Now(),
	}
	goals := []Goal{{ID: goalID, Status: GoalStatusAchieved, Progress: 1, Revision: 1}}
	input := ContinuationInput{
		Plan:        plan,
		Observation: observation,
		GoalProgress: GoalProgressBatchResult{
			Results: []GoalProgressResult{{GoalID: goalID, Disposition: GoalProgressAchieved, Changed: true}},
		},
		Goals:       goals,
		Iteration:   1,
		ReplanCount: 0,
		Policy:      DefaultContinuationPolicy(),
	}
	result, err := EvaluateContinuation(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Disposition != ContinuationContinue {
		t.Errorf("expected CONTINUE, got %s (reasons: %v)", result.Disposition, result.ReasonCodes)
	}
}

func TestEvaluateContinuation_ToolSuccessGoalActive(t *testing.T) {
	goalID := "goal-1"
	plan := &BehaviorPlan{
		ID:             "plan-1",
		InteractionID:  "interaction-1",
		ConversationID: "conv-1",
		Selected: BehaviorCandidate{
			ID:         "candidate-1",
			ActionType: CandidateActionToolCall,
		},
		GoalRefs: []GoalRef{{ID: goalID, Revision: 1}},
	}
	observation := &Observation{
		ID:             "obs-1",
		PlanID:         "plan-1",
		InteractionID:  "interaction-1",
		ConversationID: "conv-1",
		Kind:           ObservationKindToolResult,
		Outcome:        ObservationOutcomeSucceeded,
		GoalRefs:      []GoalRef{{ID: goalID, Revision: 1}},
		ObservedAt:    time.Now(),
	}
	goals := []Goal{{ID: goalID, Status: GoalStatusActive, Progress: 0.5, Revision: 1}}
	input := ContinuationInput{
		Plan:        plan,
		Observation: observation,
		GoalProgress: GoalProgressBatchResult{
			Results: []GoalProgressResult{{GoalID: goalID, Disposition: GoalProgressAdvanced, Changed: true}},
		},
		Goals:       goals,
		Iteration:   1,
		ReplanCount: 0,
		Policy:      DefaultContinuationPolicy(),
	}
	result, err := EvaluateContinuation(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Disposition != ContinuationReplan {
		t.Errorf("expected REPLAN, got %s (reasons: %v)", result.Disposition, result.ReasonCodes)
	}
}

func TestEvaluateContinuation_ToolFailedGoalActive(t *testing.T) {
	goalID := "goal-1"
	plan := &BehaviorPlan{
		ID:             "plan-1",
		InteractionID:  "interaction-1",
		ConversationID: "conv-1",
		Selected: BehaviorCandidate{
			ID:         "candidate-1",
			ActionType: CandidateActionToolCall,
		},
		GoalRefs: []GoalRef{{ID: goalID, Revision: 1}},
	}
	observation := &Observation{
		ID:             "obs-1",
		PlanID:         "plan-1",
		InteractionID:  "interaction-1",
		ConversationID: "conv-1",
		Kind:           ObservationKindToolResult,
		Outcome:        ObservationOutcomeFailed,
		GoalRefs:      []GoalRef{{ID: goalID, Revision: 1}},
		ObservedAt:    time.Now(),
	}
	goals := []Goal{{ID: goalID, Status: GoalStatusActive, Progress: 0.3, Revision: 1}}
	input := ContinuationInput{
		Plan:        plan,
		Observation: observation,
		GoalProgress: GoalProgressBatchResult{
			Results: []GoalProgressResult{{GoalID: goalID, Disposition: GoalProgressObserved, Changed: false}},
		},
		Goals:       goals,
		Iteration:   1,
		ReplanCount: 0,
		Policy:      DefaultContinuationPolicy(),
	}
	result, err := EvaluateContinuation(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Disposition != ContinuationReplan {
		t.Errorf("expected REPLAN, got %s", result.Disposition)
	}
}

func TestEvaluateContinuation_ToolCancelled(t *testing.T) {
	plan := &BehaviorPlan{
		ID:             "plan-1",
		InteractionID:  "interaction-1",
		ConversationID: "conv-1",
		Selected: BehaviorCandidate{
			ID:         "candidate-1",
			ActionType: CandidateActionToolCall,
		},
		GoalRefs: []GoalRef{{ID: "goal-1", Revision: 1}},
	}
	observation := &Observation{
		ID:             "obs-1",
		PlanID:         "plan-1",
		InteractionID:  "interaction-1",
		ConversationID: "conv-1",
		Kind:           ObservationKindToolResult,
		Outcome:        ObservationOutcomeCancelled,
		GoalRefs:      []GoalRef{{ID: "goal-1", Revision: 1}},
		ObservedAt:     time.Now(),
	}
	goals := []Goal{{ID: "goal-1", Status: GoalStatusActive}}
	input := ContinuationInput{
		Plan:         plan,
		Observation:  observation,
		GoalProgress: GoalProgressBatchResult{},
		Goals:        goals,
		Iteration:    1,
		ReplanCount:  0,
		Policy:       DefaultContinuationPolicy(),
	}
	result, err := EvaluateContinuation(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Disposition != ContinuationStop {
		t.Errorf("expected STOP, got %s", result.Disposition)
	}
}

func TestEvaluateContinuation_BudgetExhaustion(t *testing.T) {
	goalID := "goal-1"
	plan := &BehaviorPlan{
		ID:             "plan-1",
		InteractionID:  "interaction-1",
		ConversationID: "conv-1",
		Selected: BehaviorCandidate{
			ID:         "candidate-1",
			ActionType: CandidateActionToolCall,
		},
		GoalRefs: []GoalRef{{ID: goalID, Revision: 1}},
	}
	observation := &Observation{
		ID:             "obs-1",
		PlanID:         "plan-1",
		InteractionID:  "interaction-1",
		ConversationID: "conv-1",
		Kind:           ObservationKindToolResult,
		Outcome:        ObservationOutcomeFailed,
		GoalRefs:      []GoalRef{{ID: goalID, Revision: 1}},
		ObservedAt:     time.Now(),
	}
	goals := []Goal{{ID: goalID, Status: GoalStatusActive}}
	policy := ContinuationPolicy{MaxDecisionIterations: 3, MaxReplans: 2}
	input := ContinuationInput{
		Plan:        plan,
		Observation: observation,
		GoalProgress: GoalProgressBatchResult{
			Results: []GoalProgressResult{{GoalID: goalID, Disposition: GoalProgressObserved}},
		},
		Goals:       goals,
		Iteration:   3,
		ReplanCount: 2,
		Policy:      policy,
	}
	result, err := EvaluateContinuation(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Disposition != ContinuationContinue {
		t.Errorf("expected CONTINUE (budget exhausted), got %s", result.Disposition)
	}
}

func TestEvaluateContinuation_StaleRevision(t *testing.T) {
	goalID := "goal-1"
	plan := &BehaviorPlan{
		ID:             "plan-1",
		InteractionID:  "interaction-1",
		ConversationID: "conv-1",
		Selected: BehaviorCandidate{
			ID:         "candidate-1",
			ActionType: CandidateActionToolCall,
		},
		GoalRefs: []GoalRef{{ID: goalID, Revision: 1}},
	}
	observation := &Observation{
		ID:             "obs-1",
		PlanID:         "plan-1",
		InteractionID:  "interaction-1",
		ConversationID: "conv-1",
		Kind:           ObservationKindToolResult,
		Outcome:        ObservationOutcomeSucceeded,
		GoalRefs:      []GoalRef{{ID: goalID, Revision: 1}},
		ObservedAt:     time.Now(),
	}
	input := ContinuationInput{
		Plan:        plan,
		Observation: observation,
		GoalProgress: GoalProgressBatchResult{
			Results: []GoalProgressResult{{GoalID: goalID, Disposition: GoalProgressStaleRevision}},
		},
		Goals:       []Goal{{ID: goalID, Status: GoalStatusActive, Revision: 2}},
		Iteration:   1,
		ReplanCount: 0,
		Policy:      DefaultContinuationPolicy(),
	}
	result, err := EvaluateContinuation(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Disposition != ContinuationStop {
		t.Errorf("expected STOP, got %s", result.Disposition)
	}
}

func TestEvaluateContinuation_SuspendedGoal(t *testing.T) {
	goalID := "goal-1"
	plan := &BehaviorPlan{
		ID:             "plan-1",
		InteractionID:  "interaction-1",
		ConversationID: "conv-1",
		Selected: BehaviorCandidate{
			ID:         "candidate-1",
			ActionType: CandidateActionToolCall,
		},
		GoalRefs: []GoalRef{{ID: goalID, Revision: 1}},
	}
	observation := &Observation{
		ID:             "obs-1",
		PlanID:         "plan-1",
		InteractionID:  "interaction-1",
		ConversationID: "conv-1",
		Kind:           ObservationKindNoAction,
		Outcome:        ObservationOutcomeSkipped,
		GoalRefs:      []GoalRef{{ID: goalID, Revision: 1}},
		ObservedAt:     time.Now(),
	}
	goals := []Goal{{ID: goalID, Status: GoalStatusSuspended}}
	input := ContinuationInput{
		Plan:         plan,
		Observation:  observation,
		GoalProgress: GoalProgressBatchResult{},
		Goals:        goals,
		Iteration:    1,
		ReplanCount:  0,
		Policy:       DefaultContinuationPolicy(),
	}
	result, err := EvaluateContinuation(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Disposition != ContinuationWait {
		t.Errorf("expected WAIT, got %s", result.Disposition)
	}
}

func TestEvaluateContinuation_MaterializationFailure(t *testing.T) {
	goalID := "goal-1"
	plan := &BehaviorPlan{
		ID:             "plan-1",
		InteractionID:  "interaction-1",
		ConversationID: "conv-1",
		Selected: BehaviorCandidate{
			ID:         "candidate-1",
			ActionType: CandidateActionToolCall,
		},
		GoalRefs: []GoalRef{{ID: goalID, Revision: 1}},
	}
	observation := &Observation{
		ID:             "obs-1",
		PlanID:         "plan-1",
		InteractionID:  "interaction-1",
		ConversationID: "conv-1",
		Kind:           ObservationKindMaterializationFailure,
		Outcome:        ObservationOutcomeNotMaterialized,
		GoalRefs:      []GoalRef{{ID: goalID, Revision: 1}},
		ObservedAt:     time.Now(),
	}
	goals := []Goal{{ID: goalID, Status: GoalStatusActive}}
	input := ContinuationInput{
		Plan:         plan,
		Observation:  observation,
		GoalProgress: GoalProgressBatchResult{},
		Goals:        goals,
		Iteration:    1,
		ReplanCount:  0,
		Policy:       DefaultContinuationPolicy(),
	}
	result, err := EvaluateContinuation(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Disposition != ContinuationReplan {
		t.Errorf("expected REPLAN, got %s", result.Disposition)
	}
}

func TestEvaluateContinuation_DispatchFailure(t *testing.T) {
	goalID := "goal-1"
	plan := &BehaviorPlan{
		ID:             "plan-1",
		InteractionID:  "interaction-1",
		ConversationID: "conv-1",
		Selected: BehaviorCandidate{
			ID:         "candidate-1",
			ActionType: CandidateActionToolCall,
		},
		GoalRefs: []GoalRef{{ID: goalID, Revision: 1}},
	}
	observation := &Observation{
		ID:             "obs-1",
		PlanID:         "plan-1",
		InteractionID:  "interaction-1",
		ConversationID: "conv-1",
		Kind:           ObservationKindDispatchFailure,
		Outcome:        ObservationOutcomeNotDispatched,
		GoalRefs:      []GoalRef{{ID: goalID, Revision: 1}},
		ObservedAt:     time.Now(),
	}
	goals := []Goal{{ID: goalID, Status: GoalStatusActive}}
	input := ContinuationInput{
		Plan:         plan,
		Observation:  observation,
		GoalProgress: GoalProgressBatchResult{},
		Goals:        goals,
		Iteration:    1,
		ReplanCount:  0,
		Policy:       DefaultContinuationPolicy(),
	}
	result, err := EvaluateContinuation(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Disposition != ContinuationReplan {
		t.Errorf("expected REPLAN, got %s", result.Disposition)
	}
}

func TestEvaluateContinuation_NilPlan(t *testing.T) {
	input := ContinuationInput{
		Plan:         nil,
		Observation:  nil,
		GoalProgress: GoalProgressBatchResult{},
		Goals:        []Goal{},
		Policy:       DefaultContinuationPolicy(),
	}
	result, err := EvaluateContinuation(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Disposition != ContinuationStop {
		t.Errorf("expected STOP, got %s", result.Disposition)
	}
}

func TestEvaluateContinuation_IterationAndReplanCount(t *testing.T) {
	plan := &BehaviorPlan{
		ID:             "plan-1",
		InteractionID:  "interaction-1",
		ConversationID: "conv-1",
		Selected: BehaviorCandidate{
			ID:         "candidate-1",
			ActionType: CandidateActionChat,
		},
	}
	observation := &Observation{
		ID:             "obs-1",
		PlanID:         "plan-1",
		InteractionID:  "interaction-1",
		ConversationID: "conv-1",
		Kind:           ObservationKindNoAction,
		Outcome:        ObservationOutcomeSkipped,
		ObservedAt:     time.Now(),
	}
	input := ContinuationInput{
		Plan:         plan,
		Observation:  observation,
		GoalProgress: GoalProgressBatchResult{},
		Goals:        []Goal{},
		Iteration:    2,
		ReplanCount:  1,
		Policy:       DefaultContinuationPolicy(),
	}
	result, err := EvaluateContinuation(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Iteration != 2 {
		t.Errorf("expected iteration=2, got %d", result.Iteration)
	}
	if result.ReplanCount != 1 {
		t.Errorf("expected replanCount=1, got %d", result.ReplanCount)
	}
	if result.PlanID != "plan-1" {
		t.Errorf("expected planId=plan-1, got %s", result.PlanID)
	}
}