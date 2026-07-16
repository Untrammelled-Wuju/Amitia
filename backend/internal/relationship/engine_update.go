package relationship

import (
	"math"
	"sort"
)

func UpdateRelationship(input UpdateInput) UpdateResult {
	diagnostics := []string{}
	previous := normalizeState(input.Current, &diagnostics)
	budget := normalizeBudget(input.Budget, &diagnostics)
	personality := normalizePersonality(input.Personality, &diagnostics)
	raw := RelationshipDelta{}
	evidenceIDs := []string{}
	boundarySignal := false
	conflictSignal := false
	safetySignal := false
	conflictEvents := []ConflictEvent{}

	for _, item := range input.Evidence {
		intensity := clamp01(item.Intensity)
		confidence := clamp01(item.Confidence)
		if confidence == 0 && item.Confidence == 0 {
			confidence = 0.7
		}
		weight := intensity * confidence
		if item.ID != "" {
			evidenceIDs = append(evidenceIDs, item.ID)
		}
		securityBuffer := previous.Security * (0.45 + personality.Tolerance*0.25)
		attachmentPressure := personality.Attachment * (1 - previous.Security*0.65)
		tensionBefore := previous.Tension + raw.Tension
		tensionAfter := tensionBefore

		switch item.Kind {
		case EvidenceKindPositive:
			raw.Trust += 0.045 * weight * (0.7 + personality.Warmth*0.3)
			raw.Familiarity += 0.06 * weight * (0.75 + personality.Affection*0.25)
			raw.Security += 0.035 * weight * (0.75 + personality.Tolerance*0.25)
			raw.Tension -= 0.025 * weight * (0.7 + personality.Tolerance*0.3)
		case EvidenceKindSupportive:
			raw.Trust += 0.07 * weight * (0.75 + personality.Warmth*0.25)
			raw.Familiarity += 0.035 * weight
			raw.Security += 0.05 * weight * (0.7 + personality.Tolerance*0.3)
			raw.Tension -= 0.035 * weight * (0.6 + personality.Tolerance*0.4)
		case EvidenceKindRepair:
			repairFactor := 0.55 + previous.RepairConfidence*0.35 + personality.Tolerance*0.1
			raw.Trust += 0.04 * weight * repairFactor
			raw.Security += 0.045 * weight * repairFactor
			raw.Tension -= 0.045 * weight * repairFactor * (0.65 + previous.RepairConfidence*0.35)
			raw.RepairConfidence += 0.055 * weight * (0.65 + personality.Tolerance*0.35)
			raw.Boundary -= 0.025 * weight * (0.6 + personality.Warmth*0.4)
			tensionAfter = clamp01(tensionBefore - 0.045*weight*repairFactor*(0.65+previous.RepairConfidence*0.35))
		case EvidenceKindConflict:
			conflictSignal = true
			raw.Trust -= 0.065 * weight * (0.75 + personality.Sensitivity*0.25) * (1 - securityBuffer*0.35)
			raw.Familiarity += 0.012 * weight
			raw.Security -= 0.05 * weight * (0.75 + personality.Sensitivity*0.25) * (1 - previous.Security*0.25)
			raw.Tension += 0.095 * weight * (0.65 + personality.Sensitivity*0.35) * (1 - previous.RepairConfidence*0.2)
			raw.RepairConfidence -= 0.04 * weight * (0.7 + personality.Sensitivity*0.3)
			raw.Boundary += 0.035 * weight * (0.65 + personality.BoundaryStrength*0.35)
			tensionAfter = clamp01(tensionBefore + 0.095*weight*(0.65+personality.Sensitivity*0.35)*(1-previous.RepairConfidence*0.2))
			conflictEvents = append(conflictEvents, ConflictEvent{
				SourceEvidence: item.ID,
				Intensity:      intensity,
				TensionBefore:  round4(tensionBefore),
				TensionAfter:   round4(tensionAfter),
				SafetyRelated:  false,
				BoundaryActive: false,
			})
		case EvidenceKindSafety:
			safetySignal = true
			raw.Trust -= 0.11 * weight * (0.75 + personality.Sensitivity*0.25) * (1 - securityBuffer*0.2)
			raw.Security -= 0.13 * weight * (0.75 + personality.Sensitivity*0.25)
			raw.Tension += 0.11 * weight * (0.8 + personality.ConflictAvoidance*0.2)
			raw.RepairConfidence -= 0.08 * weight * (0.8 + personality.Sensitivity*0.2)
			raw.Boundary += 0.12 * weight * (0.75 + personality.BoundaryStrength*0.25)
			tensionAfter = clamp01(tensionBefore + 0.11*weight*(0.8+personality.ConflictAvoidance*0.2))
			conflictEvents = append(conflictEvents, ConflictEvent{
				SourceEvidence: item.ID,
				Intensity:      intensity,
				TensionBefore:  round4(tensionBefore),
				TensionAfter:   round4(tensionAfter),
				SafetyRelated:  true,
				BoundaryActive: false,
			})
		case EvidenceKindBoundary:
			boundarySignal = true
			raw.Trust -= 0.035 * weight * (0.7 + personality.Sensitivity*0.3) * (1 - securityBuffer*0.25)
			raw.Familiarity += 0.006 * weight
			raw.Security -= 0.06 * weight * (0.7 + personality.Sensitivity*0.3)
			raw.Tension += 0.085 * weight * (0.7 + personality.ConflictAvoidance*0.3)
			raw.RepairConfidence -= 0.025 * weight * (0.65 + personality.Sensitivity*0.35)
			raw.Boundary += 0.09 * weight * (0.75 + personality.BoundaryStrength*0.25)
			tensionAfter = clamp01(tensionBefore + 0.085*weight*(0.7+personality.ConflictAvoidance*0.3))
			conflictEvents = append(conflictEvents, ConflictEvent{
				SourceEvidence: item.ID,
				Intensity:      intensity,
				TensionBefore:  round4(tensionBefore),
				TensionAfter:   round4(tensionAfter),
				SafetyRelated:  false,
				BoundaryActive: true,
			})
		case EvidenceKindWithdrawal:
			trustImpact := 0.03 * weight * (0.65 + personality.Sensitivity*0.35) * (1 - previous.Security)
			if weight < 0.85 || previous.Security >= 0.65 {
				trustImpact = 0
				diagnostics = append(diagnostics, "secure_absence_buffer")
			}
			raw.Trust -= trustImpact
			raw.Familiarity -= 0.018 * weight * (1 - previous.Security*0.35)
			raw.Security -= 0.025 * weight * (1 - previous.Security*0.4)
			raw.Tension += 0.04 * weight * (0.65 + personality.Sensitivity*0.2 + attachmentPressure*0.15) * (1 - previous.Security*0.45)
			tensionAfter = clamp01(tensionBefore + 0.04*weight*(0.65+personality.Sensitivity*0.2+attachmentPressure*0.15)*(1-previous.Security*0.45))
		default:
			diagnostics = append(diagnostics, "unknown_evidence_kind")
		}
	}

	if boundarySignal || conflictSignal || safetySignal {
		raw.Familiarity = math.Min(raw.Familiarity, budget.MaxPositiveDelta*0.35)
		if raw.Trust > 0 {
			raw.Trust = math.Min(raw.Trust, budget.MaxPositiveDelta*0.45)
		}
		diagnostics = append(diagnostics, "conservative_boundary_mode")
	}
	if safetySignal {
		if raw.Tension > 0 {
			raw.Tension = math.Min(raw.Tension, budget.MaxTensionDelta*0.65)
		}
		if raw.Boundary > 0 {
			raw.Boundary = math.Min(raw.Boundary, budget.MaxBoundaryDelta*0.55)
		}
		if raw.RepairConfidence < 0 {
			raw.RepairConfidence = math.Max(raw.RepairConfidence, -budget.MaxNegativeDelta*0.7)
		}
		diagnostics = append(diagnostics, "safety_event_separated")
	}

	delta := RelationshipDelta{
		Trust:            round4(clampDelta(raw.Trust, budget.MaxNegativeDelta, budget.MaxPositiveDelta)),
		Familiarity:      round4(clampDelta(raw.Familiarity, budget.MaxNegativeDelta, budget.MaxPositiveDelta)),
		Security:         round4(clampDelta(raw.Security, budget.MaxNegativeDelta, budget.MaxPositiveDelta)),
		Tension:          round4(clampDelta(raw.Tension, budget.MaxTensionDelta, budget.MaxTensionDelta)),
		RepairConfidence: round4(clampDelta(raw.RepairConfidence, budget.MaxNegativeDelta, budget.MaxPositiveDelta)),
		Boundary:         round4(clampDelta(raw.Boundary, budget.MaxBoundaryDelta, budget.MaxBoundaryDelta)),
	}
	next := RelationshipState{
		Trust:            round4(clamp01(previous.Trust + delta.Trust)),
		Familiarity:      round4(clamp01(previous.Familiarity + delta.Familiarity)),
		Security:         round4(clamp01(previous.Security + delta.Security)),
		Tension:          round4(clamp01(previous.Tension + delta.Tension)),
		RepairConfidence: round4(clamp01(previous.RepairConfidence + delta.RepairConfidence)),
		Boundary:         round4(clamp01(previous.Boundary + delta.Boundary)),
	}
	sort.Strings(evidenceIDs)
	diagnostics = stableDiagnostics(diagnostics)

	return UpdateResult{
		Version:  EngineVersionV1,
		Previous: previous,
		Delta:    delta,
		Next:     next,
		Budget:   budget,
		Audit: RelationshipAudit{
			FormulaVersion:     formulaVersion,
			PersonalityVersion: personality.Version,
			EvidenceIDs:        evidenceIDs,
			Diagnostics:        diagnostics,
		},
	}
}

func normalizeState(state RelationshipState, diagnostics *[]string) RelationshipState {
	normalized := RelationshipState{
		Trust:            clamp01(state.Trust),
		Familiarity:      clamp01(state.Familiarity),
		Security:         clamp01(state.Security),
		Tension:          clamp01(state.Tension),
		RepairConfidence: clamp01(state.RepairConfidence),
		Boundary:         clamp01(state.Boundary),
	}
	if state == (RelationshipState{}) {
		*diagnostics = append(*diagnostics, "default_state")
		return DefaultState()
	}
	if normalized != state {
		*diagnostics = append(*diagnostics, "state_clamped")
	}
	return normalized
}

func normalizeBudget(budget ChangeBudget, diagnostics *[]string) ChangeBudget {
	defaults := DefaultBudget()
	if budget.MaxPositiveDelta <= 0 {
		budget.MaxPositiveDelta = defaults.MaxPositiveDelta
	}
	if budget.MaxNegativeDelta <= 0 {
		budget.MaxNegativeDelta = defaults.MaxNegativeDelta
	}
	if budget.MaxTensionDelta <= 0 {
		budget.MaxTensionDelta = defaults.MaxTensionDelta
	}
	if budget.MaxBoundaryDelta <= 0 {
		budget.MaxBoundaryDelta = defaults.MaxBoundaryDelta
	}
	normalized := ChangeBudget{
		MaxPositiveDelta: clampRange(0.005, 0.2, budget.MaxPositiveDelta),
		MaxNegativeDelta: clampRange(0.005, 0.2, budget.MaxNegativeDelta),
		MaxTensionDelta:  clampRange(0.005, 0.25, budget.MaxTensionDelta),
		MaxBoundaryDelta: clampRange(0.005, 0.2, budget.MaxBoundaryDelta),
	}
	if normalized != budget {
		*diagnostics = append(*diagnostics, "budget_clamped")
	}
	return normalized
}

func normalizePersonality(personality PersonalityRef, diagnostics *[]string) PersonalityRef {
	if personality == (PersonalityRef{}) {
		*diagnostics = append(*diagnostics, "default_personality")
		return PersonalityRef{
			Warmth:            0.6,
			Affection:         0.45,
			Attachment:        0.45,
			BoundaryStrength:  0.55,
			Sensitivity:       0.45,
			Tolerance:         0.55,
			ConflictAvoidance: 0.5,
		}
	}
	return PersonalityRef{
		Version:           personality.Version,
		Warmth:            clamp01(personality.Warmth),
		Affection:         clamp01(personality.Affection),
		Attachment:        clamp01(personality.Attachment),
		BoundaryStrength:  clamp01(personality.BoundaryStrength),
		Sensitivity:       clamp01(personality.Sensitivity),
		Tolerance:         clamp01(personality.Tolerance),
		ConflictAvoidance: clamp01(personality.ConflictAvoidance),
	}
}

func ComputeTensionDecay(current RelationshipState, hoursElapsed float64, profile TensionDecayProfile) float64 {
	if hoursElapsed <= 0 {
		return current.Tension
	}
	decayFactor := 1.0
	if profile.BaseDecayHourly > 0 {
		decayFactor = math.Exp(-math.Ln2 * hoursElapsed * 0.85 / (1 / profile.BaseDecayHourly))
	}
	residual := current.Tension * decayFactor
	if current.RepairConfidence > 0.6 {
		boost := (current.RepairConfidence - 0.6) * 0.35
		residual = residual * (1 - boost)
	}
	return clamp01(residual)
}

func ComputeRepairConfidenceBase(repairHistory []RepairRecord) float64 {
	if len(repairHistory) == 0 {
		return 0.35
	}
	successful := 0
	recent := 0
	for i, r := range repairHistory {
		if r.Effective {
			successful++
			if i >= len(repairHistory)-3 {
				recent++
			}
		}
	}
	rate := float64(successful) / float64(len(repairHistory))
	recentRate := 0.0
	if len(repairHistory) >= 3 {
		recentRate = float64(recent) / float64(minInt(3, len(repairHistory)))
	}
	base := 0.15 + rate*0.5 + recentRate*0.15
	return clamp01(base)
}
