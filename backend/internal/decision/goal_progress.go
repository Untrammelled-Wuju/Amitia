package decision

import (
	"fmt"
	"math"
)

type GoalProgressExpectationMode string

const (
	GoalProgressObserveOnly GoalProgressExpectationMode = "observe_only"
	GoalProgressAdvanceTo   GoalProgressExpectationMode = "advance_to"
	GoalProgressAchieve     GoalProgressExpectationMode = "achieve"
)

type GoalProgressExpectation struct {
	Goal           GoalRef                     `json:"goal"`
	Mode           GoalProgressExpectationMode `json:"mode"`
	TargetProgress float64                     `json:"targetProgress,omitempty"`
}

type GoalProgressDisposition string

const (
	GoalProgressNoChange        GoalProgressDisposition = "no_change"
	GoalProgressObserved        GoalProgressDisposition = "observed"
	GoalProgressActivated       GoalProgressDisposition = "activated"
	GoalProgressAdvanced        GoalProgressDisposition = "advanced"
	GoalProgressAchieved        GoalProgressDisposition = "achieved"
	GoalProgressTerminalIgnore  GoalProgressDisposition = "terminal_ignored"
	GoalProgressSuspendedIgnore GoalProgressDisposition = "suspended_ignored"
	GoalProgressWishIgnore      GoalProgressDisposition = "wish_ignored"
	GoalProgressExpiredIgnore   GoalProgressDisposition = "expired_ignored"
	GoalProgressStaleRevision   GoalProgressDisposition = "stale_revision"
	GoalProgressScopeMismatch   GoalProgressDisposition = "scope_mismatch"
	GoalProgressMissing         GoalProgressDisposition = "missing_goal"
	GoalProgressStateInvalid    GoalProgressDisposition = "state_invalid"
)

type GoalProgressUpdate struct {
	GoalRef          GoalRef
	ObservationID    string
	ExpectedRevision int64
	Disposition      GoalProgressDisposition
	NextStatus       GoalStatus
	CurrentProgress  float64
	NextProgress     float64
	Apply            bool
	ReasonCodes      []string
}

type GoalProgressResult struct {
	GoalID         string
	ObservationID  string
	BeforeRevision int64
	AfterRevision  int64
	BeforeStatus   GoalStatus
	AfterStatus    GoalStatus
	BeforeProgress float64
	AfterProgress  float64
	Disposition    GoalProgressDisposition
	Changed        bool
	ReasonCodes    []string
}

type GoalProgressBatchResult struct {
	ObservationID string
	Results       []GoalProgressResult
	Applied       bool
}

type GoalProgressEvalErrorCode string

const (
	ErrGoalProgressPlanMismatch         GoalProgressEvalErrorCode = "GOAL_PROGRESS_PLAN_MISMATCH"
	ErrGoalProgressInteractionMismatch  GoalProgressEvalErrorCode = "GOAL_PROGRESS_INTERACTION_MISMATCH"
	ErrGoalProgressConversationMismatch GoalProgressEvalErrorCode = "GOAL_PROGRESS_CONVERSATION_MISMATCH"
	ErrGoalProgressUserMismatch         GoalProgressEvalErrorCode = "GOAL_PROGRESS_USER_MISMATCH"
	ErrGoalProgressCharacterMismatch    GoalProgressEvalErrorCode = "GOAL_PROGRESS_CHARACTER_MISMATCH"
	ErrGoalProgressStateInvalid         GoalProgressEvalErrorCode = "GOAL_PROGRESS_STATE_INVALID"
	ErrGoalProgressExpectationInvalid   GoalProgressEvalErrorCode = "GOAL_PROGRESS_EXPECTATION_INVALID"
)

type GoalProgressEvalError struct {
	Code GoalProgressEvalErrorCode
	Err  error
}

func (e GoalProgressEvalError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %s", e.Code, e.Err.Error())
	}
	return string(e.Code)
}

func EvaluateGoalProgress(
	goal Goal,
	expectation GoalProgressExpectation,
	observation Observation,
) (GoalProgressUpdate, error) {
	update := GoalProgressUpdate{
		GoalRef:          expectation.Goal,
		ObservationID:    observation.ID,
		ExpectedRevision: expectation.Goal.Revision,
		CurrentProgress:  goal.Progress,
		NextProgress:     goal.Progress,
		NextStatus:       goal.Status,
	}

	if goal.ID != expectation.Goal.ID {
		update.Disposition = GoalProgressNoChange
		update.ReasonCodes = []string{"goal_id_mismatch"}
		return update, GoalProgressEvalError{Code: ErrGoalProgressPlanMismatch, Err: fmt.Errorf("goal id mismatch: expected=%s actual=%s", expectation.Goal.ID, goal.ID)}
	}

	if !validGoalProgressValue(goal.Progress) {
		update.Disposition = GoalProgressStateInvalid
		update.ReasonCodes = []string{"goal_progress_invalid"}
		return update, GoalProgressEvalError{Code: ErrGoalProgressStateInvalid, Err: fmt.Errorf("goal has invalid progress: %f", goal.Progress)}
	}

	if goal.Status != GoalStatusAchieved && goal.Status != GoalStatusAbandoned && goal.Progress == 1 {
		update.Disposition = GoalProgressStateInvalid
		update.ReasonCodes = []string{"progress_1_non_achieved"}
		return update, GoalProgressEvalError{Code: ErrGoalProgressStateInvalid, Err: fmt.Errorf("active goal has progress=1")}
	}

	if goal.Status == GoalStatusAchieved {
		update.Disposition = GoalProgressTerminalIgnore
		update.ReasonCodes = []string{"goal_terminal"}
		return update, nil
	}
	if goal.Status == GoalStatusAbandoned {
		update.Disposition = GoalProgressTerminalIgnore
		update.ReasonCodes = []string{"goal_terminal"}
		return update, nil
	}
	if goal.Status == GoalStatusSuspended {
		update.Disposition = GoalProgressSuspendedIgnore
		update.ReasonCodes = []string{"goal_suspended"}
		return update, nil
	}
	if goal.Status == GoalStatusWish {
		update.Disposition = GoalProgressWishIgnore
		update.ReasonCodes = []string{"goal_wish"}
		return update, nil
	}

	if !goal.ExpiresAt.IsZero() && !observation.ObservedAt.Before(goal.ExpiresAt) {
		update.Disposition = GoalProgressExpiredIgnore
		update.ReasonCodes = []string{"goal_expired"}
		return update, nil
	}

	if goal.Revision != expectation.Goal.Revision {
		update.Disposition = GoalProgressStaleRevision
		update.ReasonCodes = []string{"goal_revision_stale"}
		return update, nil
	}

	if !scopeMatches(goal, observation) {
		update.Disposition = GoalProgressScopeMismatch
		update.ReasonCodes = []string{"scope_mismatch"}
		return update, fmt.Errorf("scope mismatch between goal and observation")
	}

	if expectation.Mode == GoalProgressAdvanceTo {
		if expectation.TargetProgress <= 0 || expectation.TargetProgress >= 1 {
			update.Disposition = GoalProgressStateInvalid
			update.ReasonCodes = []string{"advance_target_invalid"}
			return update, GoalProgressEvalError{Code: ErrGoalProgressExpectationInvalid, Err: fmt.Errorf("advance_to target must be 0 < target < 1: %f", expectation.TargetProgress)}
		}
	}

	if expectation.Mode == GoalProgressAchieve && expectation.TargetProgress != 0 {
		update.Disposition = GoalProgressStateInvalid
		update.ReasonCodes = []string{"achieve_target_must_be_zero"}
		return update, GoalProgressEvalError{Code: ErrGoalProgressExpectationInvalid, Err: fmt.Errorf("achieve mode must have targetProgress=0")}
	}
	if expectation.Mode == GoalProgressObserveOnly && expectation.TargetProgress != 0 {
		update.Disposition = GoalProgressStateInvalid
		update.ReasonCodes = []string{"observe_only_target_must_be_zero"}
		return update, GoalProgressEvalError{Code: ErrGoalProgressExpectationInvalid, Err: fmt.Errorf("observe_only mode must have targetProgress=0")}
	}

	switch observation.Kind {
	case ObservationKindNoAction:
		update.Disposition = GoalProgressObserved
		update.ReasonCodes = []string{"no_action"}
		return update, nil
	}

	switch observation.Outcome {
	case ObservationOutcomeSucceeded:
		switch expectation.Mode {
		case GoalProgressObserveOnly:
			applySucceededObserveOnly(&update, goal)
		case GoalProgressAdvanceTo:
			applySucceededAdvanceTo(&update, goal, expectation)
		case GoalProgressAchieve:
			applySucceededAchieve(&update, goal)
		}
	case ObservationOutcomeFailed, ObservationOutcomeCancelled, ObservationOutcomeTimedOut:
		applyToolFailureActive(&update, goal)
	case ObservationOutcomeSkipped:
		update.Disposition = GoalProgressObserved
		update.ReasonCodes = []string{"observation_skipped"}
	case ObservationOutcomeNotMaterialized, ObservationOutcomeNotDispatched:
		update.Disposition = GoalProgressObserved
		update.ReasonCodes = []string{"no_invocation"}
	default:
		update.Disposition = GoalProgressObserved
		update.ReasonCodes = []string{"unmatched_outcome"}
	}

	return update, nil
}

func applySucceededObserveOnly(update *GoalProgressUpdate, goal Goal) {
	if goal.Status == GoalStatusPending {
		update.NextStatus = GoalStatusActive
		update.Disposition = GoalProgressActivated
		update.ReasonCodes = []string{"tool_succeeded", "observe_only", "pending_to_active"}
		update.Apply = true
		return
	}
	update.Disposition = GoalProgressObserved
	update.ReasonCodes = []string{"tool_succeeded", "observe_only"}
	update.Apply = true
}

func applySucceededAdvanceTo(update *GoalProgressUpdate, goal Goal, expectation GoalProgressExpectation) {
	if goal.Status == GoalStatusPending {
		update.NextStatus = GoalStatusActive
		update.Disposition = GoalProgressActivated
		update.ReasonCodes = append(update.ReasonCodes, "pending_to_active")
	} else {
		update.Disposition = GoalProgressAdvanced
		update.ReasonCodes = append(update.ReasonCodes, "tool_succeeded", "advance_to")
	}
	if expectation.TargetProgress > goal.Progress {
		update.NextProgress = expectation.TargetProgress
		update.ReasonCodes = append(update.ReasonCodes, "progress_target_reached")
	} else {
		update.NextProgress = goal.Progress
		update.ReasonCodes = append(update.ReasonCodes, "progress_no_regression")
	}
	update.Apply = true
}

func applySucceededAchieve(update *GoalProgressUpdate, goal Goal) {
	update.NextStatus = GoalStatusAchieved
	update.NextProgress = 1
	if goal.Status == GoalStatusPending {
		update.Disposition = GoalProgressActivated
		update.ReasonCodes = []string{"tool_succeeded", "achieve", "pending_to_achieved"}
	} else {
		update.Disposition = GoalProgressAchieved
		update.ReasonCodes = []string{"tool_succeeded", "achieve", "completion_contract_satisfied"}
	}
	update.Apply = true
}

func applyToolFailureActive(update *GoalProgressUpdate, goal Goal) {
	if goal.Status == GoalStatusPending {
		update.NextStatus = GoalStatusActive
		update.Disposition = GoalProgressActivated
		update.ReasonCodes = []string{"tool_failed", "pending_to_active"}
		update.Apply = true
		return
	}
	update.Disposition = GoalProgressObserved
	update.ReasonCodes = []string{"tool_failed", "progress_unchanged"}
	update.Apply = true
}

func scopeMatches(goal Goal, observation Observation) bool {
	if goal.UserID != "" && observation.UserID != "" && goal.UserID != observation.UserID {
		return false
	}
	if goal.CharacterID != "" && observation.CharacterID != "" && goal.CharacterID != observation.CharacterID {
		return false
	}
	if goal.ConversationID != "" && observation.ConversationID != "" && goal.ConversationID != observation.ConversationID {
		return false
	}
	return true
}

func validGoalProgressValue(v float64) bool {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return false
	}
	return v >= 0 && v <= 1
}

func ValidateGoalProgressExpectation(expectation GoalProgressExpectation, goalIDs []string) error {
	if expectation.Goal.ID == "" {
		return GoalProgressEvalError{Code: ErrGoalProgressExpectationInvalid, Err: fmt.Errorf("expectation goal.id is required")}
	}
	found := false
	for _, id := range goalIDs {
		if id == expectation.Goal.ID {
			found = true
			break
		}
	}
	if !found {
		return GoalProgressEvalError{Code: ErrGoalProgressPlanMismatch, Err: fmt.Errorf("expectation goal.id=%s not in plan goal bindings", expectation.Goal.ID)}
	}
	switch expectation.Mode {
	case GoalProgressObserveOnly:
		if expectation.TargetProgress != 0 {
			return GoalProgressEvalError{Code: ErrGoalProgressExpectationInvalid, Err: fmt.Errorf("observe_only must have targetProgress=0")}
		}
	case GoalProgressAdvanceTo:
		if expectation.TargetProgress <= 0 || expectation.TargetProgress >= 1 {
			return GoalProgressEvalError{Code: ErrGoalProgressExpectationInvalid, Err: fmt.Errorf("advance_to target must be 0 < target < 1")}
		}
	case GoalProgressAchieve:
		if expectation.TargetProgress != 0 {
			return GoalProgressEvalError{Code: ErrGoalProgressExpectationInvalid, Err: fmt.Errorf("achieve must have targetProgress=0")}
		}
	default:
		return GoalProgressEvalError{Code: ErrGoalProgressExpectationInvalid, Err: fmt.Errorf("unknown expectation mode: %s", expectation.Mode)}
	}
	return nil
}
