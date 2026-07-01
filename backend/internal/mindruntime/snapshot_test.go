package mindruntime

import (
	"reflect"
	"testing"
	"time"
)

func TestBuildRuntimeSnapshotGeneratesVersion(t *testing.T) {
	input := completeSnapshotInput()
	input.PreviousVersion = 7

	snapshot := BuildRuntimeSnapshot(input)

	if snapshot.Version != SnapshotVersionV1 {
		t.Fatalf("unexpected snapshot version: %s", snapshot.Version)
	}
	if snapshot.StateVersion != 8 {
		t.Fatalf("unexpected state version: %d", snapshot.StateVersion)
	}
	if snapshot.ID == "" {
		t.Fatal("expected stable snapshot id")
	}
	if snapshot.CreatedAt.Location() != time.UTC {
		t.Fatalf("expected utc created at: %v", snapshot.CreatedAt.Location())
	}
}

func TestBuildTraceFramesKeepsStableOrder(t *testing.T) {
	input := completeSnapshotInput()
	input.Frames = []TraceFrame{
		{Stage: TraceStageExpression, Reference: input.ExpressionPlanRef},
		{Stage: TraceStagePersonality, Reference: input.PersonalityRef},
		{Stage: TraceStageBehavior, Reference: input.BehaviorPlanRef},
		{Stage: TraceStageAppraisal, Reference: input.AppraisalRef},
	}

	frames := BuildTraceFrames(input)

	want := []TraceStage{
		TraceStagePersonality,
		TraceStageAppraisal,
		TraceStageBehavior,
		TraceStageExpression,
	}
	for i, stage := range want {
		if frames[i].Index != i+1 {
			t.Fatalf("unexpected frame index at %d: %d", i, frames[i].Index)
		}
		if frames[i].Stage != stage {
			t.Fatalf("unexpected stage at %d: %s", i, frames[i].Stage)
		}
	}
}

func TestBuildRuntimeSnapshotAddsDiagnosticsForMissingReferences(t *testing.T) {
	input := completeSnapshotInput()
	input.AppraisalRef = RuntimeReference{}
	input.ExpressionPlanRef = RuntimeReference{}

	snapshot := BuildRuntimeSnapshot(input)

	if len(snapshot.Diagnostics) != 2 {
		t.Fatalf("expected two diagnostics, got %#v", snapshot.Diagnostics)
	}
	if snapshot.Diagnostics[0].Code != "missing_appraisal_reference" {
		t.Fatalf("unexpected appraisal diagnostic: %#v", snapshot.Diagnostics[0])
	}
	if snapshot.Diagnostics[1].Code != "missing_expression_plan_reference" {
		t.Fatalf("unexpected expression diagnostic: %#v", snapshot.Diagnostics[1])
	}
}

func TestBuildRuntimeSnapshotIsStableForRepeatedInput(t *testing.T) {
	input := completeSnapshotInput()

	first := BuildRuntimeSnapshot(input)
	second := BuildRuntimeSnapshot(input)

	if !reflect.DeepEqual(first, second) {
		t.Fatalf("expected stable snapshot\nfirst: %#v\nsecond: %#v", first, second)
	}
}

func completeSnapshotInput() RuntimeSnapshotInput {
	now := time.Date(2026, 7, 1, 10, 30, 0, 0, time.FixedZone("CST", 8*60*60))
	return RuntimeSnapshotInput{
		UserID:        "user-1",
		CharacterID:   "char-1",
		InteractionID: "interaction-1",
		CreatedAt:     now,
		PersonalityRef: RuntimeReference{
			ID:      "personality-1",
			Version: "personality-v1",
			Summary: "warm-stable",
		},
		AppraisalRef: RuntimeReference{
			ID:      "appraisal-1",
			Version: "appraisal-v1",
			Summary: "low-risk",
		},
		PsycheStateRef: RuntimeReference{
			ID:      "psyche-1",
			Version: "psyche-v2",
			Summary: "calm",
		},
		RelationshipRef: RuntimeReference{
			ID:      "relationship-1",
			Version: "relationship-v3",
			Summary: "trusted",
		},
		BehaviorPlanRef: RuntimeReference{
			ID:      "behavior-1",
			Version: "behavior-plan-v1",
			Summary: "offer_support",
		},
		ExpressionPlanRef: RuntimeReference{
			ID:      "expression-1",
			Version: "expression-plan-v1",
			Summary: "warm_short",
		},
	}
}
