package interaction

import (
	"context"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/decision"
)

func TestGoalProgressServiceNilPlanNoError(t *testing.T) {
	reg := decision.NewGoalRegistry()
	reg.Register(decision.Goal{ID: "g1", Status: decision.GoalStatusActive, Progress: 0.2, Revision: 1})
	svc := NewGoalProgressService(reg)
	obs := &decision.Observation{ID: "o1", PlanID: "p1", InteractionID: "i1"}
	result, err := svc.ApplyObservation(context.Background(), nil, obs, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if result.Applied {
		t.Error("should not apply with nil plan")
	}
}

func TestGoalProgressServicePlanMismatch(t *testing.T) {
	reg := decision.NewGoalRegistry()
	svc := NewGoalProgressService(reg)
	plan := &decision.BehaviorPlan{ID: "p1", InteractionID: "i1", ConversationID: "conv1"}
	obs := &decision.Observation{ID: "o1", PlanID: "p2", InteractionID: "i1"}
	_, err := svc.ApplyObservation(context.Background(), plan, obs, time.Now())
	if err == nil {
		t.Fatal("expected plan mismatch error")
	}
}

func TestGoalProgressServiceInteractionMismatch(t *testing.T) {
	reg := decision.NewGoalRegistry()
	svc := NewGoalProgressService(reg)
	plan := &decision.BehaviorPlan{ID: "p1", InteractionID: "i1"}
	obs := &decision.Observation{ID: "o1", PlanID: "p1", InteractionID: "i2"}
	_, err := svc.ApplyObservation(context.Background(), plan, obs, time.Now())
	if err == nil {
		t.Fatal("expected interaction mismatch error")
	}
}

func TestGoalProgressServiceConversationMismatch(t *testing.T) {
	reg := decision.NewGoalRegistry()
	svc := NewGoalProgressService(reg)
	plan := &decision.BehaviorPlan{ID: "p1", InteractionID: "i1", ConversationID: "conv1"}
	obs := &decision.Observation{ID: "o1", PlanID: "p1", InteractionID: "i1", ConversationID: "conv2"}
	_, err := svc.ApplyObservation(context.Background(), plan, obs, time.Now())
	if err == nil {
		t.Fatal("expected conversation mismatch error")
	}
}

func TestGoalProgressServiceNoGoalRefsNoApply(t *testing.T) {
	reg := decision.NewGoalRegistry()
	svc := NewGoalProgressService(reg)
	plan := &decision.BehaviorPlan{ID: "p1", InteractionID: "i1"}
	obs := &decision.Observation{ID: "o1", PlanID: "p1", InteractionID: "i1"}
	result, err := svc.ApplyObservation(context.Background(), plan, obs, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if result.Applied {
		t.Error("should not apply when no goal refs")
	}
}

func TestGoalProgressServiceObserveOnlyDoesNotAdvanceProgress(t *testing.T) {
	reg := decision.NewGoalRegistry()
	reg.Register(decision.Goal{ID: "g1", Status: decision.GoalStatusActive, Progress: 0.2, Revision: 1, UserID: "u1", CharacterID: "c1", ConversationID: "conv1"})
	svc := NewGoalProgressService(reg)

	plan := &decision.BehaviorPlan{
		ID:             "p1",
		InteractionID:  "i1",
		GoalIDs:        []string{"g1"},
		GoalRefs:       []decision.GoalRef{{ID: "g1", Revision: 1}},
		GoalProgress:   []decision.GoalProgressExpectation{{Goal: decision.GoalRef{ID: "g1", Revision: 1}, Mode: decision.GoalProgressObserveOnly}},
		ConversationID: "conv1",
	}
	obs := &decision.Observation{
		ID:             "o1",
		PlanID:         "p1",
		ActionID:       "a1",
		InteractionID:  "i1",
		UserID:         "u1",
		CharacterID:    "c1",
		ConversationID: "conv1",
		GoalRefs:       []decision.GoalRef{{ID: "g1", Revision: 1}},
		Kind:           decision.ObservationKindToolResult,
		Outcome:        decision.ObservationOutcomeSucceeded,
		ObservedAt:     time.Now(),
	}

	result, err := svc.ApplyObservation(context.Background(), plan, obs, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if !result.Applied {
		t.Fatal("should be applied")
	}

	g, _ := reg.Get("g1")
	if g.Status != decision.GoalStatusActive {
		t.Errorf("status: got %s, want active", g.Status)
	}
	if g.Progress != 0.2 {
		t.Errorf("progress: got %f, want 0.2", g.Progress)
	}
	if g.Revision != 2 {
		t.Errorf("revision: got %d, want 2", g.Revision)
	}
	if g.LastObservationID != "o1" {
		t.Errorf("LastObservationID: got %s, want o1", g.LastObservationID)
	}
}

func TestGoalProgressServiceAdvanceTo05(t *testing.T) {
	reg := decision.NewGoalRegistry()
	reg.Register(decision.Goal{ID: "g1", Status: decision.GoalStatusActive, Progress: 0.2, Revision: 1, UserID: "u1"})
	svc := NewGoalProgressService(reg)

	plan := &decision.BehaviorPlan{
		ID:           "p1",
		GoalRefs:     []decision.GoalRef{{ID: "g1", Revision: 1}},
		GoalProgress: []decision.GoalProgressExpectation{{Goal: decision.GoalRef{ID: "g1", Revision: 1}, Mode: decision.GoalProgressAdvanceTo, TargetProgress: 0.5}},
	}
	obs := &decision.Observation{
		ID:         "o1",
		PlanID:     "p1",
		ActionID:   "a1",
		GoalRefs:   []decision.GoalRef{{ID: "g1", Revision: 1}},
		Kind:       decision.ObservationKindToolResult,
		Outcome:    decision.ObservationOutcomeSucceeded,
		ObservedAt: time.Now(),
		UserID:     "u1",
	}

	result, err := svc.ApplyObservation(context.Background(), plan, obs, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if !result.Applied {
		t.Fatal("not applied")
	}

	g, _ := reg.Get("g1")
	if g.Progress != 0.5 {
		t.Errorf("progress: got %f, want 0.5", g.Progress)
	}
	if g.Status != decision.GoalStatusActive {
		t.Errorf("status: got %s, want active", g.Status)
	}
}

func TestGoalProgressServiceAchieveProgress1(t *testing.T) {
	reg := decision.NewGoalRegistry()
	reg.Register(decision.Goal{ID: "g1", Status: decision.GoalStatusActive, Progress: 0.5, Revision: 1, UserID: "u1"})
	svc := NewGoalProgressService(reg)

	plan := &decision.BehaviorPlan{
		ID:           "p1",
		GoalRefs:     []decision.GoalRef{{ID: "g1", Revision: 1}},
		GoalProgress: []decision.GoalProgressExpectation{{Goal: decision.GoalRef{ID: "g1", Revision: 1}, Mode: decision.GoalProgressAchieve}},
	}
	obs := &decision.Observation{
		ID:         "o1",
		PlanID:     "p1",
		ActionID:   "a1",
		GoalRefs:   []decision.GoalRef{{ID: "g1", Revision: 1}},
		Kind:       decision.ObservationKindToolResult,
		Outcome:    decision.ObservationOutcomeSucceeded,
		ObservedAt: time.Now(),
		UserID:     "u1",
	}

	result, err := svc.ApplyObservation(context.Background(), plan, obs, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if !result.Applied {
		t.Fatal("not applied")
	}

	g, _ := reg.Get("g1")
	if g.Status != decision.GoalStatusAchieved {
		t.Errorf("status: got %s, want achieved", g.Status)
	}
	if g.Progress != 1 {
		t.Errorf("progress: got %f, want 1", g.Progress)
	}
}

func TestGoalProgressServicePendingToolSuccessActivatesAndRevision(t *testing.T) {
	reg := decision.NewGoalRegistry()
	reg.Register(decision.Goal{ID: "g1", Status: decision.GoalStatusPending, Progress: 0, Revision: 1, UserID: "u1"})
	svc := NewGoalProgressService(reg)

	plan := &decision.BehaviorPlan{
		ID:           "p1",
		GoalRefs:     []decision.GoalRef{{ID: "g1", Revision: 1}},
		GoalProgress: []decision.GoalProgressExpectation{{Goal: decision.GoalRef{ID: "g1", Revision: 1}, Mode: decision.GoalProgressObserveOnly}},
	}
	obs := &decision.Observation{
		ID:         "o1",
		PlanID:     "p1",
		ActionID:   "a1",
		GoalRefs:   []decision.GoalRef{{ID: "g1", Revision: 1}},
		Kind:       decision.ObservationKindToolResult,
		Outcome:    decision.ObservationOutcomeSucceeded,
		ObservedAt: time.Now(),
		UserID:     "u1",
	}

	_, err := svc.ApplyObservation(context.Background(), plan, obs, time.Now())
	if err != nil {
		t.Fatal(err)
	}

	g, _ := reg.Get("g1")
	if g.Status != decision.GoalStatusActive {
		t.Errorf("status: got %s, want active", g.Status)
	}
	if g.Revision != 2 {
		t.Errorf("revision: got %d, want 2", g.Revision)
	}
}

func TestGoalProgressServiceMissingGoalInBatch(t *testing.T) {
	reg := decision.NewGoalRegistry()
	reg.Register(decision.Goal{ID: "g1", Status: decision.GoalStatusActive, Progress: 0.2, Revision: 1, UserID: "u1"})
	svc := NewGoalProgressService(reg)

	plan := &decision.BehaviorPlan{
		ID:           "p1",
		GoalRefs:     []decision.GoalRef{{ID: "g1", Revision: 1}, {ID: "missing", Revision: 1}},
		GoalProgress: []decision.GoalProgressExpectation{{Goal: decision.GoalRef{ID: "g1", Revision: 1}, Mode: decision.GoalProgressAdvanceTo, TargetProgress: 0.5}},
	}
	obs := &decision.Observation{
		ID:         "o1",
		PlanID:     "p1",
		ActionID:   "a1",
		GoalRefs:   []decision.GoalRef{{ID: "g1", Revision: 1}, {ID: "missing", Revision: 1}},
		Kind:       decision.ObservationKindToolResult,
		Outcome:    decision.ObservationOutcomeSucceeded,
		ObservedAt: time.Now(),
		UserID:     "u1",
	}

	_, err := svc.ApplyObservation(context.Background(), plan, obs, time.Now())
	if err == nil {
		t.Fatal("missing goal in batch should return error and prevent entire batch commit")
	}

	g1, _ := reg.Get("g1")
	if g1.Progress != 0.2 {
		t.Errorf("batch atomicity: g1 should not change on missing goal: progress=%f", g1.Progress)
	}
	if g1.Revision != 1 {
		t.Errorf("g1 revision should stay 1: got %d", g1.Revision)
	}
}

func TestGoalProgressServiceStaleRevisionNoMutation(t *testing.T) {
	reg := decision.NewGoalRegistry()
	reg.Register(decision.Goal{ID: "g1", Status: decision.GoalStatusAchieved, Progress: 1, Revision: 5, UserID: "u1"})
	svc := NewGoalProgressService(reg)

	plan := &decision.BehaviorPlan{
		ID:           "p1",
		GoalRefs:     []decision.GoalRef{{ID: "g1", Revision: 3}},
		GoalProgress: []decision.GoalProgressExpectation{{Goal: decision.GoalRef{ID: "g1", Revision: 3}, Mode: decision.GoalProgressAchieve}},
	}
	obs := &decision.Observation{
		ID:         "o1",
		PlanID:     "p1",
		ActionID:   "a1",
		GoalRefs:   []decision.GoalRef{{ID: "g1", Revision: 3}},
		Kind:       decision.ObservationKindToolResult,
		Outcome:    decision.ObservationOutcomeSucceeded,
		ObservedAt: time.Now(),
		UserID:     "u1",
	}

	_, err := svc.ApplyObservation(context.Background(), plan, obs, time.Now())
	if err != nil {
		t.Fatal(err)
	}

	g, _ := reg.Get("g1")
	if g.Revision != 5 {
		t.Errorf("stale revision should not change goal: revision=%d", g.Revision)
	}
}

func TestGoalProgressServiceSuspendedIgnored(t *testing.T) {
	reg := decision.NewGoalRegistry()
	reg.Register(decision.Goal{ID: "g1", Status: decision.GoalStatusSuspended, Progress: 0.5, Revision: 3, UserID: "u1"})
	svc := NewGoalProgressService(reg)

	plan := &decision.BehaviorPlan{
		ID:           "p1",
		GoalRefs:     []decision.GoalRef{{ID: "g1", Revision: 3}},
		GoalProgress: []decision.GoalProgressExpectation{{Goal: decision.GoalRef{ID: "g1", Revision: 3}, Mode: decision.GoalProgressAchieve}},
	}
	obs := &decision.Observation{
		ID:         "o1",
		PlanID:     "p1",
		ActionID:   "a1",
		GoalRefs:   []decision.GoalRef{{ID: "g1", Revision: 3}},
		Kind:       decision.ObservationKindToolResult,
		Outcome:    decision.ObservationOutcomeSucceeded,
		ObservedAt: time.Now(),
		UserID:     "u1",
	}

	result, err := svc.ApplyObservation(context.Background(), plan, obs, time.Now())
	if err != nil {
		t.Fatal(err)
	}

	g, _ := reg.Get("g1")
	if g.Status != decision.GoalStatusSuspended {
		t.Errorf("status: got %s, want suspended", g.Status)
	}
	if g.Revision != 3 {
		t.Errorf("revision: got %d, want 3", g.Revision)
	}
	_ = result
}
