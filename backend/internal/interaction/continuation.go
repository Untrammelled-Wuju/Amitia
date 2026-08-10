package interaction

import (
	"context"
	"fmt"

	"github.com/u-ai/backend/internal/decision"
)

type ContinuationState struct {
	InteractionID string
	Iteration     int
	ReplanCount   int
}

type ContinuationService interface {
	Evaluate(
		ctx context.Context,
		plan *decision.BehaviorPlan,
		observation *decision.Observation,
		progress decision.GoalProgressBatchResult,
		state ContinuationState,
	) (decision.ContinuationDecision, error)
}

type continuationService struct {
	goals  *decision.GoalRegistry
	policy decision.ContinuationPolicy
}

func NewContinuationService(goals *decision.GoalRegistry) *continuationService {
	return &continuationService{
		goals:  goals,
		policy: decision.DefaultContinuationPolicy(),
	}
}

func NewContinuationServiceWithPolicy(goals *decision.GoalRegistry, policy decision.ContinuationPolicy) *continuationService {
	return &continuationService{
		goals:  goals,
		policy: policy,
	}
}

func (s *continuationService) Evaluate(
	ctx context.Context,
	plan *decision.BehaviorPlan,
	observation *decision.Observation,
	progress decision.GoalProgressBatchResult,
	state ContinuationState,
) (decision.ContinuationDecision, error) {
	if plan == nil {
		return decision.ContinuationDecision{
			Disposition: decision.ContinuationStop,
			ReasonCodes: []string{"no_plan"},
		}, nil
	}

	if observation != nil && len(observation.GoalRefs) > 0 {
		for _, ref := range observation.GoalRefs {
			if _, ok := s.goals.Get(ref.ID); !ok {
				return decision.ContinuationDecision{
					Disposition:   decision.ContinuationStop,
					PlanID:        plan.ID,
					ObservationID: observation.ID,
					InteractionID: state.InteractionID,
					Iteration:     state.Iteration,
					ReplanCount:   state.ReplanCount,
					ReasonCodes:   []string{"missing_goal_registry"},
				}, nil
			}
		}
	}

	goals := s.resolveGoals(plan, observation)

	input := decision.ContinuationInput{
		Plan:         plan,
		Observation:  observation,
		GoalProgress: progress,
		Goals:        goals,
		Iteration:    state.Iteration,
		ReplanCount:  state.ReplanCount,
		Policy:       s.policy,
	}

	result, err := decision.EvaluateContinuation(input)
	if err != nil {
		return decision.ContinuationDecision{
			Disposition: decision.ContinuationStop,
			PlanID:      plan.ID,
			ReasonCodes: []string{"evaluator_error", err.Error()},
		}, fmt.Errorf("continuation evaluation failed: %w", err)
	}

	result.InteractionID = state.InteractionID
	result.Iteration = state.Iteration
	result.ReplanCount = state.ReplanCount
	return result, nil
}

func (s *continuationService) resolveGoals(plan *decision.BehaviorPlan, observation *decision.Observation) []decision.Goal {
	var refs []decision.GoalRef
	if observation != nil && len(observation.GoalRefs) > 0 {
		refs = observation.GoalRefs
	} else if len(plan.GoalRefs) > 0 {
		refs = plan.GoalRefs
	}

	if len(refs) == 0 {
		return nil
	}
	goals := make([]decision.Goal, 0, len(refs))
	for _, ref := range refs {
		if goal, ok := s.goals.Get(ref.ID); ok {
			goals = append(goals, goal)
		}
	}
	return goals
}