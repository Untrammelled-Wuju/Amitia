package mindruntime

import (
	"testing"
	"time"
)

func TestFilterCausalChain_filtersByKind(t *testing.T) {
	events := completeCausalEvents()
	filter := CausalChainFilter{Kinds: []TraceEventKind{TraceEventCancel, TraceEventSuperseded}}
	result := FilterCausalChain(events, filter)
	if result.Total != 17 {
		t.Fatalf("expected 17 total events, got %d", result.Total)
	}
	if result.Filtered != 2 {
		t.Fatalf("expected 2 filtered events, got %d", result.Filtered)
	}
	for _, e := range result.Events {
		if e.Kind != TraceEventCancel && e.Kind != TraceEventSuperseded {
			t.Fatalf("unexpected event kind %s in filter result", e.Kind)
		}
	}
}

func TestFilterCausalChain_filtersByStatus(t *testing.T) {
	events := completeCausalEvents()
	filter := CausalChainFilter{Status: "completed"}
	result := FilterCausalChain(events, filter)
	if result.Filtered < 1 {
		t.Fatal("expected at least one completed event")
	}
	for _, e := range result.Events {
		if e.Status != "completed" {
			t.Fatalf("unexpected status %s", e.Status)
		}
	}
}

func TestFilterCausalChain_filtersByReason(t *testing.T) {
	events := completeCausalEvents()
	filter := CausalChainFilter{Reason: "hello"}
	result := FilterCausalChain(events, filter)
	if result.Filtered > 0 {
		t.Fatalf("expected no hello reason events, got %d", result.Filtered)
	}
}

func TestFilterCausalChain_emptyEventsReturnsZero(t *testing.T) {
	result := FilterCausalChain(nil, CausalChainFilter{})
	if result.Total != 0 || result.Filtered != 0 || result.Events != nil {
		t.Fatal("expected empty result for nil events")
	}
}

func TestFilterCausalChain_byScope(t *testing.T) {
	events := completeCausalEvents()
	filter := CausalChainFilter{Scope: "char-1"}
	result := FilterCausalChain(events, filter)
	if result.Filtered == 0 {
		t.Fatal("expected at least one scoped event")
	}
	filter = CausalChainFilter{Scope: "nonexistent"}
	result = FilterCausalChain(events, filter)
	if result.Filtered > 0 {
		t.Fatal("expected no events for nonexistent scope")
	}
}

func TestAggregateRuntimeMetrics_groupsAndAggregates(t *testing.T) {
	metrics := []RuntimeMetric{
		{Name: RuntimeMetricLatencyMillis, Value: 100},
		{Name: RuntimeMetricLatencyMillis, Value: 200},
		{Name: RuntimeMetricLatencyMillis, Value: 300},
		{Name: RuntimeMetricModelCallCount, Value: 2},
		{Name: RuntimeMetricModelCallCount, Value: 4},
	}
	agg := AggregateRuntimeMetrics(metrics)
	if len(agg) != 2 {
		t.Fatalf("expected 2 aggregations, got %d", len(agg))
	}
	for _, a := range agg {
		switch a.Name {
		case RuntimeMetricLatencyMillis:
			if a.Count != 3 || a.Min != 100 || a.Max != 300 || a.Sum != 600 {
				t.Fatalf("unexpected latency agg: %+v", a)
			}
		case RuntimeMetricModelCallCount:
			if a.Count != 2 || a.Min != 2 || a.Max != 4 || a.Sum != 6 {
				t.Fatalf("unexpected model call agg: %+v", a)
			}
		}
	}
}

func TestAggregateRuntimeMetrics_nilReturnsNil(t *testing.T) {
	if agg := AggregateRuntimeMetrics(nil); agg != nil {
		t.Fatal("expected nil for nil metrics")
	}
}

func TestCausalChainSummary_countsByKindAndStatus(t *testing.T) {
	events := completeCausalEvents()
	summary := BuildCausalChainSummary(events, 300*time.Millisecond)
	if summary.TotalEvents != 17 {
		t.Fatalf("expected 17 events, got %d", summary.TotalEvents)
	}
	if summary.ByKind[TraceEventRequest] != 1 {
		t.Fatalf("expected 1 request event, got %d", summary.ByKind[TraceEventRequest])
	}
	if summary.CancelledCount != 1 || summary.SupersededCount != 1 {
		t.Fatalf("expected 1 cancelled and 1 superseded, got c=%d s=%d", summary.CancelledCount, summary.SupersededCount)
	}
	if summary.ToolCalls != 1 || summary.Deliveries != 1 {
		t.Fatalf("expected 1 tool and 1 delivery, got t=%d d=%d", summary.ToolCalls, summary.Deliveries)
	}
	if summary.Compensations != 1 {
		t.Fatalf("expected 1 compensation, got %d", summary.Compensations)
	}
	if summary.Validations != 1 {
		t.Fatalf("expected 1 validation, got %d", summary.Validations)
	}
	if summary.Duration != 300*time.Millisecond {
		t.Fatalf("unexpected duration %v", summary.Duration)
	}
}

func TestCausalChainSummary_emptyEvents(t *testing.T) {
	summary := BuildCausalChainSummary(nil, 0)
	if summary.TotalEvents != 0 || len(summary.ByKind) != 0 {
		t.Fatal("expected empty summary")
	}
}

func TestQueryCausalChainByStatus(t *testing.T) {
	events := completeCausalEvents()
	query := RuntimeExtendedQuery{Status: "completed"}
	result := QueryCausalChain(events, query)
	if result.Filtered < 1 {
		t.Fatal("expected at least one completed event")
	}
}

func TestQueryCausalChainByKind(t *testing.T) {
	events := completeCausalEvents()
	query := RuntimeExtendedQuery{Kind: TraceEventRequest}
	result := QueryCausalChain(events, query)
	if result.Filtered != 1 {
		t.Fatalf("expected 1 request event, got %d", result.Filtered)
	}
}

func TestQueryCausalChainByIDs(t *testing.T) {
	events := completeCausalEvents()
	query := RuntimeExtendedQuery{RequestID: "req-1"}
	result := QueryCausalChainByIDs(events, query)
	if result.Filtered < 1 {
		t.Fatal("expected at least one event with request id")
	}
}

func TestFilterByCharacter(t *testing.T) {
	events := completeCausalEvents()
	result := FilterByCharacter(events, "char-1")
	if len(result) == 0 {
		t.Fatalf("expected events for char-1, got %d", len(result))
	}
}

func TestFilterByCharacterEmpty(t *testing.T) {
	result := FilterByCharacter(nil, "char-1")
	if len(result) != 0 {
		t.Fatal("expected no events for nil input")
	}
}

func TestFilterByDelivery(t *testing.T) {
	events := completeCausalEvents()
	result := FilterByDelivery(events, "delivery-1")
	if len(result) != 1 {
		t.Fatalf("expected 1 delivery event, got %d", len(result))
	}
}

func TestFilterByTool(t *testing.T) {
	events := completeCausalEvents()
	result := FilterByTool(events, "tool-1")
	if len(result) != 1 {
		t.Fatalf("expected 1 tool event, got %d", len(result))
	}
}

func completeCausalEvents() []RuntimeCausalEvent {
	snap := BuildRuntimeSnapshot(completeSnapshotInput())
	report := BuildRuntimeObservabilityReport(RuntimeObservabilityInput{
		Snapshot:             snap,
		RequestID:            "req-1",
		DeliveryID:           "delivery-1",
		ToolID:               "tool-1",
		Path:                 "standard",
		Priority:             "P1",
		Scope:                "char-1",
		InteractionStatus:    "completed",
		QueueDuration:        25 * time.Millisecond,
		TotalDuration:        120 * time.Millisecond,
		CancellationReason:   "deadline_exceeded",
		SupersededBy:         "interaction-2",
		ToolStatus:           "ok",
		DeliveryStatus:       "sent",
		DeliveryIntent:       "dispatch",
		OutboxStatus:         "queued",
		LeaseStatus:          "held",
		CircuitBreakerStatus: "closed",
		ContextVersion:       1,
		BudgetUsed:           0.5,
		BudgetLimit:          1.0,
		CandidateCount:       3,
		ValidationResult:     "ok",
		CompensationEvent:    "rollback-needed",
	})
	return report.CausalChain
}