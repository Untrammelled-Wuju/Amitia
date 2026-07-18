package temporal

import (
	"math"
	"sort"
	"time"
)

const (
	minimumExpectedGap = 2 * time.Hour
	maximumExpectedGap = 14 * 24 * time.Hour
	minimumGapMAD      = time.Hour
)

type cadenceEstimate struct {
	ExpectedGap time.Duration
	MAD         time.Duration
	SampleCount int
}

func estimateCadence(samples []CadenceSample) cadenceEstimate {
	values := make([]float64, 0, len(samples))
	for _, sample := range samples {
		if sample.Included && sample.GapSeconds >= SessionBreakThreshold.Seconds() && !math.IsNaN(sample.GapSeconds) && !math.IsInf(sample.GapSeconds, 0) {
			values = append(values, sample.GapSeconds)
		}
	}
	if len(values) < 5 {
		return cadenceEstimate{ExpectedGap: DefaultExpectedGap, MAD: minimumGapMAD, SampleCount: len(values)}
	}
	center := median(values)
	deviations := make([]float64, len(values))
	for index, value := range values {
		deviations[index] = math.Abs(value - center)
	}
	mad := median(deviations) * 1.4826
	center = clampFloat(center, minimumExpectedGap.Seconds(), maximumExpectedGap.Seconds())
	mad = math.Max(mad, minimumGapMAD.Seconds())
	return cadenceEstimate{ExpectedGap: time.Duration(center * float64(time.Second)), MAD: time.Duration(mad * float64(time.Second)), SampleCount: len(values)}
}

func selectCadence(relationshipSamples, globalSamples []CadenceSample) cadenceEstimate {
	relationship := estimateCadence(relationshipSamples)
	if relationship.SampleCount >= 5 {
		return relationship
	}
	global := estimateCadence(globalSamples)
	if global.SampleCount >= 5 {
		return global
	}
	return cadenceEstimate{ExpectedGap: DefaultExpectedGap, MAD: minimumGapMAD, SampleCount: relationship.SampleCount}
}

func median(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	ordered := append([]float64(nil), values...)
	sort.Float64s(ordered)
	middle := len(ordered) / 2
	if len(ordered)%2 == 1 {
		return ordered[middle]
	}
	return (ordered[middle-1] + ordered[middle]) / 2
}

func gapMetrics(now, previous time.Time, cadence cadenceEstimate) (float64, float64, float64, []string) {
	if previous.IsZero() {
		return 0, 0, 0, nil
	}
	gap := now.Sub(previous).Seconds()
	diagnostics := []string{}
	if gap < 0 {
		gap = 0
		diagnostics = append(diagnostics, "future_previous_timestamp", "clock_rollback_clamped")
	}
	maximumGap := (100 * 365 * 24 * time.Hour).Seconds()
	if gap > maximumGap || math.IsInf(gap, 0) || math.IsNaN(gap) {
		gap = maximumGap
		diagnostics = append(diagnostics, "gap_clamped")
	}
	expected := cadence.ExpectedGap.Seconds()
	if expected <= 0 {
		expected = DefaultExpectedGap.Seconds()
	}
	mad := math.Max(cadence.MAD.Seconds(), minimumGapMAD.Seconds())
	normalized := gap / expected
	deviation := (gap - expected) / math.Max(mad, math.Max(expected*0.35, minimumGapMAD.Seconds()))
	return roundFinite(gap), roundFinite(normalized), roundFinite(deviation), diagnostics
}

func clampFloat(value, minimum, maximum float64) float64 {
	if value < minimum {
		return minimum
	}
	if value > maximum {
		return maximum
	}
	return value
}

func roundFinite(value float64) float64 {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return 0
	}
	return math.Round(value*1000) / 1000
}
