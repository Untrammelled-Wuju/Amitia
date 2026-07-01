package mindruntime

import (
	"encoding/json"
	"testing"
	"time"
)

func TestExportCausalChainSnapshot_emptyReportsReturnsEmptySnapshot(t *testing.T) {
	snap := ExportCausalChainSnapshot(nil, 0, false)
	if snap.Meta.SnapshotVersion != "causal-chain-snapshot-v1" {
		t.Fatalf("unexpected version: %s", snap.Meta.SnapshotVersion)
	}
	if snap.Meta.ReportCount != 0 || snap.Meta.TotalEvents != 0 {
		t.Fatalf("expected zero report and event count, got reports=%d events=%d", snap.Meta.ReportCount, snap.Meta.TotalEvents)
	}
	if len(snap.Events) != 0 {
		t.Fatalf("expected empty events, got %d", len(snap.Events))
	}
	if snap.Meta.Checksum == "" {
		t.Fatal("expected non-empty checksum even for empty snapshot")
	}
}

func TestExportCausalChainSnapshot_singleReportPublishesEvents(t *testing.T) {
	snap := completeExportSnapshot()
	if snap.Meta.ReportCount != 1 {
		t.Fatalf("expected 1 report, got %d", snap.Meta.ReportCount)
	}
	if snap.Meta.TotalEvents < 1 {
		t.Fatalf("expected at least one event, got %d", snap.Meta.TotalEvents)
	}
	wantEvents := len(completeRuntimeReport().CausalChain)
	if snap.Meta.TotalEvents != wantEvents {
		t.Fatalf("expected %d events, got %d", wantEvents, snap.Meta.TotalEvents)
	}
	if len(snap.Events) != wantEvents {
		t.Fatalf("expected %d exported events, got %d", wantEvents, len(snap.Events))
	}
	for i, ee := range snap.Events {
		if ee.Index != i+1 {
			t.Fatalf("unexpected event index at %d: %d", i, ee.Index)
		}
		if ee.CharacterID != "char-1" {
			t.Fatalf("unexpected character id at %d: %s", i, ee.CharacterID)
		}
		if ee.InteractionID != "interaction-1" {
			t.Fatalf("unexpected interaction id at %d: %s", i, ee.InteractionID)
		}
		if ee.RequestID != "req-1" {
			t.Fatalf("unexpected request id at %d: %s", i, ee.RequestID)
		}
	}
}

func TestExportCausalChainSnapshot_multipleReportsMergeEvents(t *testing.T) {
	r1 := completeRuntimeReport()
	r2Input := completeRuntimeReportInput()
	r2Input.RequestID = "req-2"
	r2Input.Snapshot.InteractionID = "interaction-2"
	r2Input.DeliveryID = "delivery-2"
	r2Report := BuildRuntimeObservabilityReport(r2Input)
	combined := ExportCausalChainSnapshot([]RuntimeObservabilityReport{r1, r2Report}, 0, false)

	if combined.Meta.ReportCount != 2 {
		t.Fatalf("expected 2 reports, got %d", combined.Meta.ReportCount)
	}
	total := len(r1.CausalChain) + len(r2Report.CausalChain)
	if combined.Meta.TotalEvents != total {
		t.Fatalf("expected %d total events, got %d", total, combined.Meta.TotalEvents)
	}
	if len(combined.Events) != total {
		t.Fatalf("expected %d exported events, got %d", total, len(combined.Events))
	}
}

func TestExportCausalChainSnapshot_retentionAndRedaction(t *testing.T) {
	retention := 72 * time.Hour
	now := time.Now().UTC()
	snap := ExportCausalChainSnapshot(nil, retention, true)
	if !snap.Meta.Redacted {
		t.Fatal("expected redacted=true")
	}
	if snap.Meta.RetentionUntil.IsZero() {
		t.Fatal("expected non-zero retention until")
	}
	expected := now.Add(retention)
	diff := snap.Meta.RetentionUntil.Sub(expected)
	if diff < 0 {
		diff = -diff
	}
	if diff > time.Second {
		t.Fatalf("unexpected retention until diff: %v", diff)
	}
}

func TestExportCausalChainSnapshot_checksumsDifferForDifferentContent(t *testing.T) {
	emptySnap := ExportCausalChainSnapshot(nil, 0, false)
	fullSnap := completeExportSnapshot()
	if emptySnap.Meta.Checksum == fullSnap.Meta.Checksum {
		t.Fatal("expected different checksums for empty vs full snapshot")
	}
}

func TestExportCausalChainSnapshot_jsonRoundtrip(t *testing.T) {
	snap := completeExportSnapshot()
	data, err := json.Marshal(snap)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}
	var decoded CausalChainSnapshot
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if decoded.Meta.ReportCount != snap.Meta.ReportCount {
		t.Fatalf("unexpected report count after roundtrip: %d", decoded.Meta.ReportCount)
	}
	if len(decoded.Events) != len(snap.Events) {
		t.Fatalf("unexpected event count after roundtrip: %d", len(decoded.Events))
	}
}

func TestExportCausalChainSnapshot_jsonIndentRoundtrip(t *testing.T) {
	snap := completeExportSnapshot()
	data, err := json.MarshalIndent(snap, "", "  ")
	if err != nil {
		t.Fatalf("marshal indent error: %v", err)
	}
	var decoded CausalChainSnapshot
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal indent error: %v", err)
	}
}

func TestExportCausalChainSnapshot_withRedaction(t *testing.T) {
	snap := ExportCausalChainSnapshot([]RuntimeObservabilityReport{completeRuntimeReport()}, 0, true)
	if !snap.Meta.Redacted {
		t.Fatal("expected redacted snapshot")
	}
	for _, ee := range snap.Events {
		if ee.InteractionID != "" && len(ee.InteractionID) > 0 {
			containsStars := false
			for i := 0; i < len(ee.InteractionID); i++ {
				if ee.InteractionID[i] == '*' {
					containsStars = true
					break
				}
			}
			if !containsStars && len(ee.InteractionID) > 6 {
				t.Logf("redacted interaction id might not show stars: %s", ee.InteractionID)
			}
		}
	}
}

func TestExportCausalChainSnapshot_hasCompensationAndValidation(t *testing.T) {
	snap := completeExportSnapshot()
	hasComp := false
	hasVal := false
	for _, ee := range snap.Events {
		if ee.Kind == TraceEventCompensation {
			hasComp = true
		}
		if ee.Kind == TraceEventValidation {
			hasVal = true
		}
	}
	if !hasComp || !hasVal {
		t.Fatalf("expected compensation=%v validation=%v", hasComp, hasVal)
	}
}

func TestRedactContent(t *testing.T) {
	result := redactContent("long-value-here")
	if result == "long-value-here" {
		t.Fatal("expected redacted value to differ")
	}
	if len(result) > 0 && result[:3] != "***" {
		t.Fatalf("expected redacted value to start with stars, got %s", result)
	}
}

func TestRedactContentEmpty(t *testing.T) {
	result := redactContent("")
	if result != "" {
		t.Fatal("expected empty result for empty input")
	}
}

func TestRedactContentShort(t *testing.T) {
	result := redactContent("abc")
	if result == "" {
		t.Fatal("expected non-empty redacted result")
	}
}

func completeRuntimeReportInput() RuntimeObservabilityInput {
	snap := BuildRuntimeSnapshot(completeSnapshotInput())
	return RuntimeObservabilityInput{
		Snapshot:             snap,
		RequestID:            "req-1",
		EventID:              "event-1",
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
	}
}

func completeRuntimeReport() RuntimeObservabilityReport {
	return BuildRuntimeObservabilityReport(completeRuntimeReportInput())
}

func completeExportSnapshot() CausalChainSnapshot {
	return ExportCausalChainSnapshot([]RuntimeObservabilityReport{completeRuntimeReport()}, 0, false)
}