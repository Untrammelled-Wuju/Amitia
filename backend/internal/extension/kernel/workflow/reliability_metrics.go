package workflow

import (
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

const (
	MetricWorkflowTriggerTotal           = "workflow_trigger_total"
	MetricWorkflowTriggerDuplicateTotal  = "workflow_trigger_duplicate_total"
	MetricWorkflowRunTotal               = "workflow_run_total"
	MetricWorkflowRunSuccessTotal        = "workflow_run_success_total"
	MetricWorkflowRunFailedTotal         = "workflow_run_failed_total"
	MetricWorkflowNodeRetryTotal         = "workflow_node_retry_total"
	MetricWorkflowNodeTimeoutTotal       = "workflow_node_timeout_total"
	MetricWorkflowCheckpointResumeTotal  = "workflow_checkpoint_resume_total"
	MetricWorkflowCompensationTotal      = "workflow_compensation_total"
	MetricWorkflowDeviceWaitTotal        = "workflow_device_wait_total"
	MetricWorkflowDeviceReconnectTotal   = "workflow_device_reconnect_total"
	MetricRuntimeCrashTotal              = "runtime_crash_total"
	MetricRuntimeRecoveryTotal           = "runtime_recovery_total"
	MetricRuntimeRecoveryExhaustedTotal  = "runtime_recovery_exhausted_total"
	MetricAndroidAccessibilityDisconnect = "android_accessibility_disconnect_total"
	MetricUIAgentActionTotal             = "ui_agent_action_total"
	MetricUIAgentFallbackTotal           = "ui_agent_fallback_total"
	MetricUIAgentNoEffectTotal           = "ui_agent_no_effect_total"
	MetricUIAgentLoopTotal               = "ui_agent_loop_total"
	MetricWakeDetectionTotal             = "wake_detection_total"
	MetricWakeFalseOrRejectedTotal       = "wake_false_or_rejected_total"
	MetricWorkflowRunLatencyMS           = "workflow_run_latency_ms"
	MetricUIAgentActionLatencyMS         = "ui_agent_action_latency_ms"
	MetricWakeDetectionLatencyMS         = "wake_detection_latency_ms"
	reliabilityLatencySampleLimit        = 2048
)

var reliabilityCounterNames = []string{
	MetricWorkflowTriggerTotal,
	MetricWorkflowTriggerDuplicateTotal,
	MetricWorkflowRunTotal,
	MetricWorkflowRunSuccessTotal,
	MetricWorkflowRunFailedTotal,
	MetricWorkflowNodeRetryTotal,
	MetricWorkflowNodeTimeoutTotal,
	MetricWorkflowCheckpointResumeTotal,
	MetricWorkflowCompensationTotal,
	MetricWorkflowDeviceWaitTotal,
	MetricWorkflowDeviceReconnectTotal,
	MetricRuntimeCrashTotal,
	MetricRuntimeRecoveryTotal,
	MetricRuntimeRecoveryExhaustedTotal,
	MetricAndroidAccessibilityDisconnect,
	MetricUIAgentActionTotal,
	MetricUIAgentFallbackTotal,
	MetricUIAgentNoEffectTotal,
	MetricUIAgentLoopTotal,
	MetricWakeDetectionTotal,
	MetricWakeFalseOrRejectedTotal,
}

type ReliabilityPercentiles struct {
	Count int64   `json:"count"`
	P50   float64 `json:"p50"`
	P95   float64 `json:"p95"`
	P99   float64 `json:"p99"`
}

type WorkflowReliabilityMetricsSnapshot struct {
	Version      string                            `json:"version"`
	CollectedAt  time.Time                         `json:"collectedAt"`
	Counters     map[string]int64                  `json:"counters"`
	Percentiles  map[string]ReliabilityPercentiles `json:"percentiles"`
	SampleWindow int                               `json:"sampleWindow"`
}

type reliabilityDistribution struct {
	mu      sync.Mutex
	samples []float64
	next    int
	count   int64
}

func (d *reliabilityDistribution) observe(value float64) {
	if value < 0 {
		return
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	d.count++
	if len(d.samples) < reliabilityLatencySampleLimit {
		d.samples = append(d.samples, value)
		return
	}
	d.samples[d.next] = value
	d.next = (d.next + 1) % reliabilityLatencySampleLimit
}

func (d *reliabilityDistribution) snapshot() ReliabilityPercentiles {
	d.mu.Lock()
	values := append([]float64(nil), d.samples...)
	count := d.count
	d.mu.Unlock()
	if len(values) == 0 {
		return ReliabilityPercentiles{Count: count}
	}
	sort.Float64s(values)
	return ReliabilityPercentiles{
		Count: count,
		P50:   reliabilityPercentile(values, 0.50),
		P95:   reliabilityPercentile(values, 0.95),
		P99:   reliabilityPercentile(values, 0.99),
	}
}

func reliabilityPercentile(values []float64, quantile float64) float64 {
	if len(values) == 0 {
		return 0
	}
	if quantile <= 0 {
		return values[0]
	}
	if quantile >= 1 {
		return values[len(values)-1]
	}
	index := int(float64(len(values)-1) * quantile)
	return values[index]
}

type WorkflowReliabilityMetrics struct {
	counters      sync.Map
	distributions sync.Map
}

var DefaultWorkflowReliabilityMetrics = NewWorkflowReliabilityMetrics()

func NewWorkflowReliabilityMetrics() *WorkflowReliabilityMetrics {
	metrics := &WorkflowReliabilityMetrics{}
	for _, name := range reliabilityCounterNames {
		metrics.counters.Store(name, &atomic.Int64{})
	}
	return metrics
}

func (m *WorkflowReliabilityMetrics) Inc(name string) {
	m.Add(name, 1)
}

func (m *WorkflowReliabilityMetrics) Add(name string, delta int64) {
	if m == nil || name == "" || delta == 0 {
		return
	}
	value, _ := m.counters.LoadOrStore(name, &atomic.Int64{})
	value.(*atomic.Int64).Add(delta)
}

func (m *WorkflowReliabilityMetrics) Observe(name string, value float64) {
	if m == nil || name == "" {
		return
	}
	distribution, _ := m.distributions.LoadOrStore(name, &reliabilityDistribution{})
	distribution.(*reliabilityDistribution).observe(value)
}

func (m *WorkflowReliabilityMetrics) Snapshot() WorkflowReliabilityMetricsSnapshot {
	result := WorkflowReliabilityMetricsSnapshot{
		Version:      "workflow-reliability-v1",
		CollectedAt:  time.Now().UTC(),
		Counters:     make(map[string]int64, len(reliabilityCounterNames)),
		Percentiles:  make(map[string]ReliabilityPercentiles),
		SampleWindow: reliabilityLatencySampleLimit,
	}
	if m == nil {
		return result
	}
	m.counters.Range(func(key, value any) bool {
		name, ok := key.(string)
		counter, valid := value.(*atomic.Int64)
		if ok && valid {
			result.Counters[name] = counter.Load()
		}
		return true
	})
	m.distributions.Range(func(key, value any) bool {
		name, ok := key.(string)
		distribution, valid := value.(*reliabilityDistribution)
		if ok && valid {
			result.Percentiles[name] = distribution.snapshot()
		}
		return true
	})
	return result
}
