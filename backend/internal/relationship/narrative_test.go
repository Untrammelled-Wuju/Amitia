package relationship

import (
	"testing"
)

func TestDefaultNarrativeIsNeutral(t *testing.T) {
	n := DefaultNarrative()
	if n.Tone != NarrativeNeutral {
		t.Fatalf("expected neutral tone, got %s", n.Tone)
	}
	if n.Confidence != 0 {
		t.Fatalf("expected confidence 0, got %v", n.Confidence)
	}
}

func TestComputeNarrativePositiveWithManyPositiveEvents(t *testing.T) {
	dims := DefaultDimensions()
	dims.Trust.Value = 75
	events := make([]RelationshipEvent, 10)
	for i := range events {
		events[i] = RelationshipEvent{
			Type:       EventTypePositiveInteraction,
			Intensity:  0.8,
			Confidence: 0.8,
		}
	}

	n := ComputeNarrative(dims, events)
	if n.Tone != NarrativePositive {
		t.Fatalf("expected positive tone, got %s", n.Tone)
	}
	if n.Confidence <= 0 {
		t.Fatalf("expected positive confidence, got %v", n.Confidence)
	}
}

func TestComputeNarrativeTenseWithManyNegativeEvents(t *testing.T) {
	dims := DefaultDimensions()
	dims.Conflict.Value = 65
	events := make([]RelationshipEvent, 10)
	for i := range events {
		events[i] = RelationshipEvent{
			Type:       EventTypeNegativeInteraction,
			Intensity:  0.8,
			Confidence: 0.8,
		}
	}

	n := ComputeNarrative(dims, events)
	if n.Tone != NarrativeTense {
		t.Fatalf("expected tense tone, got %s", n.Tone)
	}
}

func TestComputeNarrativeRecoveringWhenRepairing(t *testing.T) {
	dims := DefaultDimensions()
	dims.Repair.Value = 65
	events := []RelationshipEvent{
		{Type: EventTypeRupture, Intensity: 0.8, Confidence: 0.8},
		{Type: EventTypeRepairEffort, Intensity: 0.9, Confidence: 0.9},
		{Type: EventTypeRepairEffort, Intensity: 0.8, Confidence: 0.8},
	}

	n := ComputeNarrative(dims, events)
	if n.Tone != NarrativeRecovering {
		t.Fatalf("expected recovering tone, got %s", n.Tone)
	}
}

func TestComputeNarrativeDistantWhenLowIntimacy(t *testing.T) {
	dims := DefaultDimensions()
	dims.Trust.Value = 25
	dims.Intimacy.Value = 20
	events := []RelationshipEvent{
		{Type: EventTypeWithdrawal, Intensity: 0.8, Confidence: 0.8},
	}

	n := ComputeNarrative(dims, events)
	if n.Tone != NarrativeDistant {
		t.Fatalf("expected distant tone, got %s", n.Tone)
	}
}

func TestComputeNarrativeNeutralWhenMixed(t *testing.T) {
	dims := DefaultDimensions()
	events := []RelationshipEvent{
		{Type: EventTypePositiveInteraction, Intensity: 0.5, Confidence: 0.5},
		{Type: EventTypeNegativeInteraction, Intensity: 0.5, Confidence: 0.5},
	}

	n := ComputeNarrative(dims, events)
	if n.Tone != NarrativeNeutral {
		t.Fatalf("expected neutral tone for mixed, got %s", n.Tone)
	}
}

func TestComputeNarrativeFromStatePositive(t *testing.T) {
	state := DefaultState()
	state.Trust = 0.75
	state.Familiarity = 0.70
	state.Tension = 0.10

	n := ComputeNarrativeFromState(state, nil)
	if n.Tone != NarrativePositive {
		t.Fatalf("expected positive tone from state, got %s", n.Tone)
	}
}

func TestComputeNarrativeFromStateTense(t *testing.T) {
	state := DefaultState()
	state.Tension = 0.7

	n := ComputeNarrativeFromState(state, nil)
	if n.Tone != NarrativeTense {
		t.Fatalf("expected tense tone from high tension, got %s", n.Tone)
	}
}

func TestNarrativeEvidenceWeightPositive(t *testing.T) {
	w := NarrativeEvidenceWeight(NarrativePositive, 0.8)
	if w <= 0 {
		t.Fatalf("expected positive weight, got %v", w)
	}
}

func TestNarrativeEvidenceWeightTense(t *testing.T) {
	w := NarrativeEvidenceWeight(NarrativeTense, 0.8)
	if w >= 0 {
		t.Fatalf("expected negative weight for tense, got %v", w)
	}
}
