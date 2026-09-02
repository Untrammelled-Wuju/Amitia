package workflow

import "testing"

func TestWorkflowReliabilityMetricsSnapshot(t *testing.T) {
	m := NewWorkflowReliabilityMetrics()
	m.Inc(MetricWorkflowRunTotal)
	m.Add(MetricWorkflowNodeRetryTotal, 2)
	for _, value := range []float64{10, 20, 30, 40, 50, 60, 70, 80, 90, 100} {
		m.Observe(MetricWorkflowRunLatencyMS, value)
	}
	snapshot := m.Snapshot()
	if snapshot.Version != "workflow-reliability-v1" {
		t.Fatalf("unexpected version: %s", snapshot.Version)
	}
	if snapshot.Counters[MetricWorkflowRunTotal] != 1 {
		t.Fatalf("run counter = %d", snapshot.Counters[MetricWorkflowRunTotal])
	}
	if snapshot.Counters[MetricWorkflowNodeRetryTotal] != 2 {
		t.Fatalf("retry counter = %d", snapshot.Counters[MetricWorkflowNodeRetryTotal])
	}
	latency := snapshot.Percentiles[MetricWorkflowRunLatencyMS]
	if latency.Count != 10 || latency.P50 != 50 || latency.P95 != 90 || latency.P99 != 90 {
		t.Fatalf("unexpected percentiles: %+v", latency)
	}
	if _, ok := snapshot.Counters[MetricRuntimeCrashTotal]; !ok {
		t.Fatal("runtime crash metric must always be present in the reliability schema")
	}
}
