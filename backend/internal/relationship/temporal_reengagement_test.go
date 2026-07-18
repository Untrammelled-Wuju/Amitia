package relationship

import "testing"

func TestTemporalReengagementHasNoPermanentImpact(t *testing.T) {
	accum := DefaultAccumulation()
	impacts := computeEventImpacts(DefaultDimensions(), RelationshipEvent{Type: EventTypeTemporalReengagement, Intensity: 1, Confidence: 1}, 1, &accum)
	if len(impacts) != 0 {
		t.Fatalf("temporal reengagement changed permanent relationship %#v", impacts)
	}
}

func TestTemporalReengagementAloneKeepsNeutralNarrative(t *testing.T) {
	dimensions := DefaultDimensions()
	dimensions.Trust.Value = 10
	dimensions.Intimacy.Value = 10
	narrative := ComputeNarrative(dimensions, []RelationshipEvent{{Type: EventTypeTemporalReengagement, Intensity: 1, Confidence: 1}})
	if narrative.Tone != NarrativeNeutral {
		t.Fatalf("temporal reengagement produced punitive narrative %#v", narrative)
	}
}
