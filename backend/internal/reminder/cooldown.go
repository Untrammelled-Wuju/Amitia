package reminder

import (
	"math"
	"time"
)

func ScaleCooldown(gap time.Duration, multiplier float64) time.Duration {
	value := float64(gap) * multiplier
	if math.IsNaN(value) || value <= 0 {
		return 0
	}
	if value >= float64(math.MaxInt64) {
		return time.Duration(math.MaxInt64)
	}
	return time.Duration(value)
}

func ProactiveMinimumInterval(policy *ProactivePolicy) time.Duration {
	if policy == nil || policy.ActiveLevel <= 0 {
		return 0
	}
	return ScaleCooldown(ScaleCooldown(time.Minute, float64(policy.MinInterval)), 40/float64(policy.ActiveLevel))
}
