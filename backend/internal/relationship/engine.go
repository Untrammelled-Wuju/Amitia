package relationship

import (
	"math"
	"sort"
	"time"
)

const formulaVersion = "relationship-delta-formula-v1"

func DefaultState() RelationshipState {
	return RelationshipState{
		Trust:            0.5,
		Familiarity:      0.35,
		Security:         0.45,
		Tension:          0.15,
		RepairConfidence: 0.35,
		Boundary:         0.5,
	}
}

func DefaultBudget() ChangeBudget {
	return ChangeBudget{
		MaxPositiveDelta: 0.08,
		MaxNegativeDelta: 0.07,
		MaxTensionDelta:  0.12,
		MaxBoundaryDelta: 0.1,
	}
}

func DefaultTensionDecay() TensionDecayProfile {
	return TensionDecayProfile{
		BaseDecayHourly:  0.04,
		UnresolvedWeight: 0.55,
		SafeDecay:        true,
	}
}

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

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func clampDelta(value, negativeLimit, positiveLimit float64) float64 {
	if value < -negativeLimit {
		return -negativeLimit
	}
	if value > positiveLimit {
		return positiveLimit
	}
	return value
}

func stableDiagnostics(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	sort.Strings(values)
	out := values[:0]
	var last string
	for i, value := range values {
		if i == 0 || value != last {
			out = append(out, value)
			last = value
		}
	}
	return out
}

func clamp01(value float64) float64 {
	return clampRange(0, 1, value)
}

func clampRange(minimum, maximum, value float64) float64 {
	if value < minimum {
		return minimum
	}
	if value > maximum {
		return maximum
	}
	return value
}

func round4(value float64) float64 {
	return math.Round(value*10000) / 10000
}

func DefaultDimensions() RelationshipDimensions {
	now := time.Now()
	return RelationshipDimensions{
		Trust:      DimensionState{Value: 50, Velocity: 0, LastUpdated: now},
		Intimacy:   DimensionState{Value: 35, Velocity: 0, LastUpdated: now},
		Dependency: DimensionState{Value: 30, Velocity: 0, LastUpdated: now},
		Conflict:   DimensionState{Value: 15, Velocity: 0, LastUpdated: now},
		Repair:     DimensionState{Value: 35, Velocity: 0, LastUpdated: now},
	}
}

func DefaultAccumulation() EventAccumulation {
	return EventAccumulation{
		MaxSingleDelta: 8,
		MaxTotalDelta:  12,
		Accumulated:    0,
	}
}

func DimensionsFromState(state RelationshipState) RelationshipDimensions {
	now := time.Now()
	return RelationshipDimensions{
		Trust:      DimensionState{Value: round4(state.Trust * 100), Velocity: 0, LastUpdated: now},
		Intimacy:   DimensionState{Value: round4(state.Familiarity * 100), Velocity: 0, LastUpdated: now},
		Dependency: DimensionState{Value: round4(state.Security * 100), Velocity: 0, LastUpdated: now},
		Conflict:   DimensionState{Value: round4(state.Tension * 100), Velocity: 0, LastUpdated: now},
		Repair:     DimensionState{Value: round4(state.RepairConfidence * 100), Velocity: 0, LastUpdated: now},
	}
}

func StateFromDimensions(dims RelationshipDimensions) RelationshipState {
	return RelationshipState{
		Trust:            round4(clamp01(dims.Trust.Value / 100)),
		Familiarity:      round4(clamp01(dims.Intimacy.Value / 100)),
		Security:         round4(clamp01(dims.Dependency.Value / 100)),
		Tension:          round4(clamp01(dims.Conflict.Value / 100)),
		RepairConfidence: round4(clamp01(dims.Repair.Value / 100)),
		Boundary:         0.5,
	}
}

func ComputeVelocity(current, previous float64, elapsedHours float64) float64 {
	if elapsedHours <= 0 {
		return 0
	}
	return round4((current - previous) / elapsedHours)
}

func ApplyRelationshipEvent(dims *RelationshipDimensions, event RelationshipEvent, accum *EventAccumulation) EventApplyResult {
	if dims == nil {
		d := DefaultDimensions()
		dims = &d
	}
	if accum == nil {
		a := DefaultAccumulation()
		accum = &a
	}

	now := time.Now()
	if event.OccurredAt.IsZero() {
		event.OccurredAt = now
	}

	prev := copyDimensions(*dims)

	intensity := clamp01(event.Intensity)
	confidence := clamp01(event.Confidence)
	if confidence == 0 && event.Confidence == 0 {
		confidence = 0.7
	}
	weight := intensity * confidence

	impacts := computeEventImpacts(*dims, event, weight, accum)
	overflow := []EventImpact{}

	for _, impact := range impacts {
		clamped := impact
		if accum.Accumulated+impact.Delta > accum.MaxTotalDelta {
			overflowDelta := (accum.Accumulated + impact.Delta) - accum.MaxTotalDelta
			clamped.Delta = impact.Delta - overflowDelta
			if clamped.Delta > 0 {
				overflow = append(overflow, EventImpact{
					Dimension: impact.Dimension,
					Delta:     overflowDelta,
					Reason:    "accumulation_overflow",
				})
			}
		}
		if clamped.Delta != 0 {
			applyDimensionDelta(dims, clamped.Dimension, clamped.Delta, event.OccurredAt)
			accum.Accumulated += clamped.Delta
		}
	}

	for _, impact := range overflow {
		if impact.Delta > accum.MaxSingleDelta {
			impact.Delta = accum.MaxSingleDelta
		}
		if accum.Accumulated+impact.Delta <= accum.MaxTotalDelta+accum.MaxSingleDelta*0.5 {
			applyDimensionDelta(dims, impact.Dimension, impact.Delta, event.OccurredAt)
			accum.Accumulated += impact.Delta
		}
	}

	updateVelocities(dims, &prev, event.OccurredAt)

	return EventApplyResult{
		Previous: &prev,
		Next:     copyPtr(*dims),
		Impacts:  impacts,
		Overflow: overflow,
	}
}

func AccumulateEvents(dims *RelationshipDimensions, events []RelationshipEvent) []EventApplyResult {
	if dims == nil {
		d := DefaultDimensions()
		dims = &d
	}
	accum := DefaultAccumulation()
	results := make([]EventApplyResult, 0, len(events))

	causalMap := buildCausalChainMap(events)

	for _, event := range events {
		causalPenalty := computeCausalPenalty(event, causalMap)
		if causalPenalty > 0 {
			event.Intensity = event.Intensity * (1 - causalPenalty)
		}
		event.CausalChain = resolveCausalChain(event, causalMap)
		result := ApplyRelationshipEvent(dims, event, &accum)
		results = append(results, result)
	}

	return results
}

func computeEventImpacts(dims RelationshipDimensions, event RelationshipEvent, weight float64, accum *EventAccumulation) []EventImpact {
	singleCap := accum.MaxSingleDelta
	if singleCap <= 0 {
		singleCap = 8
	}

	switch event.Type {
	case EventTypePositiveInteraction:
		return []EventImpact{
			{Dimension: "trust", Delta: clampDeltaEvent(4.5*weight, singleCap), Reason: string(EventTypePositiveInteraction)},
			{Dimension: "intimacy", Delta: clampDeltaEvent(5.0*weight, singleCap), Reason: string(EventTypePositiveInteraction)},
			{Dimension: "dependency", Delta: clampDeltaEvent(2.5*weight, singleCap), Reason: string(EventTypePositiveInteraction)},
			{Dimension: "conflict", Delta: clampDeltaEvent(-2.0*weight, singleCap), Reason: string(EventTypePositiveInteraction)},
		}
	case EventTypeNegativeInteraction:
		return []EventImpact{
			{Dimension: "trust", Delta: clampDeltaEvent(-3.5*weight, singleCap), Reason: string(EventTypeNegativeInteraction)},
			{Dimension: "intimacy", Delta: clampDeltaEvent(-2.5*weight, singleCap), Reason: string(EventTypeNegativeInteraction)},
			{Dimension: "conflict", Delta: clampDeltaEvent(6.5*weight, singleCap), Reason: string(EventTypeNegativeInteraction)},
			{Dimension: "repair", Delta: clampDeltaEvent(-2.0*weight, singleCap), Reason: string(EventTypeNegativeInteraction)},
		}
	case EventTypeRepairEffort:
		repairFactor := 0.55 + dims.Repair.Value/100*0.35
		return []EventImpact{
			{Dimension: "trust", Delta: clampDeltaEvent(4.0*weight*repairFactor, singleCap), Reason: string(EventTypeRepairEffort)},
			{Dimension: "conflict", Delta: clampDeltaEvent(-4.5*weight*repairFactor, singleCap), Reason: string(EventTypeRepairEffort)},
			{Dimension: "repair", Delta: clampDeltaEvent(5.5*weight*repairFactor, singleCap), Reason: string(EventTypeRepairEffort)},
		}
	case EventTypeRupture:
		return []EventImpact{
			{Dimension: "trust", Delta: clampDeltaEvent(-6.5*weight, singleCap), Reason: string(EventTypeRupture)},
			{Dimension: "intimacy", Delta: clampDeltaEvent(-4.0*weight, singleCap), Reason: string(EventTypeRupture)},
			{Dimension: "conflict", Delta: clampDeltaEvent(9.5*weight, singleCap), Reason: string(EventTypeRupture)},
			{Dimension: "repair", Delta: clampDeltaEvent(-4.5*weight, singleCap), Reason: string(EventTypeRupture)},
		}
	case EventTypeBoundaryCrossing:
		return []EventImpact{
			{Dimension: "trust", Delta: clampDeltaEvent(-3.5*weight, singleCap), Reason: string(EventTypeBoundaryCrossing)},
			{Dimension: "intimacy", Delta: clampDeltaEvent(-2.0*weight, singleCap), Reason: string(EventTypeBoundaryCrossing)},
			{Dimension: "conflict", Delta: clampDeltaEvent(8.5*weight, singleCap), Reason: string(EventTypeBoundaryCrossing)},
			{Dimension: "repair", Delta: clampDeltaEvent(-2.5*weight, singleCap), Reason: string(EventTypeBoundaryCrossing)},
		}
	case EventTypeWithdrawal:
		return []EventImpact{
			{Dimension: "trust", Delta: clampDeltaEvent(-2.5*weight, singleCap), Reason: string(EventTypeWithdrawal)},
			{Dimension: "intimacy", Delta: clampDeltaEvent(-3.5*weight, singleCap), Reason: string(EventTypeWithdrawal)},
			{Dimension: "dependency", Delta: clampDeltaEvent(-2.0*weight, singleCap), Reason: string(EventTypeWithdrawal)},
			{Dimension: "conflict", Delta: clampDeltaEvent(4.0*weight, singleCap), Reason: string(EventTypeWithdrawal)},
		}
	case EventTypeVulnerabilityShare:
		return []EventImpact{
			{Dimension: "trust", Delta: clampDeltaEvent(6.0*weight, singleCap), Reason: string(EventTypeVulnerabilityShare)},
			{Dimension: "intimacy", Delta: clampDeltaEvent(7.0*weight, singleCap), Reason: string(EventTypeVulnerabilityShare)},
			{Dimension: "dependency", Delta: clampDeltaEvent(3.5*weight, singleCap), Reason: string(EventTypeVulnerabilityShare)},
			{Dimension: "repair", Delta: clampDeltaEvent(3.0*weight, singleCap), Reason: string(EventTypeVulnerabilityShare)},
		}
	case EventTypeNeutralInteraction:
		return []EventImpact{
			{Dimension: "intimacy", Delta: clampDeltaEvent(1.5*weight, singleCap), Reason: string(EventTypeNeutralInteraction)},
		}
	default:
		return nil
	}
}

func clampDeltaEvent(value, cap float64) float64 {
	if cap <= 0 {
		cap = 8
	}
	if value > cap {
		return cap
	}
	if value < -cap {
		return -cap
	}
	return round4(value)
}

func applyDimensionDelta(dims *RelationshipDimensions, dimension string, delta float64, updatedAt time.Time) {
	switch dimension {
	case "trust":
		dims.Trust.Value = round4(clamp01Scale(dims.Trust.Value+delta, 0, 100))
		dims.Trust.LastUpdated = updatedAt
	case "intimacy":
		dims.Intimacy.Value = round4(clamp01Scale(dims.Intimacy.Value+delta, 0, 100))
		dims.Intimacy.LastUpdated = updatedAt
	case "dependency":
		dims.Dependency.Value = round4(clamp01Scale(dims.Dependency.Value+delta, 0, 100))
		dims.Dependency.LastUpdated = updatedAt
	case "conflict":
		dims.Conflict.Value = round4(clamp01Scale(dims.Conflict.Value+delta, 0, 100))
		dims.Conflict.LastUpdated = updatedAt
	case "repair":
		dims.Repair.Value = round4(clamp01Scale(dims.Repair.Value+delta, 0, 100))
		dims.Repair.LastUpdated = updatedAt
	}
}

func clamp01Scale(value, minimum, maximum float64) float64 {
	if value < minimum {
		return minimum
	}
	if value > maximum {
		return maximum
	}
	return value
}

func updateVelocities(dims *RelationshipDimensions, prev *RelationshipDimensions, now time.Time) {
	computeAndSet := func(dim *DimensionState, prevDim *DimensionState) {
		if dim.LastUpdated.IsZero() || prevDim.LastUpdated.IsZero() {
			return
		}
		elapsed := now.Sub(prevDim.LastUpdated).Hours()
		if elapsed <= 0 {
			return
		}
		dim.Velocity = ComputeVelocity(dim.Value, prevDim.Value, elapsed)
	}

	computeAndSet(&dims.Trust, &prev.Trust)
	computeAndSet(&dims.Intimacy, &prev.Intimacy)
	computeAndSet(&dims.Dependency, &prev.Dependency)
	computeAndSet(&dims.Conflict, &prev.Conflict)
	computeAndSet(&dims.Repair, &prev.Repair)
}

func copyDimensions(dims RelationshipDimensions) RelationshipDimensions {
	return RelationshipDimensions{
		Trust:      DimensionState{Value: dims.Trust.Value, Velocity: dims.Trust.Velocity, LastUpdated: dims.Trust.LastUpdated},
		Intimacy:   DimensionState{Value: dims.Intimacy.Value, Velocity: dims.Intimacy.Velocity, LastUpdated: dims.Intimacy.LastUpdated},
		Dependency: DimensionState{Value: dims.Dependency.Value, Velocity: dims.Dependency.Velocity, LastUpdated: dims.Dependency.LastUpdated},
		Conflict:   DimensionState{Value: dims.Conflict.Value, Velocity: dims.Conflict.Velocity, LastUpdated: dims.Conflict.LastUpdated},
		Repair:     DimensionState{Value: dims.Repair.Value, Velocity: dims.Repair.Velocity, LastUpdated: dims.Repair.LastUpdated},
	}
}

func copyPtr(dims RelationshipDimensions) *RelationshipDimensions {
	c := copyDimensions(dims)
	return &c
}

func buildCausalChainMap(events []RelationshipEvent) map[string]*RelationshipEvent {
	m := make(map[string]*RelationshipEvent, len(events))
	for i := range events {
		if events[i].ID != "" {
			m[events[i].ID] = &events[i]
		}
	}
	return m
}

func resolveCausalChain(event RelationshipEvent, causalMap map[string]*RelationshipEvent) []string {
	chain := make([]string, 0)
	visited := make(map[string]bool)
	currentID := event.ParentEventID

	for depth := 0; depth < 10 && currentID != ""; depth++ {
		if visited[currentID] {
			break
		}
		visited[currentID] = true
		chain = append(chain, currentID)
		parent, exists := causalMap[currentID]
		if !exists {
			break
		}
		currentID = parent.ParentEventID
	}

	return chain
}

func computeCausalPenalty(event RelationshipEvent, causalMap map[string]*RelationshipEvent) float64 {
	if event.ParentEventID == "" {
		return 0
	}
	depth := len(resolveCausalChain(event, causalMap))
	if depth <= 0 {
		return 0
	}
	penalty := 0.10 + float64(depth)*0.05
	if penalty > 0.35 {
		penalty = 0.35
	}
	return penalty
}
