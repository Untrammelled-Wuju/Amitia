package relationship

import (
	"math"
	"sort"
)

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

func ComputeVelocity(current, previous float64, elapsedHours float64) float64 {
	if elapsedHours <= 0 {
		return 0
	}
	return round4((current - previous) / elapsedHours)
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

func clamp01Scale(value, minimum, maximum float64) float64 {
	if value < minimum {
		return minimum
	}
	if value > maximum {
		return maximum
	}
	return value
}
