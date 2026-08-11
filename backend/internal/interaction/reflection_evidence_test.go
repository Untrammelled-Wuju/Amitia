package interaction

import (
	"fmt"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/decision"
	"github.com/u-ai/backend/internal/mindruntime"
)

func TestObservationToVerifiedEventSkipsEmptyID(t *testing.T) {
	obs := decision.Observation{
		Kind:       decision.ObservationKindToolResult,
		Outcome:    decision.ObservationOutcomeSucceeded,
		ObservedAt: time.Now(),
	}
	evt, ok := ObservationToVerifiedEvent(obs, decision.GoalProgressBatchResult{}, decision.ContinuationDecision{})
	if ok {
		t.Fatal("expected false for empty observation ID")
	}
	if evt.ID != "" {
		t.Fatal("expected empty event ID")
	}
}

func TestObservationToVerifiedEventSkipsNoAction(t *testing.T) {
	obs := decision.Observation{
		ID:         "obs-1",
		Kind:       decision.ObservationKindNoAction,
		Outcome:    decision.ObservationOutcomeSucceeded,
		ObservedAt: time.Now(),
	}
	evt, ok := ObservationToVerifiedEvent(obs, decision.GoalProgressBatchResult{}, decision.ContinuationDecision{})
	if ok {
		t.Fatal("expected false for no_action observation")
	}
	if evt.ID != "" {
		t.Fatal("expected empty event ID")
	}
}

func TestObservationToVerifiedEventReturnsTrueForToolResult(t *testing.T) {
	now := time.Now().UTC()
	obs := decision.Observation{
		ID:         "obs-tool-1",
		Kind:       decision.ObservationKindToolResult,
		Outcome:    decision.ObservationOutcomeSucceeded,
		ToolID:     "create_schedule",
		ObservedAt: now,
	}
	progress := decision.GoalProgressBatchResult{
		Results: []decision.GoalProgressResult{
			{GoalID: "g1", Disposition: decision.GoalProgressAchieved},
		},
	}
	cont := decision.ContinuationDecision{Disposition: decision.ContinuationStop}
	evt, ok := ObservationToVerifiedEvent(obs, progress, cont)
	if !ok {
		t.Fatal("expected true for valid tool_result observation")
	}
	if evt.ID != "obs-tool-1" {
		t.Errorf("expected event ID obs-tool-1, got %s", evt.ID)
	}
	if evt.Importance != 0.8 {
		t.Errorf("expected importance 0.8 for goal achieved, got %f", evt.Importance)
	}
	if evt.Timestamp != now {
		t.Errorf("expected timestamp preserved, got %v", evt.Timestamp)
	}
}

func TestObservationToVerifiedEventFailureOutcome(t *testing.T) {
	obs := decision.Observation{
		ID:      "obs-fail-1",
		Kind:    decision.ObservationKindToolResult,
		Outcome: decision.ObservationOutcomeFailed,
	}
	evt, ok := ObservationToVerifiedEvent(obs, decision.GoalProgressBatchResult{}, decision.ContinuationDecision{})
	if !ok {
		t.Fatal("expected true")
	}
	if evt.Importance != 0.6 {
		t.Errorf("expected importance 0.6 for failed outcome, got %f", evt.Importance)
	}
	if !containsSubstring(evt.Summary, "outcome=failed") {
		t.Errorf("expected summary to contain outcome=failed, got %s", evt.Summary)
	}
}

func TestObservationToVerifiedEventCancelledOutcome(t *testing.T) {
	obs := decision.Observation{
		ID:      "obs-cancel-1",
		Kind:    decision.ObservationKindToolResult,
		Outcome: decision.ObservationOutcomeCancelled,
	}
	evt, ok := ObservationToVerifiedEvent(obs, decision.GoalProgressBatchResult{}, decision.ContinuationDecision{})
	if !ok {
		t.Fatal("expected true")
	}
	if evt.Importance != 0.4 {
		t.Errorf("expected importance 0.4 for cancelled outcome, got %f", evt.Importance)
	}
}

const testRawSecretOutput = "B10_SECRET_OUTPUT_98234"

func TestObservationToVerifiedEventContainsErrorCode(t *testing.T) {
	obs := decision.Observation{
		ID:      "obs-err-1",
		Kind:    decision.ObservationKindToolResult,
		Outcome: decision.ObservationOutcomeFailed,
		Evidence: decision.ObservationEvidence{
			Error: &decision.ObservationError{Code: "permission_denied", Message: testRawSecretOutput},
		},
	}
	evt, ok := ObservationToVerifiedEvent(obs, decision.GoalProgressBatchResult{}, decision.ContinuationDecision{})
	if !ok {
		t.Fatal("expected true")
	}
	if !containsSubstring(evt.Summary, "error=permission_denied") {
		t.Errorf("expected summary to contain error code, got %s", evt.Summary)
	}
	if containsSubstring(evt.Summary, testRawSecretOutput) {
		t.Error("raw error message leaked into summary")
	}
}

func TestBoundedIDSetDedup(t *testing.T) {
	s := newBoundedIDSet(4)
	if !s.Add("a") {
		t.Fatal("expected first add to return true")
	}
	if s.Add("a") {
		t.Fatal("expected duplicate add to return false")
	}
	if !s.Contains("a") {
		t.Fatal("expected Contains(a) to be true")
	}
	if s.Contains("b") {
		t.Fatal("expected Contains(b) to be false")
	}
}

func TestBoundedIDSetEvicts(t *testing.T) {
	s := newBoundedIDSet(3)
	s.Add("a")
	s.Add("b")
	s.Add("c")
	s.Add("d")
	if s.Contains("a") {
		t.Fatal("expected a to be evicted")
	}
	if !s.Contains("d") {
		t.Fatal("expected d to be present")
	}
}

func TestEvidenceWindowBoundedEvents(t *testing.T) {
	w := NewReflectionEvidenceWindow()
	for i := 0; i < MaxEvidenceEvents+10; i++ {
		w.AddEvents(mindruntime.VerifiedEvent{ID: string(rune('a'+i%26)) + string(rune('0'+i/26))})
	}
	if len(w.Events) != MaxEvidenceEvents {
		t.Errorf("expected events to be capped at %d, got %d", MaxEvidenceEvents, len(w.Events))
	}
}

func TestEvidenceWindowSkipsEmptyID(t *testing.T) {
	w := NewReflectionEvidenceWindow()
	added := w.AddEvents(mindruntime.VerifiedEvent{}, mindruntime.VerifiedEvent{ID: "valid"})
	if added != 1 {
		t.Errorf("expected 1 event added, got %d", added)
	}
}

func TestEvidenceSelectorDeterministic(t *testing.T) {
	w := NewReflectionEvidenceWindow()
	now := time.Now()
	for i := 0; i < 20; i++ {
		importance := 0.5
		if i%4 == 0 {
			importance = 0.8
		}
		id := fmt.Sprintf("evt-%02d", i)
		w.AddEvents(mindruntime.VerifiedEvent{
			ID:         id,
			Importance: importance,
			Timestamp:  now.Add(time.Duration(i) * time.Second),
		})
	}
	sel := NewReflectionEvidenceSelector()
	result := sel.Select(w)
	if len(result.Events) != SelectorMaxEvents {
		t.Errorf("expected %d events after selection, got %d", SelectorMaxEvents, len(result.Events))
	}

	for i := 1; i < len(result.Events); i++ {
		if result.Events[i-1].Importance < result.Events[i].Importance {
			t.Errorf("events not sorted by importance at index %d", i)
		}
	}
}

func TestReflectionScopeKeyNormalize(t *testing.T) {
	k := ReflectionScopeKey{UserID: "  user1  ", CharacterID: " char1 ", ConversationID: " conv1 "}
	n := k.Normalize()
	if n.UserID != "user1" || n.CharacterID != "char1" || n.ConversationID != "conv1" {
		t.Errorf("normalize failed: %+v", n)
	}
}

func TestReflectionScopeKeyIsZero(t *testing.T) {
	if (ReflectionScopeKey{}).IsZero() != true {
		t.Fatal("expected empty key to be zero")
	}
	if (ReflectionScopeKey{CharacterID: "abc"}).IsZero() != false {
		t.Fatal("expected non-empty key to not be zero")
	}
}

func TestReflectionEventImportanceDeterministic(t *testing.T) {
	cases := []struct {
		outcome  decision.ObservationOutcome
		expected float64
	}{
		{decision.ObservationOutcomeSucceeded, 0.5},
		{decision.ObservationOutcomeFailed, 0.6},
		{decision.ObservationOutcomeCancelled, 0.4},
		{decision.ObservationOutcomeTimedOut, 0.6},
		{decision.ObservationOutcomeSkipped, 0.5},
	}
	for _, c := range cases {
		obs := decision.Observation{Kind: decision.ObservationKindToolResult, Outcome: c.outcome}
		got := reflectionEventImportance(obs, decision.GoalProgressBatchResult{})
		if got != c.expected {
			t.Errorf("expected importance %f for outcome %s, got %f", c.expected, c.outcome, got)
		}
	}
}

func TestReflectionEventImportanceGoalAchieved(t *testing.T) {
	obs := decision.Observation{Kind: decision.ObservationKindToolResult, Outcome: decision.ObservationOutcomeSucceeded}
	progress := decision.GoalProgressBatchResult{
		Results: []decision.GoalProgressResult{
			{GoalID: "g1", Disposition: decision.GoalProgressAchieved},
		},
	}
	if got := reflectionEventImportance(obs, progress); got != 0.8 {
		t.Errorf("expected importance 0.8 when goal achieved, got %f", got)
	}
}

func TestSummaryContainsNoRawOutput(t *testing.T) {
	obs := decision.Observation{
		ID:      "obs-summary-1",
		Kind:    decision.ObservationKindToolResult,
		Outcome: decision.ObservationOutcomeSucceeded,
		Evidence: decision.ObservationEvidence{
			Contents: []decision.ObservationContent{
				{Kind: decision.ObservationContentText, Text: testRawSecretOutput},
			},
		},
	}
	summary := buildStructuredSummary(obs, decision.GoalProgressBatchResult{}, decision.ContinuationDecision{})
	if containsSubstring(summary, testRawSecretOutput) {
		t.Error("summary should not contain raw tool output")
	}
}

func containsSubstring(s, sub string) bool {
	return len(s) >= len(sub) && (sub == "" || (len(s) > 0 && containsHelper(s, sub)))
}

func containsHelper(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
