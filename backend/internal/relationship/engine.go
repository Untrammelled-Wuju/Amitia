package relationship

import (
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
