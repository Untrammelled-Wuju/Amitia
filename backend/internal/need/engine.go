package need

import (
	"math"
	"sort"
	"time"
)

const formulaVersion = "need-regulation-formula-v1"
const saturationThreshold = 0.88

func DefaultSnapshot(now time.Time) NeedSnapshot {
	return NeedSnapshot{
		Version: EngineVersionV1,
		States: map[NeedKind]NeedState{
			NeedKindReassurance: {Level: 0.32, Baseline: 0.28, Trend: 0, UpdatedAt: now},
			NeedKindConnection:  {Level: 0.38, Baseline: 0.34, Trend: 0, UpdatedAt: now},
			NeedKindAutonomy:    {Level: 0.3, Baseline: 0.3, Trend: 0, UpdatedAt: now},
			NeedKindClarity:     {Level: 0.26, Baseline: 0.24, Trend: 0, UpdatedAt: now},
			NeedKindRest:        {Level: 0.22, Baseline: 0.2, Trend: 0, UpdatedAt: now},
			NeedKindExpression:  {Level: 0.28, Baseline: 0.25, Trend: 0, UpdatedAt: now},
			NeedKindNovelty:     {Level: 0.24, Baseline: 0.22, Trend: 0, UpdatedAt: now},
		},
		UpdatedAt: now,
	}
}

func DefaultBudget() ChangeBudget {
	return ChangeBudget{
		MaxLevelDelta: 0.14,
		MaxTrendDelta: 0.1,
	}
}

func UpdateNeeds(input UpdateInput) UpdateResult {
	now := input.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}
	diagnostics := []string{}
	before := normalizeSnapshot(input.Current, now, &diagnostics)
	personality := normalizePersonality(input.Personality, &diagnostics)
	budget := normalizeBudget(input.Budget, &diagnostics)
	decayed, elapsed := decaySnapshot(before, personality, now)
	if elapsed > 0 {
		diagnostics = append(diagnostics, "decay_applied")
	}

	rawDelta := map[NeedKind]NeedDelta{}
	for _, signal := range input.Signals {
		kind := normalizeKind(signal.Kind)
		pressure := clamp01(signal.Pressure)
		relief := clamp01(signal.Relief)
		confidence := clamp01(signal.Confidence)
		if signal.Confidence == 0 {
			confidence = 0.75
		}
		net := (pressure - relief) * confidence
		levelFactor, trendFactor := signalFactors(kind, personality)
		item := rawDelta[kind]
		item.Level += net * levelFactor
		item.Trend += net * trendFactor
		rawDelta[kind] = item
	}

	delta := map[NeedKind]NeedDelta{}
	afterStates := map[NeedKind]NeedState{}
	for _, kind := range orderedKinds() {
		state := decayed.States[kind]
		item := rawDelta[kind]
		if item.Level != 0 && math.Abs(item.Level) > budget.MaxLevelDelta {
			diagnostics = append(diagnostics, "budget_clamped")
		}
		if item.Trend != 0 && math.Abs(item.Trend) > budget.MaxTrendDelta {
			diagnostics = append(diagnostics, "budget_clamped")
		}
		applied := NeedDelta{
			Level: round4(clampSigned(item.Level, budget.MaxLevelDelta)),
			Trend: round4(clampSigned(item.Trend, budget.MaxTrendDelta)),
		}
		level := round4(clampRange(0, 1, state.Level+applied.Level))
		trend := round4(clampRange(-1, 1, state.Trend+applied.Trend))
		saturated := isNeedSaturated(level, trend)
		if saturated {
			diagnostics = append(diagnostics, "need_saturated:"+string(kind))
		}
		next := NeedState{
			Level:     level,
			Baseline:  state.Baseline,
			Trend:     trend,
			Saturated: saturated,
			UpdatedAt: now,
		}
		delta[kind] = applied
		afterStates[kind] = next
	}

	if len(input.Signals) == 0 {
		diagnostics = append(diagnostics, "no_signals")
	}

	return UpdateResult{
		Version: EngineVersionV1,
		Before:  before,
		Delta:   delta,
		After: NeedSnapshot{
			Version:   EngineVersionV1,
			States:    afterStates,
			UpdatedAt: now,
		},
		Budget: budget,
		Audit: Audit{
			FormulaVersion:     formulaVersion,
			PersonalityVersion: personality.Version,
			ElapsedHours:       round4(elapsed),
			Diagnostics:        stableDiagnostics(diagnostics),
		},
	}
}

func normalizeSnapshot(snapshot NeedSnapshot, now time.Time, diagnostics *[]string) NeedSnapshot {
	if snapshot.Version == "" || len(snapshot.States) == 0 {
		*diagnostics = append(*diagnostics, "default_snapshot")
		return DefaultSnapshot(now)
	}
	normalized := DefaultSnapshot(now)
	normalized.Version = EngineVersionV1
	normalized.UpdatedAt = snapshot.UpdatedAt
	if normalized.UpdatedAt.IsZero() {
		normalized.UpdatedAt = now
	}
	for _, kind := range orderedKinds() {
		state, ok := snapshot.States[kind]
		if !ok {
			continue
		}
		if state.UpdatedAt.IsZero() {
			state.UpdatedAt = normalized.UpdatedAt
		}
		level := clampRange(0, 1, state.Level)
		trend := clampRange(-1, 1, state.Trend)
		normalized.States[kind] = NeedState{
			Level:     level,
			Baseline:  clampRange(0, 1, state.Baseline),
			Trend:     trend,
			Saturated: isNeedSaturated(level, trend),
			UpdatedAt: state.UpdatedAt,
		}
	}
	return normalized
}

func normalizePersonality(input PersonalityRef, diagnostics *[]string) PersonalityRef {
	if input == (PersonalityRef{}) {
		*diagnostics = append(*diagnostics, "default_personality")
		return PersonalityRef{
			Version:          "need-personality-ref-v1",
			Sensitivity:      0.5,
			Stability:        0.58,
			RecoveryBias:     0.55,
			AttachmentBias:   0.52,
			BoundaryStrength: 0.56,
		}
	}
	return PersonalityRef{
		Version:          input.Version,
		Sensitivity:      clamp01(input.Sensitivity),
		Stability:        clamp01(input.Stability),
		RecoveryBias:     clamp01(input.RecoveryBias),
		AttachmentBias:   clamp01(input.AttachmentBias),
		BoundaryStrength: clamp01(input.BoundaryStrength),
	}
}

func normalizeBudget(input ChangeBudget, diagnostics *[]string) ChangeBudget {
	defaults := DefaultBudget()
	if input.MaxLevelDelta <= 0 {
		input.MaxLevelDelta = defaults.MaxLevelDelta
	}
	if input.MaxTrendDelta <= 0 {
		input.MaxTrendDelta = defaults.MaxTrendDelta
	}
	normalized := ChangeBudget{
		MaxLevelDelta: clampRange(0.01, 0.4, input.MaxLevelDelta),
		MaxTrendDelta: clampRange(0.01, 0.3, input.MaxTrendDelta),
	}
	if normalized != input {
		*diagnostics = append(*diagnostics, "budget_clamped")
	}
	return normalized
}

func decaySnapshot(snapshot NeedSnapshot, personality PersonalityRef, now time.Time) (NeedSnapshot, float64) {
	if !now.After(snapshot.UpdatedAt) {
		return snapshot, 0
	}
	elapsed := now.Sub(snapshot.UpdatedAt).Hours()
	recoveryBoost := 0.8 + personality.RecoveryBias*0.4
	levelHalfLife := clampRange(6, 42, 14+(1-personality.Stability)*10+(1-personality.RecoveryBias)*6)
	trendHalfLife := clampRange(4, 24, 8+(1-personality.Stability)*6)
	levelDecay := decayFactor(elapsed, levelHalfLife, recoveryBoost)
	trendDecay := decayFactor(elapsed, trendHalfLife, recoveryBoost)

	next := NeedSnapshot{
		Version:   EngineVersionV1,
		States:    map[NeedKind]NeedState{},
		UpdatedAt: now,
	}
	for _, kind := range orderedKinds() {
		state := snapshot.States[kind]
		level := state.Baseline + (state.Level-state.Baseline)*levelDecay
		nextLevel := round4(clampRange(0, 1, level))
		nextTrend := round4(clampRange(-1, 1, state.Trend*trendDecay))
		next.States[kind] = NeedState{
			Level:     nextLevel,
			Baseline:  state.Baseline,
			Trend:     nextTrend,
			Saturated: isNeedSaturated(nextLevel, nextTrend),
			UpdatedAt: now,
		}
	}
	return next, elapsed
}

func signalFactors(kind NeedKind, personality PersonalityRef) (float64, float64) {
	switch kind {
	case NeedKindReassurance:
		return 0.08 + personality.Sensitivity*0.1 + personality.AttachmentBias*0.08, 0.05 + personality.Sensitivity*0.04
	case NeedKindConnection:
		return 0.07 + personality.AttachmentBias*0.08, 0.045 + personality.AttachmentBias*0.03
	case NeedKindAutonomy:
		return 0.06 + personality.BoundaryStrength*0.09, 0.04 + personality.BoundaryStrength*0.04
	case NeedKindClarity:
		return 0.065 + personality.Sensitivity*0.03 + (1-personality.Stability)*0.04, 0.045 + personality.Sensitivity*0.03
	case NeedKindRest:
		return 0.055 + (1-personality.Stability)*0.08, 0.04 + (1-personality.Stability)*0.03
	case NeedKindExpression:
		return 0.06 + personality.Sensitivity*0.05 + personality.AttachmentBias*0.03, 0.04 + personality.Sensitivity*0.04
	case NeedKindNovelty:
		return 0.05 + (1-personality.Stability)*0.05 + personality.AttachmentBias*0.02, 0.035 + (1-personality.Stability)*0.035
	default:
		return 0.06, 0.04
	}
}

func normalizeKind(kind NeedKind) NeedKind {
	switch kind {
	case NeedKindReassurance, NeedKindConnection, NeedKindAutonomy, NeedKindClarity, NeedKindRest, NeedKindExpression, NeedKindNovelty:
		return kind
	default:
		return NeedKindClarity
	}
}

func orderedKinds() []NeedKind {
	return []NeedKind{
		NeedKindAutonomy,
		NeedKindClarity,
		NeedKindConnection,
		NeedKindExpression,
		NeedKindNovelty,
		NeedKindReassurance,
		NeedKindRest,
	}
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

func isNeedSaturated(level, trend float64) bool {
	if level >= 0.96 {
		return true
	}
	return level >= saturationThreshold && trend >= 0
}

func decayFactor(elapsedHours, halfLife, recoveryBoost float64) float64 {
	if elapsedHours <= 0 || halfLife <= 0 {
		return 1
	}
	return clampRange(0, 1, math.Exp(-math.Ln2*elapsedHours*recoveryBoost/halfLife))
}

func clampSigned(value, limit float64) float64 {
	if value > limit {
		return limit
	}
	if value < -limit {
		return -limit
	}
	return value
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
