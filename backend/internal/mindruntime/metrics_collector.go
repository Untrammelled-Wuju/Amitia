package mindruntime

import (
	"sync"
	"time"
)

type MetricKey struct {
	Scope string
	Name  string
}

type MetricSample struct {
	Value     float64
	Collected time.Time
}

type RuntimeMetricsCollector struct {
	mu       sync.RWMutex
	counters map[MetricKey]int64
	gauges   map[MetricKey]float64
	latency  map[MetricKey][]float64
	rates    map[MetricKey]struct {
		success int64
		total   int64
	}
}

type RuntimeMetricsSnapshot struct {
	TotalInteractions        int64   `json:"totalInteractions"`
	ActiveInteractions       int64   `json:"activeInteractions"`
	AvgLatencyMs             float64 `json:"avgLatencyMs"`
	P95LatencyMs             float64 `json:"p95LatencyMs"`
	CancelRate               float64 `json:"cancelRate"`
	SupersedeRate            float64 `json:"supersedeRate"`
	DeliverySuccessRate      float64 `json:"deliverySuccessRate"`
	ToolUnknownRate          float64 `json:"toolUnknownRate"`
	CircuitBreakerRate       float64 `json:"circuitBreakerRate"`
	QueueBackpressureRate    float64 `json:"queueBackpressureRate"`
	ReconciliationOpenIssues int64   `json:"reconciliationOpenIssues"`
	TotalModelCalls          int64   `json:"totalModelCalls"`
	CacheHitRate             float64 `json:"cacheHitRate"`
	CollectedAt              string  `json:"collectedAt"`
	Version                  string  `json:"version"`
}

var DefaultMetricsCollector = NewRuntimeMetricsCollector()

func NewRuntimeMetricsCollector() *RuntimeMetricsCollector {
	return &RuntimeMetricsCollector{
		counters: make(map[MetricKey]int64),
		gauges:   make(map[MetricKey]float64),
		latency:  make(map[MetricKey][]float64),
		rates: make(map[MetricKey]struct {
			success int64
			total   int64
		}),
	}
}

func (c *RuntimeMetricsCollector) IncrementCounter(scope, name string, delta int64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	key := MetricKey{Scope: scope, Name: name}
	c.counters[key] += delta
}

func (c *RuntimeMetricsCollector) SetGauge(scope, name string, value float64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	key := MetricKey{Scope: scope, Name: name}
	c.gauges[key] = value
}

func (c *RuntimeMetricsCollector) RecordLatency(scope, name string, durationMs float64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	key := MetricKey{Scope: scope, Name: name}
	c.latency[key] = append(c.latency[key], durationMs)
	maxSamples := int64(1000)
	if int64(len(c.latency[key])) > maxSamples {
		c.latency[key] = c.latency[key][len(c.latency[key])-int(maxSamples):]
	}
}

func (c *RuntimeMetricsCollector) RecordRate(scope, name string, success bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	key := MetricKey{Scope: scope, Name: name}
	entry := c.rates[key]
	entry.total++
	if success {
		entry.success++
	}
	c.rates[key] = entry
	maxSamples := int64(5000)
	if int64(entry.total) > maxSamples {
		half := entry.total / int64(2)
		entry.total = half
		entry.success = entry.success * half / entry.total
		if entry.success < 0 {
			entry.success = 0
		}
		c.rates[key] = entry
	}
}

func (c *RuntimeMetricsCollector) Snapshot() RuntimeMetricsSnapshot {
	c.mu.RLock()
	defer c.mu.RUnlock()

	s := RuntimeMetricsSnapshot{
		CollectedAt: time.Now().UTC().Format(time.RFC3339Nano),
		Version:     "runtime-metrics-v1",
	}

	for key, val := range c.counters {
		switch key.Name {
		case "total_interactions":
			s.TotalInteractions += val
		case "active_interactions":
			s.ActiveInteractions += val
		case "total_model_calls":
			s.TotalModelCalls += val
		case "reconciliation_open_issues":
			s.ReconciliationOpenIssues += val
		}
	}

	for _, samples := range c.latency {
		if len(samples) == 0 {
			continue
		}
		var sum float64
		sorted := make([]float64, len(samples))
		copy(sorted, samples)
		for _, v := range sorted {
			sum += v
		}
		s.AvgLatencyMs = sum / float64(len(sorted))
	}

	var totalCancel int64
	var totalSupersede int64
	var totalDeliveries int64
	var totalDeliveriesSuccess int64
	var totalToolUnknown int64
	var totalCircuitOpen int64
	var totalBackpressure int64
	var totalCacheHit int64
	var totalCacheTotal int64

	for key, entry := range c.rates {
		switch key.Name {
		case "cancel":
			totalCancel += entry.total
		case "supersede":
			totalSupersede += entry.total
		case "delivery":
			totalDeliveries += entry.total
			totalDeliveriesSuccess += entry.success
		case "tool_unknown":
			totalToolUnknown += entry.total
		case "circuit_open":
			totalCircuitOpen += entry.total
		case "backpressure":
			totalBackpressure += entry.total
		case "cache":
			totalCacheTotal += entry.total
			totalCacheHit += entry.success
		}
	}

	s.CancelRate = computeRate(totalCancel, s.TotalInteractions)
	s.SupersedeRate = computeRate(totalSupersede, s.TotalInteractions)
	s.DeliverySuccessRate = computeRate(totalDeliveriesSuccess, totalDeliveries)
	s.ToolUnknownRate = computeRate(totalToolUnknown, totalDeliveries)
	s.CircuitBreakerRate = computeRate(totalCircuitOpen, totalDeliveries)
	s.QueueBackpressureRate = computeRate(totalBackpressure, totalDeliveries)
	s.CacheHitRate = computeRate(totalCacheHit, totalCacheTotal)

	return s
}

func (c *RuntimeMetricsCollector) Reset() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.counters = make(map[MetricKey]int64)
	c.gauges = make(map[MetricKey]float64)
	c.latency = make(map[MetricKey][]float64)
	c.rates = make(map[MetricKey]struct {
		success int64
		total   int64
	})
}

func computeRate(numerator, denominator int64) float64 {
	if denominator == 0 {
		return 0
	}
	return float64(numerator) / float64(denominator)
}
