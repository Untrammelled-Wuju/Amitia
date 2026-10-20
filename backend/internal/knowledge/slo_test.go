package knowledge

import (
	"encoding/json"
	"testing"
	"time"
)

func TestPercentileInt64(t *testing.T) {
	values := []int64{10, 20, 30, 40, 50, 60, 70, 80, 90, 100}
	if got := percentileInt64(values, 0.50); got != 50 {
		t.Fatalf("p50 = %d", got)
	}
	if got := percentileInt64(values, 0.95); got != 100 {
		t.Fatalf("p95 = %d", got)
	}
	if got := percentileInt64(nil, 0.95); got != 0 {
		t.Fatalf("empty percentile = %d", got)
	}
}

func TestRetrievalSLOSummaryIsScopedByConfigHash(t *testing.T) {
	repo := newKnowledgeTestRepository(t)
	if err := repo.db.Exec(`CREATE TABLE retrieval_traces (
		id TEXT PRIMARY KEY, tenant_id TEXT NOT NULL, duration_ms INTEGER NOT NULL,
		error_code TEXT NOT NULL DEFAULT '', query_plan_json TEXT NOT NULL DEFAULT '{}',
		slo_target_ms INTEGER NOT NULL DEFAULT 0, slo_exceeded INTEGER NOT NULL DEFAULT 0,
		config_hash TEXT NOT NULL DEFAULT '', created_at DATETIME NOT NULL
	)`).Error; err != nil {
		t.Fatalf("create retrieval traces: %v", err)
	}
	now := time.Now().UTC()
	insert := func(id, hash, level string, duration, target int64, exceeded int) {
		plan, _ := json.Marshal(retrievalQueryPlanTrace{Level: level})
		if err := repo.db.Exec(`INSERT INTO retrieval_traces
			(id, tenant_id, duration_ms, error_code, query_plan_json, slo_target_ms, slo_exceeded, config_hash, created_at)
			VALUES (?, 'tenant-1', ?, '', ?, ?, ?, ?, ?)`, id, duration, string(plan), target, exceeded, hash, now).Error; err != nil {
			t.Fatalf("insert trace: %v", err)
		}
	}
	insert("1", "cfg-a", RetrievalLevelFast, 100, 150, 0)
	insert("2", "cfg-a", RetrievalLevelFast, 200, 150, 1)
	insert("3", "cfg-b", RetrievalLevelFast, 9999, 150, 1)

	summary, err := repo.RetrievalSLOSummary("tenant-1", "cfg-a", time.Hour, 2, map[string]time.Duration{RetrievalLevelFast: 150 * time.Millisecond})
	if err != nil {
		t.Fatalf("summary: %v", err)
	}
	if len(summary.Levels) != 3 {
		t.Fatalf("levels = %#v", summary.Levels)
	}
	fast := summary.Levels[0]
	if fast.Samples != 2 || fast.P95Ms != 200 || fast.ExceededCount != 1 || !fast.EnoughSamples || fast.TargetMet {
		t.Fatalf("unexpected FAST summary: %#v", fast)
	}
}
