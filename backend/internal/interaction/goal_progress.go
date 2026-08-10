package interaction

import (
	"context"
	"fmt"
	"time"

	"github.com/u-ai/backend/internal/decision"
)

type GoalProgressService struct {
	goals *decision.GoalRegistry
}

func NewGoalProgressService(goals *decision.GoalRegistry) GoalProgressService {
	return GoalProgressService{goals: goals}
}

func (s GoalProgressService) ApplyObservation(
	ctx context.Context,
	plan *decision.BehaviorPlan,
	observation *decision.Observation,
	now time.Time,
) (decision.GoalProgressBatchResult, error) {
	result := decision.GoalProgressBatchResult{
		ObservationID: observation.ID,
		Results:       make([]decision.GoalProgressResult, 0),
		Applied:       false,
	}

	if plan == nil || observation == nil {
		return result, nil
	}
	if plan.ID != observation.PlanID {
		return result, decision.GoalProgressEvalError{Code: decision.ErrGoalProgressPlanMismatch, Err: fmt.Errorf("plan.id=%s observation.planId=%s", plan.ID, observation.PlanID)}
	}
	if plan.InteractionID != observation.InteractionID {
		return result, decision.GoalProgressEvalError{Code: decision.ErrGoalProgressInteractionMismatch, Err: fmt.Errorf("plan.interactionId=%s observation.interactionId=%s", plan.InteractionID, observation.InteractionID)}
	}
	if plan.ConversationID != "" && observation.ConversationID != "" && plan.ConversationID != observation.ConversationID {
		return result, decision.GoalProgressEvalError{Code: decision.ErrGoalProgressConversationMismatch, Err: fmt.Errorf("plan.conversationId=%s observation.conversationId=%s", plan.ConversationID, observation.ConversationID)}
	}

	if len(observation.GoalRefs) == 0 {
		return result, nil
	}

	expectations := buildExpectations(plan, observation)

	for _, ref := range observation.GoalRefs {
		if _, ok := s.goals.Get(ref.ID); !ok {
			return decision.GoalProgressBatchResult{ObservationID: observation.ID}, fmt.Errorf("missing goal in batch: %s", ref.ID)
		}
	}

	updates := make([]decision.GoalProgressUpdate, 0, len(expectations))
	for _, exp := range expectations {
		goal, _ := s.goals.Get(exp.Goal.ID)
		update, err := decision.EvaluateGoalProgress(goal, exp, *observation)
		if err == nil {
			updates = append(updates, update)
			continue
		}
		var gerr decision.GoalProgressEvalError
		if AsGoalProgressEvalError(err, &gerr) {
			update.ReasonCodes = append(update.ReasonCodes, string(gerr.Code))
			updates = append(updates, update)
			continue
		}
		return result, err
	}

	if !allNoApply(updates) {
		results, err := s.goals.ApplyProgressBatch(updates, now)
		if err != nil {
			return decision.GoalProgressBatchResult{ObservationID: observation.ID}, err
		}
		result.Results = results
		result.Applied = true
	}

	return result, nil
}

func buildExpectations(plan *decision.BehaviorPlan, observation *decision.Observation) []decision.GoalProgressExpectation {
	if len(observation.GoalRefs) == 0 {
		return nil
	}
	expectations := make([]decision.GoalProgressExpectation, 0, len(observation.GoalRefs))
	for _, ref := range observation.GoalRefs {
		mode := decision.GoalProgressObserveOnly
		var target float64
		for _, gp := range plan.GoalProgress {
			if gp.Goal.ID == ref.ID {
				mode = gp.Mode
				target = gp.TargetProgress
				break
			}
		}
		expectations = append(expectations, decision.GoalProgressExpectation{
			Goal:           ref,
			Mode:           mode,
			TargetProgress: target,
		})
	}
	return expectations
}

func allNoApply(updates []decision.GoalProgressUpdate) bool {
	for _, u := range updates {
		if u.Apply {
			return false
		}
	}
	return true
}

func AsGoalProgressEvalError(err error, target *decision.GoalProgressEvalError) bool {
	if err == nil {
		return false
	}
	if ge, ok := err.(decision.GoalProgressEvalError); ok {
		*target = ge
		return true
	}
	type wrapper interface{ Unwrap() error }
	if w, ok := err.(wrapper); ok {
		return AsGoalProgressEvalError(w.Unwrap(), target)
	}
	return false
}
