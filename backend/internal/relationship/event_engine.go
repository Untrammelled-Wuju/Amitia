package relationship

import "time"

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
