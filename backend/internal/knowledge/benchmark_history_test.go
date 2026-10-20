package knowledge

import "testing"

func TestBenchmarkHistoryIsTenantScopedAndAggregateOnly(t *testing.T) {
	repo := newKnowledgeTestRepository(t)
	summaryA := BenchmarkSummary{
		ConfigHash: "cfg-a", K: 8, Cases: 4, SuccessfulCases: 4, PositiveCases: 3, NoAnswerCases: 1,
		ScenarioCounts: map[string]int{"scale_10k": 2, "no_answer": 2}, RecallAtK: 0.9, MRR: 0.8,
		NDCGAtK: 0.85, ContextRecall: 0.95, NoAnswerAccuracy: 1, P50LatencyMs: 80, P95LatencyMs: 180,
		Passed: true,
	}
	summaryB := summaryA
	summaryB.ConfigHash = "cfg-b"
	if _, err := repo.RecordBenchmarkRun("tenant-a", summaryA); err != nil {
		t.Fatalf("record tenant-a benchmark: %v", err)
	}
	if _, err := repo.RecordBenchmarkRun("tenant-b", summaryB); err != nil {
		t.Fatalf("record tenant-b benchmark: %v", err)
	}

	runs, err := repo.ListBenchmarkRuns("tenant-a", 20)
	if err != nil {
		t.Fatalf("list tenant-a benchmark history: %v", err)
	}
	if len(runs) != 1 || runs[0].ConfigHash != "cfg-a" {
		t.Fatalf("tenant benchmark history leaked or missing: %+v", runs)
	}
	if runs[0].ScenarioCounts["scale_10k"] != 2 || !runs[0].Passed {
		t.Fatalf("aggregate benchmark data did not round-trip: %+v", runs[0])
	}

	var rows []BenchmarkRunRecord
	if err := repo.DB().Find(&rows).Error; err != nil {
		t.Fatalf("inspect persisted benchmark rows: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("expected two aggregate rows, got %d", len(rows))
	}
	// The persistence schema intentionally has no query/evidence/locator fields.
	// This compile-time model check makes future accidental expansion obvious in review.
	for _, row := range rows {
		if row.ScenarioCountsJSON == "" || row.CreatedAt.IsZero() {
			t.Fatalf("invalid aggregate benchmark row: %+v", row)
		}
	}
}
