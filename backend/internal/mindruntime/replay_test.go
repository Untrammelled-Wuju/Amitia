package mindruntime

import (
	"testing"
	"time"
)

func TestBuildRuntimeReplayRecordIndexesQueryableIDs(t *testing.T) {
	snapshot := BuildRuntimeSnapshot(completeSnapshotInput())

	record := BuildRuntimeReplayRecord(RuntimeObservabilityInput{
		Snapshot:          snapshot,
		RequestID:         " req-3 ",
		EventID:           " event-3 ",
		DeliveryID:        " delivery-3 ",
		ToolID:            " tool-3 ",
		Path:              "standard",
		Priority:          "P1",
		Scope:             "scope-test",
		InteractionStatus: "completed",
	}, 24*time.Hour)

	if record.ID != snapshot.ID {
		t.Fatalf("unexpected record id: %s", record.ID)
	}
	if !record.Redacted {
		t.Fatal("expected redacted replay record")
	}
	if record.Index.RequestID != "req-3" || record.Index.EventID != "event-3" {
		t.Fatalf("unexpected request/event index: %#v", record.Index)
	}
	if record.Index.InteractionID != snapshot.InteractionID {
		t.Fatalf("unexpected interaction index: %#v", record.Index)
	}
	if record.Index.DeliveryID != "delivery-3" || record.Index.ToolID != "tool-3" {
		t.Fatalf("unexpected delivery/tool index: %#v", record.Index)
	}
	if record.Index.Scope != "scope-test" {
		t.Fatalf("unexpected scope index: %s", record.Index.Scope)
	}
	if record.RetentionUntil != snapshot.CreatedAt.Add(24*time.Hour).UTC() {
		t.Fatalf("unexpected retention: %s", record.RetentionUntil)
	}
}

func TestRuntimeReplayRecordMatchesOnlyExplicitQueries(t *testing.T) {
	record := BuildRuntimeReplayRecord(RuntimeObservabilityInput{
		Snapshot:   BuildRuntimeSnapshot(completeSnapshotInput()),
		RequestID:  "req-4",
		EventID:    "event-4",
		DeliveryID: "delivery-4",
		ToolID:     "tool-4",
	}, time.Hour)

	if record.Matches(RuntimeReplayQuery{}) {
		t.Fatal("empty query should not match")
	}
	if !record.Matches(RuntimeReplayQuery{RequestID: "req-4"}) {
		t.Fatal("expected request query match")
	}
	if !record.Matches(RuntimeReplayQuery{InteractionID: "interaction-1", DeliveryID: "delivery-4"}) {
		t.Fatal("expected combined query match")
	}
	if record.Matches(RuntimeReplayQuery{RequestID: "req-4", ToolID: "other"}) {
		t.Fatal("mismatched tool query should not match")
	}
}

func TestRuntimeReplayRecordExpiredAtHonorsRetention(t *testing.T) {
	snapshot := BuildRuntimeSnapshot(completeSnapshotInput())
	record := BuildRuntimeReplayRecord(RuntimeObservabilityInput{Snapshot: snapshot}, time.Hour)

	if record.ExpiredAt(snapshot.CreatedAt.Add(59 * time.Minute)) {
		t.Fatal("record expired too early")
	}
	if !record.ExpiredAt(snapshot.CreatedAt.Add(time.Hour)) {
		t.Fatal("record should expire at retention boundary")
	}
	withoutRetention := BuildRuntimeReplayRecord(RuntimeObservabilityInput{Snapshot: snapshot}, 0)
	if withoutRetention.ExpiredAt(snapshot.CreatedAt.Add(90 * 24 * time.Hour)) {
		t.Fatal("record without retention should not expire")
	}
}

func TestRuntimeReplayRecordMatchesByScope(t *testing.T) {
	record := BuildRuntimeReplayRecord(RuntimeObservabilityInput{
		Snapshot: BuildRuntimeSnapshot(completeSnapshotInput()),
		Scope:    "scope-a",
	}, time.Hour)

	if !record.Matches(RuntimeReplayQuery{Scope: "scope-a"}) {
		t.Fatal("expected scope match")
	}
	if record.Matches(RuntimeReplayQuery{Scope: "scope-b"}) {
		t.Fatal("expected scope mismatch")
	}
}

func TestReconstructCausalChain(t *testing.T) {
	snap := BuildRuntimeSnapshot(completeSnapshotInput())
	report := BuildRuntimeObservabilityReport(RuntimeObservabilityInput{
		Snapshot:          snap,
		RequestID:         "req-r",
		InteractionStatus: "active",
	})
	chain := ReconstructCausalChain(report.CausalChain)
	if chain.EventCount == 0 {
		t.Fatal("expected non-zero event count")
	}
	if chain.EventCount != len(report.CausalChain) {
		t.Fatalf("event count mismatch: %d vs %d", chain.EventCount, len(report.CausalChain))
	}
}

func TestReconstructCausalChain_empty(t *testing.T) {
	chain := ReconstructCausalChain(nil)
	if chain.EventCount != 0 {
		t.Fatal("expected zero event count for nil")
	}
}

func TestFindEventsByKind(t *testing.T) {
	snap := BuildRuntimeSnapshot(completeSnapshotInput())
	report := BuildRuntimeObservabilityReport(RuntimeObservabilityInput{
		Snapshot:          snap,
		RequestID:         "req-r",
		InteractionStatus: "active",
	})
	cancelled := FindCancelledEvents(report.CausalChain)
	if len(cancelled) != 0 {
		t.Fatalf("expected no cancelled events, got %d", len(cancelled))
	}
	superseded := FindSupersededEvents(report.CausalChain)
	if len(superseded) != 0 {
		t.Fatalf("expected no superseded events, got %d", len(superseded))
	}
}

func TestFindEventsByStatus(t *testing.T) {
	snap := BuildRuntimeSnapshot(completeSnapshotInput())
	report := BuildRuntimeObservabilityReport(RuntimeObservabilityInput{
		Snapshot:          snap,
		RequestID:         "req-r",
		InteractionStatus: "active",
	})
	result := FindEventsByStatus(report.CausalChain, "active")
	if len(result) == 0 {
		t.Fatal("expected at least one active event")
	}
}

func TestReplaySideEffectLog(t *testing.T) {
	log := NewReplaySideEffectLog()
	log.Record("e1", TraceEventTool, true)
	log.Record("e2", TraceEventDelivery, false)
	log.Record("e3", TraceEventRequest, true)
	log.Rollback("e1")

	applied := log.AppliedEffects()
	if len(applied) != 2 {
		t.Fatalf("expected 2 applied effects, got %d", len(applied))
	}
}
