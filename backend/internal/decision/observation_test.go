package decision

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"
)

func TestBuildObservationIDDeterministic(t *testing.T) {
	id1 := BuildObservationID("action-001")
	id2 := BuildObservationID("action-001")
	if id1 != id2 {
		t.Fatalf("expected deterministic id, got %s vs %s", id1, id2)
	}
	if len(id1) != 36 {
		t.Fatalf("expected obs:<32 hex>, got %s (len=%d)", id1, len(id1))
	}
}

func TestBuildObservationIDDiffersByActionID(t *testing.T) {
	id1 := BuildObservationID("action-001")
	id2 := BuildObservationID("action-002")
	if id1 == id2 {
		t.Fatalf("expected different ids for different action ids")
	}
}

func TestValidateObservationToolResultValid(t *testing.T) {
	o := Observation{
		Version:        ObservationVersionV1,
		ID:             BuildObservationID("a1"),
		ActionID:       "a1",
		PlanID:         "p1",
		InteractionID:  "i1",
		ConversationID: "c1",
		Kind:           ObservationKindToolResult,
		TargetKind:     ObservationTargetTool,
		Outcome:        ObservationOutcomeSucceeded,
		InvocationID:   "inv-1",
		ToolID:         "builtin/browser/search",
		ObservedAt:     time.Now().UTC(),
	}
	if err := ValidateObservation(o); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateObservationToolResultMissingToolID(t *testing.T) {
	o := Observation{
		Version:        ObservationVersionV1,
		ID:             BuildObservationID("a1"),
		ActionID:       "a1",
		InteractionID:  "i1",
		ConversationID: "c1",
		Kind:           ObservationKindToolResult,
		TargetKind:     ObservationTargetTool,
		Outcome:        ObservationOutcomeSucceeded,
		InvocationID:   "inv-1",
		ObservedAt:     time.Now().UTC(),
	}
	err := ValidateObservation(o)
	if err == nil {
		t.Fatal("expected error for missing toolId")
	}
	if obErr, ok := err.(ObservationBuildError); !ok || obErr.Code != ErrObservationToolMismatch {
		t.Fatalf("expected ErrObservationToolMismatch, got %v", err)
	}
}

func TestValidateObservationToolResultMissingInvocationID(t *testing.T) {
	o := Observation{
		Version:        ObservationVersionV1,
		ID:             BuildObservationID("a1"),
		ActionID:       "a1",
		InteractionID:  "i1",
		ConversationID: "c1",
		Kind:           ObservationKindToolResult,
		TargetKind:     ObservationTargetTool,
		Outcome:        ObservationOutcomeSucceeded,
		ToolID:         "builtin/browser/search",
		ObservedAt:     time.Now().UTC(),
	}
	err := ValidateObservation(o)
	if err == nil {
		t.Fatal("expected error for missing invocationId")
	}
	if obErr, ok := err.(ObservationBuildError); !ok || obErr.Code != ErrObservationInvocationMismatch {
		t.Fatalf("expected ErrObservationInvocationMismatch, got %v", err)
	}
}

func TestValidateObservationNoActionValid(t *testing.T) {
	o := Observation{
		Version:        ObservationVersionV1,
		ID:             BuildObservationID("a1"),
		ActionID:       "a1",
		InteractionID:  "i1",
		ConversationID: "c1",
		Kind:           ObservationKindNoAction,
		TargetKind:     ObservationTargetNone,
		Outcome:        ObservationOutcomeSkipped,
		ObservedAt:     time.Now().UTC(),
	}
	if err := ValidateObservation(o); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateObservationNoActionWithToolID(t *testing.T) {
	o := Observation{
		Version:        ObservationVersionV1,
		ID:             BuildObservationID("a1"),
		ActionID:       "a1",
		InteractionID:  "i1",
		ConversationID: "c1",
		Kind:           ObservationKindNoAction,
		TargetKind:     ObservationTargetNone,
		Outcome:        ObservationOutcomeSkipped,
		ToolID:         "should-not-exist",
		ObservedAt:     time.Now().UTC(),
	}
	err := ValidateObservation(o)
	if err == nil {
		t.Fatal("expected error for no_action with toolId")
	}
}

func TestValidateObservationMaterializationFailureNoInvocation(t *testing.T) {
	o := Observation{
		Version:        ObservationVersionV1,
		ID:             BuildObservationID("a1"),
		ActionID:       "a1",
		InteractionID:  "i1",
		ConversationID: "c1",
		Kind:           ObservationKindMaterializationFailure,
		TargetKind:     ObservationTargetNone,
		Outcome:        ObservationOutcomeNotMaterialized,
		ObservedAt:     time.Now().UTC(),
	}
	if err := ValidateObservation(o); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateObservationMaterializationFailureWithInvocation(t *testing.T) {
	o := Observation{
		Version:        ObservationVersionV1,
		ID:             BuildObservationID("a1"),
		ActionID:       "a1",
		InteractionID:  "i1",
		ConversationID: "c1",
		Kind:           ObservationKindMaterializationFailure,
		TargetKind:     ObservationTargetNone,
		Outcome:        ObservationOutcomeNotMaterialized,
		InvocationID:   "inv-should-be-empty",
		ObservedAt:     time.Now().UTC(),
	}
	err := ValidateObservation(o)
	if err == nil {
		t.Fatal("expected error for materialization_failure with invocationId")
	}
}

func TestValidateObservationRequiresObservedAt(t *testing.T) {
	o := Observation{
		Version:        ObservationVersionV1,
		ID:             BuildObservationID("a1"),
		ActionID:       "a1",
		InteractionID:  "i1",
		ConversationID: "c1",
		Kind:           ObservationKindNoAction,
		TargetKind:     ObservationTargetNone,
		Outcome:        ObservationOutcomeSkipped,
	}
	err := ValidateObservation(o)
	if err == nil {
		t.Fatal("expected error for missing observedAt")
	}
	if obErr, ok := err.(ObservationBuildError); !ok || obErr.Code != ErrObservationTimeMissing {
		t.Fatalf("expected ErrObservationTimeMissing, got %v", err)
	}
}

func TestObservationJSONRoundTrip(t *testing.T) {
	o := Observation{
		Version:        ObservationVersionV1,
		ID:             BuildObservationID("a1"),
		PlanID:         "plan-1",
		ActionID:       "action-1",
		InteractionID:  "i-1",
		RequestID:      "req-1",
		UserID:         "u-1",
		CharacterID:    "c-1",
		ConversationID: "conv-1",
		CandidateID:    "cand-1",
		GoalIDs:        []string{"g-1"},
		IntentionIDs:   []string{"int-1"},
		Kind:           ObservationKindToolResult,
		TargetKind:     ObservationTargetTool,
		Outcome:        ObservationOutcomeSucceeded,
		InvocationID:   "inv-1",
		ExternalCallID: "call-1",
		ToolID:         "builtin/browser/search",
		Evidence: ObservationEvidence{
			Contents: []ObservationContent{
				{Kind: ObservationContentText, Text: "hello"},
			},
			Structured: json.RawMessage(`{"count":5}`),
			SideEffects: []ObservationSideEffect{
				{Kind: "file_written", ResourceURI: "amitia://workspace/a.txt", State: "completed"},
			},
			Resources: []ObservationResource{
				{URI: "amitia://workspace/a.txt", MimeType: "text/plain"},
			},
			Metadata: map[string]any{"runtimeType": "browser"},
		},
		ObservedAt: time.Date(2026, 8, 9, 12, 0, 0, 0, time.UTC),
	}
	if err := ValidateObservation(o); err != nil {
		t.Fatalf("unexpected validation error: %v", err)
	}
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(o); err != nil {
		t.Fatalf("unexpected encode error: %v", err)
	}
	var decoded Observation
	if err := json.NewDecoder(&buf).Decode(&decoded); err != nil {
		t.Fatalf("unexpected decode error: %v", err)
	}
	if decoded.ID != o.ID {
		t.Fatalf("id mismatch: %s vs %s", decoded.ID, o.ID)
	}
	if decoded.Kind != o.Kind || decoded.Outcome != o.Outcome || decoded.TargetKind != o.TargetKind {
		t.Fatalf("kind/outcome/targetKind mismatch")
	}
	if decoded.Evidence.Structured == nil {
		t.Fatalf("structured evidence lost")
	}
	if len(decoded.Evidence.SideEffects) != 1 || decoded.Evidence.SideEffects[0].ResourceURI != "amitia://workspace/a.txt" {
		t.Fatalf("side effects lost or corrupted")
	}
}
