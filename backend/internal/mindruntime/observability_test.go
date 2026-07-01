package mindruntime

import (
	"testing"
	"time"
)

func TestBuildRuntimeObservabilityReportBuildsCausalChain(t *testing.T) {
	snapshot := BuildRuntimeSnapshot(completeSnapshotInput())

	report := BuildRuntimeObservabilityReport(RuntimeObservabilityInput{
		Snapshot:             snapshot,
		RequestID:            "req-1",
		EventID:              "event-1",
		DeliveryID:           "delivery-1",
		ToolID:               "tool-1",
		Path:                 "standard",
		Priority:             "P1",
		InteractionStatus:    "completed",
		QueueDuration:        25 * time.Millisecond,
		TotalDuration:        120 * time.Millisecond,
		CancellationReason:   "deadline_exceeded",
		SupersededBy:         "interaction-2",
		ToolStatus:           "ok",
		DeliveryStatus:       "sent",
		OutboxStatus:         "queued",
		LeaseStatus:          "held",
		CircuitBreakerStatus: "closed",
	})

	if report.SnapshotID != snapshot.ID {
		t.Fatalf("unexpected snapshot id: %s", report.SnapshotID)
	}
	if len(report.CausalChain) != 15 {
		t.Fatalf("unexpected causal chain length: %d", len(report.CausalChain))
	}
	if report.CausalChain[0].Kind != TraceEventRequest {
		t.Fatalf("unexpected first event: %#v", report.CausalChain[0])
	}
	if report.CausalChain[1].Kind != TraceEventInteraction {
		t.Fatalf("unexpected second event: %#v", report.CausalChain[1])
	}
	if report.CausalChain[2].Kind != TraceEventFrame || report.CausalChain[2].Stage != TraceStagePersonality {
		t.Fatalf("unexpected first frame event: %#v", report.CausalChain[2])
	}
	if report.CausalChain[8].Kind != TraceEventTool {
		t.Fatalf("unexpected tool event: %#v", report.CausalChain[8])
	}
	if report.CausalChain[14].Kind != TraceEventCircuit {
		t.Fatalf("unexpected circuit event: %#v", report.CausalChain[14])
	}
	for i, event := range report.CausalChain {
		if event.Index != i+1 {
			t.Fatalf("unexpected event index at %d: %#v", i, event)
		}
	}
}

func TestBuildRuntimeObservabilityReportBuildsMetrics(t *testing.T) {
	input := completeSnapshotInput()
	input.AppraisalRef = RuntimeReference{}
	snapshot := BuildRuntimeSnapshot(input)

	report := BuildRuntimeObservabilityReport(RuntimeObservabilityInput{
		Snapshot:          snapshot,
		QueueDuration:     45 * time.Millisecond,
		TotalDuration:     210 * time.Millisecond,
		DeliveryStatus:    "unknown",
		LeaseStatus:       "collision",
		ModelCallCount:    4,
		QueueDepth:        7,
		ConflictCount:     2,
		BudgetRejected:    true,
		DegradationReason: "budget_exhausted",
		ConsistencyDiffs:  3,
	})

	assertMetricValue(t, report.Metrics, RuntimeMetricLatencyMillis, 210)
	assertMetricValue(t, report.Metrics, RuntimeMetricQueueMillis, 45)
	assertMetricValue(t, report.Metrics, RuntimeMetricTraceFrameCount, 6)
	assertMetricValue(t, report.Metrics, RuntimeMetricDiagnosticCount, 1)
	assertMetricValue(t, report.Metrics, RuntimeMetricModelCallCount, 4)
	assertMetricValue(t, report.Metrics, RuntimeMetricQueueDepth, 7)
	assertMetricValue(t, report.Metrics, RuntimeMetricConflictCount, 2)
	assertMetricValue(t, report.Metrics, RuntimeMetricBudgetRejected, 1)
	assertMetricValue(t, report.Metrics, RuntimeMetricDegraded, 1)
	assertMetricValue(t, report.Metrics, RuntimeMetricLeaseCollision, 1)
	assertMetricValue(t, report.Metrics, RuntimeMetricUnknownDelivery, 1)
	assertMetricValue(t, report.Metrics, RuntimeMetricConsistencyDiffs, 3)
}

func TestBuildRuntimeObservabilityReportOnlyUsesSnapshotDiagnostics(t *testing.T) {
	snapshot := BuildRuntimeSnapshot(completeSnapshotInput())

	report := BuildRuntimeObservabilityReport(RuntimeObservabilityInput{
		Snapshot:          snapshot,
		RequestID:         "req-2",
		Path:              "fast",
		InteractionStatus: "cancelled",
	})

	if len(report.Diagnostics) != len(snapshot.Diagnostics) {
		t.Fatalf("unexpected diagnostics copy length: %d", len(report.Diagnostics))
	}
	if report.RequestID != "req-2" || report.Path != "fast" || report.InteractionStatus != "cancelled" {
		t.Fatalf("unexpected report fields: %#v", report)
	}
}

func assertMetricValue(t *testing.T, metrics []RuntimeMetric, name RuntimeMetricName, want int64) {
	t.Helper()
	for _, metric := range metrics {
		if metric.Name == name {
			if metric.Value != want {
				t.Fatalf("unexpected metric %s value: %d", name, metric.Value)
			}
			return
		}
	}
	t.Fatalf("missing metric %s", name)
}
