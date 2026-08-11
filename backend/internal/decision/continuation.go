package decision

type ContinuationDisposition string

const (
	ContinuationStop     ContinuationDisposition = "stop"
	ContinuationWait     ContinuationDisposition = "wait"
	ContinuationContinue ContinuationDisposition = "continue"
	ContinuationReplan   ContinuationDisposition = "replan"
)

type ContinuationDecision struct {
	Disposition   ContinuationDisposition `json:"disposition"`
	PlanID        string                  `json:"planId,omitempty"`
	ObservationID string                  `json:"observationId,omitempty"`
	InteractionID string                  `json:"interactionId,omitempty"`
	Iteration     int                     `json:"iteration"`
	ReplanCount   int                     `json:"replanCount"`
	ReasonCodes   []string                `json:"reasonCodes,omitempty"`
}

type ContinuationPolicy struct {
	MaxDecisionIterations int
	MaxReplans            int
}

type ContinuationInput struct {
	Plan         *BehaviorPlan
	Observation  *Observation
	GoalProgress GoalProgressBatchResult
	Goals        []Goal
	Iteration    int
	ReplanCount  int
	Policy       ContinuationPolicy
}

type ReplanContext struct {
	PreviousPlanID      string
	PreviousCandidateID string
	ObservationID       string
	ObservationOutcome  ObservationOutcome
	ErrorCode           string
	Iteration           int
}

func DefaultContinuationPolicy() ContinuationPolicy {
	return ContinuationPolicy{
		MaxDecisionIterations: 3,
		MaxReplans:            2,
	}
}

func EvaluateContinuation(input ContinuationInput) (ContinuationDecision, error) {
	decision := ContinuationDecision{
		Disposition: ContinuationStop,
		ReasonCodes: []string{},
		Iteration:   input.Iteration,
		ReplanCount: input.ReplanCount,
	}

	if input.Policy.MaxDecisionIterations <= 0 {
		input.Policy = DefaultContinuationPolicy()
	}

	if input.Plan == nil {
		decision.Disposition = ContinuationStop
		decision.ReasonCodes = []string{"no_plan"}
		return decision, nil
	}

	decision.PlanID = input.Plan.ID
	decision.InteractionID = input.Plan.InteractionID

	if input.Observation != nil {
		decision.ObservationID = input.Observation.ID
	}

	if input.Plan.SafetyLevel == BehaviorSafetyLevelBlocked {
		decision.Disposition = ContinuationStop
		decision.ReasonCodes = []string{"safety_blocked"}
		return decision, nil
	}

	if input.Observation != nil && len(input.Observation.GoalRefs) > 0 {
		for _, result := range input.GoalProgress.Results {
			if result.Disposition == GoalProgressStaleRevision {
				decision.Disposition = ContinuationStop
				decision.ReasonCodes = []string{"stale_revision"}
				return decision, nil
			}
			if result.Disposition == GoalProgressScopeMismatch {
				decision.Disposition = ContinuationStop
				decision.ReasonCodes = []string{"scope_mismatch"}
				return decision, nil
			}
			if result.Disposition == GoalProgressMissing {
				decision.Disposition = ContinuationStop
				decision.ReasonCodes = []string{"missing_goal"}
				return decision, nil
			}
		}
	}

	if input.Observation != nil && input.Plan.Selected.ActionType == CandidateActionWait &&
		input.Observation.Kind == ObservationKindNoAction {
		decision.Disposition = ContinuationWait
		decision.ReasonCodes = []string{"wait_plan_no_action"}
		return decision, nil
	}

	if input.Observation != nil && input.Observation.Outcome == ObservationOutcomeAccepted &&
		input.Observation.Kind == ObservationKindTaskAccepted {
		decision.Disposition = ContinuationWait
		decision.ReasonCodes = []string{"background_task_running"}
		return decision, nil
	}

	if input.Observation != nil && len(input.Observation.GoalRefs) > 0 {
		allAchieved := true
		anySuspended := false
		anyWish := false
		anyActive := false

		for _, goalRef := range input.Observation.GoalRefs {
			goal := input.FindGoalByID(goalRef.ID)
			if goal == nil {
				decision.Disposition = ContinuationStop
				decision.ReasonCodes = []string{"missing_goal"}
				return decision, nil
			}

			if goal.Status == GoalStatusAchieved || goal.Status == GoalStatusAbandoned {
				continue
			}
			if goal.Status == GoalStatusSuspended {
				anySuspended = true
				allAchieved = false
				continue
			}
			if goal.Status == GoalStatusWish {
				anyWish = true
				allAchieved = false
				continue
			}
			anyActive = true
			allAchieved = false
		}

		if allAchieved && len(input.Observation.GoalRefs) > 0 {
			decision.Disposition = ContinuationContinue
			decision.ReasonCodes = []string{"all_goals_achieved"}
			return decision, nil
		}

		if anySuspended && !anyActive {
			decision.Disposition = ContinuationWait
			decision.ReasonCodes = []string{"goal_suspended"}
			return decision, nil
		}

		if anyWish && !anyActive {
			decision.Disposition = ContinuationWait
			decision.ReasonCodes = []string{"goal_wish"}
			return decision, nil
		}
	}

	if input.Plan.DoNotSend {
		if input.ReplanBudgetExhausted(input.Policy) {
			decision.Disposition = ContinuationStop
			decision.ReasonCodes = []string{"do_not_send", "budget_exhausted"}
			return decision, nil
		}
		if input.HasActiveGoal() && !input.ReplanBudgetExhausted(input.Policy) {
			decision.Disposition = ContinuationReplan
			decision.ReasonCodes = []string{"do_not_send", "active_goal"}
			return decision, nil
		}
		decision.Disposition = ContinuationStop
		decision.ReasonCodes = []string{"do_not_send"}
		return decision, nil
	}

	if input.Observation != nil {
		switch input.Observation.Outcome {
		case ObservationOutcomeCancelled:
			decision.Disposition = ContinuationStop
			decision.ReasonCodes = []string{"tool_cancelled"}
			return decision, nil
		}
	}

	if input.ReplanBudgetExhausted(input.Policy) {
		if input.Plan.DoNotSend {
			decision.Disposition = ContinuationStop
			decision.ReasonCodes = []string{"budget_exhausted", "do_not_send"}
			return decision, nil
		}
		decision.Disposition = ContinuationContinue
		decision.ReasonCodes = []string{"replan_budget_exhausted"}
		return decision, nil
	}

	if input.Observation != nil {
		switch input.Observation.Kind {
		case ObservationKindMaterializationFailure:
			if input.HasActiveGoal() {
				decision.Disposition = ContinuationReplan
				decision.ReasonCodes = []string{"materialization_failure"}
				return decision, nil
			}
			decision.Disposition = ContinuationContinue
			decision.ReasonCodes = []string{"materialization_failure", "no_active_goal"}
			return decision, nil

		case ObservationKindDispatchFailure:
			if input.HasActiveGoal() {
				decision.Disposition = ContinuationReplan
				decision.ReasonCodes = []string{"dispatch_failure"}
				return decision, nil
			}
			decision.Disposition = ContinuationContinue
			decision.ReasonCodes = []string{"dispatch_failure", "no_active_goal"}
			return decision, nil
		}
	}

	if input.Plan.Selected.ActionType == CandidateActionChat {
		decision.Disposition = ContinuationContinue
		decision.ReasonCodes = []string{"respond_plan"}
		return decision, nil
	}

	if input.Plan.Selected.ActionType == CandidateActionWait {
		decision.Disposition = ContinuationWait
		decision.ReasonCodes = []string{"wait_plan"}
		return decision, nil
	}

	if input.Observation != nil && input.Observation.Kind == ObservationKindToolResult {
		switch input.Observation.Outcome {
		case ObservationOutcomeSucceeded:
			if input.Plan.GoalProgress != nil && len(input.Plan.GoalProgress) > 0 {
				allGoalsCompleted := true
				for _, exp := range input.Plan.GoalProgress {
					goal := input.FindGoalByID(exp.Goal.ID)
					if goal == nil {
						continue
					}
					if goal.Status != GoalStatusAchieved && goal.Status != GoalStatusAbandoned {
						allGoalsCompleted = false
						break
					}
				}
				if allGoalsCompleted {
					decision.Disposition = ContinuationContinue
					decision.ReasonCodes = []string{"tool_success_all_goals_completed"}
					return decision, nil
				}
			}

			if input.HasActiveGoal() {
				decision.Disposition = ContinuationReplan
				decision.ReasonCodes = []string{"tool_success_goal_active"}
				return decision, nil
			}

			decision.Disposition = ContinuationContinue
			decision.ReasonCodes = []string{"tool_success"}
			return decision, nil

		case ObservationOutcomeFailed, ObservationOutcomeTimedOut:
			if input.HasActiveGoal() {
				decision.Disposition = ContinuationReplan
				decision.ReasonCodes = []string{"tool_failed_replan"}
				return decision, nil
			}
			decision.Disposition = ContinuationContinue
			decision.ReasonCodes = []string{"tool_failed_no_active_goal"}
			return decision, nil

		case ObservationOutcomeCancelled:
			decision.Disposition = ContinuationStop
			decision.ReasonCodes = []string{"tool_cancelled"}
			return decision, nil
		}
	}

	if input.HasActiveGoal() && input.Iteration < input.Policy.MaxDecisionIterations {
		decision.Disposition = ContinuationReplan
		decision.ReasonCodes = []string{"active_goal_replan"}
		return decision, nil
	}

	decision.Disposition = ContinuationContinue
	decision.ReasonCodes = []string{"default_continue"}
	return decision, nil
}

func (input ContinuationInput) FindGoalByID(id string) *Goal {
	for i := range input.Goals {
		if input.Goals[i].ID == id {
			return &input.Goals[i]
		}
	}
	return nil
}

func (input ContinuationInput) HasActiveGoal() bool {
	if input.Observation == nil || len(input.Observation.GoalRefs) == 0 {
		return false
	}
	for _, goalRef := range input.Observation.GoalRefs {
		goal := input.FindGoalByID(goalRef.ID)
		if goal == nil {
			continue
		}
		if goal.Status == GoalStatusActive || goal.Status == GoalStatusPending {
			return true
		}
	}
	return false
}

func (input ContinuationInput) GoalAllAchieved() bool {
	if input.Observation == nil || len(input.Observation.GoalRefs) == 0 {
		return true
	}
	for _, goalRef := range input.Observation.GoalRefs {
		goal := input.FindGoalByID(goalRef.ID)
		if goal == nil {
			return false
		}
		if goal.Status != GoalStatusAchieved && goal.Status != GoalStatusAbandoned {
			return false
		}
	}
	return true
}

func (input ContinuationInput) ReplanBudgetExhausted(policy ContinuationPolicy) bool {
	return input.ReplanCount >= policy.MaxReplans
}