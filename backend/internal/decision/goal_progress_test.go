package decision

import (
	"math"
	"testing"
	"time"
)

func TestEvaluateObserveOnlyToolSuccessDoesNotAdvanceProgress(t *testing.T) {
	goal := Goal{ID: "g1", Status: GoalStatusActive, Progress: 0.2, Revision: 1, UserID: "u1", CharacterID: "c1", ConversationID: "conv1"}
	obs := Observation{ID: "o1", PlanID: "p1", ActionID: "a1", InteractionID: "i1", ObservedAt: time.Now(), Kind: ObservationKindToolResult, Outcome: ObservationOutcomeSucceeded, UserID: "u1", CharacterID: "c1", ConversationID: "conv1"}
	exp := GoalProgressExpectation{Goal: GoalRef{ID: "g1", Revision: 1}, Mode: GoalProgressObserveOnly}
	update, err := EvaluateGoalProgress(goal, exp, obs)
	if err != nil {
		t.Fatal(err)
	}
	if update.NextStatus != GoalStatusActive {
		t.Errorf("status: got %s, want active", update.NextStatus)
	}
	if update.NextProgress != 0.2 {
		t.Errorf("progress: got %f, want 0.2", update.NextProgress)
	}
	if update.Disposition != GoalProgressObserved {
		t.Errorf("disposition: got %s, want observed", update.Disposition)
	}
}

func TestEvaluatePendingFirstToolSuccessActivates(t *testing.T) {
	goal := Goal{ID: "g1", Status: GoalStatusPending, Progress: 0, Revision: 1, UserID: "u1"}
	obs := Observation{ID: "o1", PlanID: "p1", ActionID: "a1", InteractionID: "i1", ObservedAt: time.Now(), Kind: ObservationKindToolResult, Outcome: ObservationOutcomeSucceeded, UserID: "u1"}
	exp := GoalProgressExpectation{Goal: GoalRef{ID: "g1", Revision: 1}, Mode: GoalProgressObserveOnly}
	update, _ := EvaluateGoalProgress(goal, exp, obs)
	if update.NextStatus != GoalStatusActive {
		t.Errorf("status: got %s, want active", update.NextStatus)
	}
	if update.Disposition != GoalProgressActivated {
		t.Errorf("disposition: got %s, want activated", update.Disposition)
	}
}

func TestEvaluatePendingToolFailed(t *testing.T) {
	goal := Goal{ID: "g1", Status: GoalStatusPending, Progress: 0, Revision: 1, UserID: "u1"}
	obs := Observation{ID: "o1", ObservedAt: time.Now(), Kind: ObservationKindToolResult, Outcome: ObservationOutcomeFailed, UserID: "u1"}
	exp := GoalProgressExpectation{Goal: GoalRef{ID: "g1", Revision: 1}, Mode: GoalProgressObserveOnly}
	update, _ := EvaluateGoalProgress(goal, exp, obs)
	if update.NextStatus != GoalStatusActive {
		t.Errorf("got %s, want active", update.NextStatus)
	}
	if update.NextProgress != 0 {
		t.Errorf("progress: got %f, want 0", update.NextProgress)
	}
}

func TestEvaluateAdvanceTo(t *testing.T) {
	goal := Goal{ID: "g1", Status: GoalStatusActive, Progress: 0.2, Revision: 1, UserID: "u1"}
	obs := Observation{ID: "o1", ObservedAt: time.Now(), Kind: ObservationKindToolResult, Outcome: ObservationOutcomeSucceeded, UserID: "u1"}
	exp := GoalProgressExpectation{Goal: GoalRef{ID: "g1", Revision: 1}, Mode: GoalProgressAdvanceTo, TargetProgress: 0.6}
	update, _ := EvaluateGoalProgress(goal, exp, obs)
	if update.NextProgress != 0.6 {
		t.Errorf("progress: got %f, want 0.6", update.NextProgress)
	}
	if update.NextStatus != GoalStatusActive {
		t.Errorf("status: got %s, want active", update.NextStatus)
	}
	if update.Disposition != GoalProgressAdvanced {
		t.Errorf("disposition: got %s, want advanced", update.Disposition)
	}
}

func TestEvaluateAdvanceNoRegression(t *testing.T) {
	goal := Goal{ID: "g1", Status: GoalStatusActive, Progress: 0.8, Revision: 1, UserID: "u1"}
	obs := Observation{ID: "o1", ObservedAt: time.Now(), Kind: ObservationKindToolResult, Outcome: ObservationOutcomeSucceeded, UserID: "u1"}
	exp := GoalProgressExpectation{Goal: GoalRef{ID: "g1", Revision: 1}, Mode: GoalProgressAdvanceTo, TargetProgress: 0.6}
	update, _ := EvaluateGoalProgress(goal, exp, obs)
	if update.NextProgress != 0.8 {
		t.Errorf("progress: got %f, want 0.8", update.NextProgress)
	}
}

func TestEvaluateAchieve(t *testing.T) {
	goal := Goal{ID: "g1", Status: GoalStatusActive, Progress: 0.5, Revision: 1, UserID: "u1"}
	obs := Observation{ID: "o1", ObservedAt: time.Now(), Kind: ObservationKindToolResult, Outcome: ObservationOutcomeSucceeded, UserID: "u1"}
	exp := GoalProgressExpectation{Goal: GoalRef{ID: "g1", Revision: 1}, Mode: GoalProgressAchieve}
	update, _ := EvaluateGoalProgress(goal, exp, obs)
	if update.NextStatus != GoalStatusAchieved {
		t.Errorf("status: got %s, want achieved", update.NextStatus)
	}
	if update.NextProgress != 1 {
		t.Errorf("progress: got %f, want 1", update.NextProgress)
	}
	if update.Disposition != GoalProgressAchieved {
		t.Errorf("disposition: got %s, want achieved", update.Disposition)
	}
}

func TestEvaluateAchieveFailedNotComplete(t *testing.T) {
	goal := Goal{ID: "g1", Status: GoalStatusActive, Progress: 0.5, Revision: 1, UserID: "u1"}
	obs := Observation{ID: "o1", ObservedAt: time.Now(), Kind: ObservationKindToolResult, Outcome: ObservationOutcomeFailed, UserID: "u1"}
	exp := GoalProgressExpectation{Goal: GoalRef{ID: "g1", Revision: 1}, Mode: GoalProgressAchieve}
	update, _ := EvaluateGoalProgress(goal, exp, obs)
	if update.NextStatus == GoalStatusAchieved {
		t.Errorf("should not achieve on failure")
	}
}

func TestEvaluateAchieveCancelledNotComplete(t *testing.T) {
	goal := Goal{ID: "g1", Status: GoalStatusActive, Progress: 0.5, Revision: 1, UserID: "u1"}
	obs := Observation{ID: "o1", ObservedAt: time.Now(), Kind: ObservationKindToolResult, Outcome: ObservationOutcomeCancelled, UserID: "u1"}
	exp := GoalProgressExpectation{Goal: GoalRef{ID: "g1", Revision: 1}, Mode: GoalProgressAchieve}
	update, _ := EvaluateGoalProgress(goal, exp, obs)
	if update.NextStatus == GoalStatusAchieved {
		t.Error("should not achieve on cancelled")
	}
}

func TestEvaluateAchieveTimedOutNotComplete(t *testing.T) {
	goal := Goal{ID: "g1", Status: GoalStatusActive, Progress: 0.5, Revision: 1, UserID: "u1"}
	obs := Observation{ID: "o1", ObservedAt: time.Now(), Kind: ObservationKindToolResult, Outcome: ObservationOutcomeTimedOut, UserID: "u1"}
	exp := GoalProgressExpectation{Goal: GoalRef{ID: "g1", Revision: 1}, Mode: GoalProgressAchieve}
	update, _ := EvaluateGoalProgress(goal, exp, obs)
	if update.NextStatus == GoalStatusAchieved {
		t.Error("should not achieve on timed_out")
	}
}

func TestEvaluateSuspendedIgnored(t *testing.T) {
	goal := Goal{ID: "g1", Status: GoalStatusSuspended, Progress: 0.5, Revision: 2, UserID: "u1"}
	obs := Observation{ID: "o1", ObservedAt: time.Now(), Kind: ObservationKindToolResult, Outcome: ObservationOutcomeSucceeded, UserID: "u1"}
	exp := GoalProgressExpectation{Goal: GoalRef{ID: "g1", Revision: 2}, Mode: GoalProgressAchieve}
	update, _ := EvaluateGoalProgress(goal, exp, obs)
	if update.Disposition != GoalProgressSuspendedIgnore {
		t.Errorf("disposition: got %s, want suspended_ignored", update.Disposition)
	}
}

func TestEvaluateAchievedTerminalIgnored(t *testing.T) {
	goal := Goal{ID: "g1", Status: GoalStatusAchieved, Progress: 1, Revision: 5, UserID: "u1"}
	obs := Observation{ID: "o1", ObservedAt: time.Now(), Kind: ObservationKindToolResult, Outcome: ObservationOutcomeSucceeded, UserID: "u1"}
	exp := GoalProgressExpectation{Goal: GoalRef{ID: "g1", Revision: 5}, Mode: GoalProgressAchieve}
	update, err := EvaluateGoalProgress(goal, exp, obs)
	if err != nil {
		t.Fatal(err)
	}
	if update.Disposition != GoalProgressTerminalIgnore {
		t.Errorf("disposition: got %s, want terminal_ignored", update.Disposition)
	}
	if update.Apply {
		t.Error("should not apply on terminal goal")
	}
}

func TestEvaluateAbandonedTerminalIgnored(t *testing.T) {
	goal := Goal{ID: "g1", Status: GoalStatusAbandoned, Progress: 0.3, Revision: 3, UserID: "u1"}
	obs := Observation{ID: "o1", ObservedAt: time.Now(), Kind: ObservationKindToolResult, Outcome: ObservationOutcomeSucceeded, UserID: "u1"}
	exp := GoalProgressExpectation{Goal: GoalRef{ID: "g1", Revision: 3}, Mode: GoalProgressAchieve}
	update, err := EvaluateGoalProgress(goal, exp, obs)
	if err != nil {
		t.Fatal(err)
	}
	if update.Disposition != GoalProgressTerminalIgnore {
		t.Error("should be terminal_ignored")
	}
}

func TestEvaluateWishIgnored(t *testing.T) {
	goal := Goal{ID: "g1", Status: GoalStatusWish, Progress: 0.3, Revision: 3, UserID: "u1"}
	obs := Observation{ID: "o1", ObservedAt: time.Now(), Kind: ObservationKindToolResult, Outcome: ObservationOutcomeSucceeded, UserID: "u1"}
	exp := GoalProgressExpectation{Goal: GoalRef{ID: "g1", Revision: 3}, Mode: GoalProgressObserveOnly}
	update, _ := EvaluateGoalProgress(goal, exp, obs)
	if update.Disposition != GoalProgressWishIgnore {
		t.Errorf("disposition: got %s, want wish_ignored", update.Disposition)
	}
}

func TestEvaluateExpiredIgnored(t *testing.T) {
	goal := Goal{ID: "g1", Status: GoalStatusActive, Progress: 0.3, Revision: 3, UserID: "u1", ExpiresAt: time.Now().Add(-time.Hour)}
	obs := Observation{ID: "o1", ObservedAt: time.Now(), Kind: ObservationKindToolResult, Outcome: ObservationOutcomeSucceeded, UserID: "u1"}
	exp := GoalProgressExpectation{Goal: GoalRef{ID: "g1", Revision: 3}, Mode: GoalProgressAchieve}
	update, _ := EvaluateGoalProgress(goal, exp, obs)
	if update.Disposition != GoalProgressExpiredIgnore {
		t.Errorf("disposition: got %s, want expired_ignored", update.Disposition)
	}
}

func TestEvaluateStaleRevision(t *testing.T) {
	goal := Goal{ID: "g1", Status: GoalStatusActive, Progress: 0.3, Revision: 6, UserID: "u1"}
	obs := Observation{ID: "o1", ObservedAt: time.Now(), Kind: ObservationKindToolResult, Outcome: ObservationOutcomeSucceeded, UserID: "u1"}
	exp := GoalProgressExpectation{Goal: GoalRef{ID: "g1", Revision: 5}, Mode: GoalProgressAchieve}
	update, _ := EvaluateGoalProgress(goal, exp, obs)
	if update.Disposition != GoalProgressStaleRevision {
		t.Errorf("disposition: got %s, want stale_revision", update.Disposition)
	}
}

func TestEvaluateScopeMismatch(t *testing.T) {
	goal := Goal{ID: "g1", Status: GoalStatusActive, Progress: 0.3, Revision: 1, UserID: "u1", CharacterID: "c1"}
	obs := Observation{ID: "o1", ObservedAt: time.Now(), Kind: ObservationKindToolResult, Outcome: ObservationOutcomeSucceeded, UserID: "u1", CharacterID: "c2"}
	exp := GoalProgressExpectation{Goal: GoalRef{ID: "g1", Revision: 1}, Mode: GoalProgressAchieve}
	update, err := EvaluateGoalProgress(goal, exp, obs)
	if err == nil {
		t.Error("expected error on scope mismatch")
	}
	if update.Disposition != GoalProgressScopeMismatch {
		t.Errorf("disposition: got %s, want scope_mismatch", update.Disposition)
	}
}

func TestEvaluateNoActionNoProgress(t *testing.T) {
	goal := Goal{ID: "g1", Status: GoalStatusPending, Progress: 0.3, Revision: 1, UserID: "u1"}
	obs := Observation{ID: "o1", ObservedAt: time.Now(), Kind: ObservationKindNoAction, Outcome: ObservationOutcomeSkipped, UserID: "u1"}
	exp := GoalProgressExpectation{Goal: GoalRef{ID: "g1", Revision: 1}, Mode: GoalProgressObserveOnly}
	update, _ := EvaluateGoalProgress(goal, exp, obs)
	if update.Apply {
		t.Error("no_action should not apply")
	}
	if update.NextProgress != 0.3 {
		t.Errorf("progress unchanged: got %f", update.NextProgress)
	}
}

func TestEvaluateMaterializationFailureNoProgress(t *testing.T) {
	goal := Goal{ID: "g1", Status: GoalStatusActive, Progress: 0.3, Revision: 1, UserID: "u1"}
	obs := Observation{ID: "o1", ObservedAt: time.Now(), Kind: ObservationKindMaterializationFailure, Outcome: ObservationOutcomeNotMaterialized, UserID: "u1"}
	exp := GoalProgressExpectation{Goal: GoalRef{ID: "g1", Revision: 1}, Mode: GoalProgressAchieve}
	update, _ := EvaluateGoalProgress(goal, exp, obs)
	if update.NextProgress != 0.3 {
		t.Errorf("progress should stay same on materialization failure, got %f", update.NextProgress)
	}
}

func TestValidateExpectationObserveOnlyNonZeroTarget(t *testing.T) {
	err := ValidateGoalProgressExpectation(
		GoalProgressExpectation{Goal: GoalRef{ID: "g1"}, Mode: GoalProgressObserveOnly, TargetProgress: 0.5},
		[]string{"g1"},
	)
	if err == nil {
		t.Error("expected error for observe_only with non-zero target")
	}
}

func TestValidateExpectationAdvanceToTarget1(t *testing.T) {
	err := ValidateGoalProgressExpectation(
		GoalProgressExpectation{Goal: GoalRef{ID: "g1"}, Mode: GoalProgressAdvanceTo, TargetProgress: 1},
		[]string{"g1"},
	)
	if err == nil {
		t.Error("expected error for advance_to with target=1")
	}
}

func TestValidateExpectationAchieveTargetNonZero(t *testing.T) {
	err := ValidateGoalProgressExpectation(
		GoalProgressExpectation{Goal: GoalRef{ID: "g1"}, Mode: GoalProgressAchieve, TargetProgress: 0.7},
		[]string{"g1"},
	)
	if err == nil {
		t.Error("expected error for achieve with non-zero target")
	}
}

func TestEvaluateGoalInvalidProgressValue(t *testing.T) {
	goal := Goal{ID: "g1", Status: GoalStatusActive, Progress: math.NaN(), Revision: 1, UserID: "u1"}
	obs := Observation{ID: "o1", ObservedAt: time.Now(), Kind: ObservationKindToolResult, Outcome: ObservationOutcomeSucceeded, UserID: "u1"}
	exp := GoalProgressExpectation{Goal: GoalRef{ID: "g1", Revision: 1}, Mode: GoalProgressObserveOnly}
	_, err := EvaluateGoalProgress(goal, exp, obs)
	if err == nil {
		t.Error("expected error for NaN progress")
	}
}

func TestEvaluateGoalProgress1ActiveRejected(t *testing.T) {
	goal := Goal{ID: "g1", Status: GoalStatusActive, Progress: 1, Revision: 1, UserID: "u1"}
	obs := Observation{ID: "o1", ObservedAt: time.Now(), Kind: ObservationKindToolResult, Outcome: ObservationOutcomeSucceeded, UserID: "u1"}
	exp := GoalProgressExpectation{Goal: GoalRef{ID: "g1", Revision: 1}, Mode: GoalProgressObserveOnly}
	_, err := EvaluateGoalProgress(goal, exp, obs)
	if err == nil {
		t.Error("progress=1 with active status should be rejected")
	}
}
