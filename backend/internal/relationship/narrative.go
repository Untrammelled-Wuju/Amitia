package relationship

import (
	"time"
)

func DefaultNarrative() NarrativeSummary {
	return NarrativeSummary{
		Tone:       NarrativeNeutral,
		Summary:    "",
		Confidence: 0.0,
		UpdatedAt:  time.Now(),
	}
}

func ComputeNarrative(dims RelationshipDimensions, events []RelationshipEvent) NarrativeSummary {
	if events == nil || len(events) == 0 {
		return DefaultNarrative()
	}

	positives := 0
	negatives := 0
	ruptures := 0
	repairs := 0
	total := len(events)

	for _, e := range events {
		switch e.Type {
		case EventTypePositiveInteraction, EventTypeVulnerabilityShare:
			positives++
		case EventTypeNegativeInteraction, EventTypeBoundaryCrossing, EventTypeWithdrawal:
			negatives++
		case EventTypeRupture:
			negatives++
			ruptures++
		case EventTypeRepairEffort:
			repairs++
		}
	}

	trust := dims.Trust.Value / 100
	intimacy := dims.Intimacy.Value / 100
	conflict := dims.Conflict.Value / 100
	repair := dims.Repair.Value / 100

	positivityRatio := float64(positives) / float64(total)
	negativityRatio := float64(negatives) / float64(total)
	repairRatio := 0.0
	if ruptures > 0 {
		repairRatio = float64(repairs) / float64(ruptures)
	}

	var tone NarrativeTone
	confidence := 0.6

	if positivityRatio > 0.6 && negativityRatio < 0.15 && trust > 0.6 {
		tone = NarrativePositive
		confidence = clamp01(positivityRatio*0.7 + trust*0.3)
	} else if negativityRatio > 0.3 && conflict > 0.5 {
		tone = NarrativeTense
		confidence = clamp01(negativityRatio*0.7 + conflict*0.3)
	} else if repairRatio >= 1.0 && repair > 0.5 {
		tone = NarrativeRecovering
		confidence = clamp01(repairRatio*0.5 + repair*0.5)
	} else if intimacy < 0.3 && trust < 0.35 {
		tone = NarrativeDistant
		confidence = clamp01((1-intimacy)*0.5 + (1-trust)*0.5)
	} else {
		tone = NarrativeNeutral
		confidence = 0.50
	}

	return NarrativeSummary{
		Tone:       tone,
		Summary:    "",
		Confidence: round4(confidence),
		UpdatedAt:  time.Now(),
	}
}

func ComputeNarrativeFromState(state RelationshipState, dims *RelationshipDimensions) NarrativeSummary {
	if dims == nil {
		d := DimensionsFromState(state)
		dims = &d
	}

	trust := dims.Trust.Value / 100
	intimacy := dims.Intimacy.Value / 100
	conflict := dims.Conflict.Value / 100
	repair := dims.Repair.Value / 100

	var tone NarrativeTone
	confidence := 0.50

	if trust > 0.65 && intimacy > 0.60 && conflict < 0.2 {
		tone = NarrativePositive
		confidence = clamp01((trust*0.4 + intimacy*0.3 + (1-conflict)*0.3))
	} else if conflict > 0.55 || state.Tension > 0.6 {
		tone = NarrativeTense
		confidence = clamp01(conflict*0.7 + state.Tension*0.3)
	} else if repair > 0.55 && conflict > 0.25 {
		tone = NarrativeRecovering
		confidence = clamp01(repair*0.6 + (1-conflict)*0.4)
	} else if trust < 0.3 || intimacy < 0.25 {
		tone = NarrativeDistant
		confidence = clamp01((1-trust)*0.5 + (1-intimacy)*0.5)
	} else {
		tone = NarrativeNeutral
		confidence = 0.45
	}

	return NarrativeSummary{
		Tone:       tone,
		Summary:    "",
		Confidence: clamp01(confidence),
		UpdatedAt:  time.Now(),
	}
}

func NarrativeEvidenceWeight(tone NarrativeTone, confidence float64) float64 {
	switch tone {
	case NarrativePositive:
		return 1.0 + confidence*0.2
	case NarrativeTense:
		return -1.0 - (1-confidence)*0.2
	case NarrativeRecovering:
		return 0.5 + confidence*0.15
	case NarrativeDistant:
		return -0.5 - (1-confidence)*0.15
	default:
		return 0.1
	}
}