package canary

import (
	"context"
	"math"
	"sort"
	"strings"
	"sync"
	"time"
)

type HealthCheckResult struct {
	Name      string
	Passed    bool
	Value     float64
	Threshold float64
	Detail    string
}

type HealthEvaluation struct {
	Overall     string
	Checks      []HealthCheckResult
	ShouldAbort bool
	AbortReason string
}

type HealthMetricsCollector struct {
	metrics []CanaryMetric
	mu      sync.RWMutex
}

const maxMetricsCount = 10000

func NewHealthMetricsCollector() *HealthMetricsCollector {
	return &HealthMetricsCollector{
		metrics: make([]CanaryMetric, 0),
	}
}

func (c *HealthMetricsCollector) RecordMetric(ctx context.Context, metric CanaryMetric) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.metrics = append(c.metrics, metric)
	if len(c.metrics) > maxMetricsCount {
		excess := len(c.metrics) - maxMetricsCount
		c.metrics = c.metrics[excess:]
	}
	return nil
}

func (c *HealthMetricsCollector) CollectBaseline(ctx context.Context, extensionID string, generation int64, window time.Duration) (map[MetricName]float64, error) {
	now := time.Now().UTC()
	start := now.Add(-window)
	metrics, err := c.GetMetrics(ctx, extensionID, generation, start, now)
	if err != nil {
		return nil, err
	}
	return c.AggregateMetrics(ctx, metrics), nil
}

func (c *HealthMetricsCollector) GetMetrics(ctx context.Context, extensionID string, generation int64, windowStart, windowEnd time.Time) ([]CanaryMetric, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	var out []CanaryMetric
	for _, m := range c.metrics {
		if m.ExtensionID == extensionID && m.Generation == generation &&
			!m.WindowStart.Before(windowStart) && !m.WindowEnd.After(windowEnd) {
			out = append(out, m)
		}
	}
	return out, nil
}

func (c *HealthMetricsCollector) AggregateMetrics(ctx context.Context, metrics []CanaryMetric) map[MetricName]float64 {
	result := make(map[MetricName]float64)
	rateSums := make(map[MetricName]float64)
	rateCounts := make(map[MetricName]int)
	countSums := make(map[MetricName]float64)
	var latencies []float64

	for _, m := range metrics {
		nameStr := string(m.MetricName)
		switch {
		case m.MetricName == MetricP50Latency || m.MetricName == MetricP95Latency || m.MetricName == MetricP99Latency:
			latencies = append(latencies, m.MetricValue)
		case strings.Contains(nameStr, "rate") || strings.Contains(nameStr, "success"):
			rateSums[m.MetricName] += m.MetricValue
			rateCounts[m.MetricName]++
		default:
			countSums[m.MetricName] += m.MetricValue
		}
	}

	for name, sum := range rateSums {
		if rateCounts[name] > 0 {
			result[name] = sum / float64(rateCounts[name])
		}
	}

	for name, sum := range countSums {
		result[name] = sum
	}

	if len(latencies) > 0 {
		sort.Float64s(latencies)
		result[MetricP50Latency] = percentile(latencies, 50)
		result[MetricP95Latency] = percentile(latencies, 95)
		result[MetricP99Latency] = percentile(latencies, 99)
	}

	return result
}

func percentile(sorted []float64, p float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	idx := int(math.Ceil(p/100*float64(len(sorted)))) - 1
	if idx < 0 {
		idx = 0
	}
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return sorted[idx]
}

type HealthEvaluator struct{}

func NewHealthEvaluator() *HealthEvaluator {
	return &HealthEvaluator{}
}

func (e *HealthEvaluator) Evaluate(ctx context.Context, policy *CanaryHealthPolicy, current map[MetricName]float64, baseline map[MetricName]float64) HealthEvaluation {
	eval := HealthEvaluation{
		Overall: "healthy",
		Checks:  make([]HealthCheckResult, 0),
	}

	if policy == nil {
		return eval
	}

	required := policy.RequiredHealthChecks

	if isCheckRequired(required, "error_rate_absolute") {
		if errRate, ok := current[MetricErrorRate]; ok {
			passed := errRate <= policy.MaximumErrorRate
			e.addCheck(&eval, "error_rate_absolute", passed, errRate, policy.MaximumErrorRate, "error_rate_exceeded_absolute")
		}
	}

	if isCheckRequired(required, "error_rate_relative") {
		if errRate, ok := current[MetricErrorRate]; ok {
			if baseErr, ok := baseline[MetricErrorRate]; ok && baseErr > 0 {
				relativeThreshold := baseErr * (1 + policy.MaximumRelativeErrorRate)
				relPassed := errRate <= relativeThreshold
				e.addCheck(&eval, "error_rate_relative", relPassed, errRate, relativeThreshold, "error_rate_exceeded_relative")
			}
		}
	}

	if isCheckRequired(required, "p95_latency_absolute") {
		if p95, ok := current[MetricP95Latency]; ok && policy.MaximumP95Latency > 0 {
			thresholdMs := float64(policy.MaximumP95Latency / time.Millisecond)
			passed := p95 <= thresholdMs
			e.addCheck(&eval, "p95_latency_absolute", passed, p95, thresholdMs, "p95_latency_exceeded")
		}
	}

	if isCheckRequired(required, "p95_latency_regression") {
		if p95, ok := current[MetricP95Latency]; ok {
			if baseP95, ok := baseline[MetricP95Latency]; ok && baseP95 > 0 {
				relThreshold := baseP95 * (1 + policy.MaximumLatencyRegression)
				relPassed := p95 <= relThreshold
				e.addCheck(&eval, "p95_latency_regression", relPassed, p95, relThreshold, "p95_latency_regression_exceeded")
			}
		}
	}

	if isCheckRequired(required, "crash_count") {
		if crash, ok := current[MetricRuntimeCrash]; ok && policy.MaximumCrashCount > 0 {
			threshold := float64(policy.MaximumCrashCount)
			passed := crash <= threshold
			e.addCheck(&eval, "crash_count", passed, crash, threshold, "crash_count_exceeded")
		}
	}

	if isCheckRequired(required, "timeout_rate") {
		if timeout, ok := current[MetricTimeout]; ok && policy.MaximumTimeoutRate > 0 {
			passed := timeout <= policy.MaximumTimeoutRate
			e.addCheck(&eval, "timeout_rate", passed, timeout, policy.MaximumTimeoutRate, "timeout_rate_exceeded")
		}
	}

	if isCheckRequired(required, "invalid_result_rate") {
		if invalid, ok := current[MetricInvalidResult]; ok && policy.MaximumInvalidResultRate > 0 {
			passed := invalid <= policy.MaximumInvalidResultRate
			e.addCheck(&eval, "invalid_result_rate", passed, invalid, policy.MaximumInvalidResultRate, "invalid_result_rate_exceeded")
		}
	}

	if eval.ShouldAbort {
		eval.Overall = "unhealthy"
	}

	return eval
}

func (e *HealthEvaluator) addCheck(eval *HealthEvaluation, name string, passed bool, value, threshold float64, abortReason string) {
	eval.Checks = append(eval.Checks, HealthCheckResult{
		Name:      name,
		Passed:    passed,
		Value:     value,
		Threshold: threshold,
	})
	if !passed {
		eval.ShouldAbort = true
		if eval.AbortReason == "" {
			eval.AbortReason = abortReason
		}
	}
}

func isCheckRequired(required []string, name string) bool {
	if len(required) == 0 {
		return true
	}
	for _, r := range required {
		if r == name {
			return true
		}
	}
	return false
}
