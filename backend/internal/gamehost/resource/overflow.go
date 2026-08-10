package resource

import "math"

func addInt64OrCap(a, b int64) (int64, bool) {
	if b > 0 && a > math.MaxInt64-b {
		return math.MaxInt64, true
	}
	if b < 0 && a < math.MinInt64-b {
		return math.MinInt64, true
	}
	return a + b, false
}

func safeLte(used, requested, limit int64) bool {
	if limit < 0 {
		return false
	}
	if requested < 0 {
		return false
	}
	sum, overflow := addInt64OrCap(used, requested)
	if overflow {
		return false
	}
	return sum <= limit
}
