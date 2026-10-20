package knowledge

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestListRetrievalTracesIsTenantScopedAndReturnsOnlySanitizedPayload(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.Exec(`CREATE TABLE retrieval_traces (
		id TEXT PRIMARY KEY, tenant_id TEXT NOT NULL, request_id TEXT NOT NULL DEFAULT '', query_hash TEXT NOT NULL DEFAULT '',
		candidate_count INTEGER NOT NULL DEFAULT 0, selected_count INTEGER NOT NULL DEFAULT 0, sufficient INTEGER NOT NULL DEFAULT 0,
		duration_ms INTEGER NOT NULL DEFAULT 0, error_code TEXT NOT NULL DEFAULT '', temporal_mode TEXT NOT NULL DEFAULT 'current', as_of DATETIME,
		degraded INTEGER NOT NULL DEFAULT 0, ambiguous INTEGER NOT NULL DEFAULT 0, warnings_json TEXT NOT NULL DEFAULT '[]',
		candidate_trace_json TEXT NOT NULL DEFAULT '[]', evidence_trace_json TEXT NOT NULL DEFAULT '[]', query_plan_json TEXT NOT NULL DEFAULT '{}',
		config_hash TEXT NOT NULL DEFAULT '', slo_target_ms INTEGER NOT NULL DEFAULT 0, slo_exceeded INTEGER NOT NULL DEFAULT 0,
		policy_mode TEXT NOT NULL DEFAULT '', route_reason TEXT NOT NULL DEFAULT '', shadow_level TEXT NOT NULL DEFAULT '', shadow_reason TEXT NOT NULL DEFAULT '',
		created_at DATETIME NOT NULL
	)`).Error; err != nil {
		t.Fatalf("create traces table: %v", err)
	}

	candidateJSON, _ := json.Marshal([]RetrievalCandidateTrace{{Rank: 1, EvidenceID: "ev-1", SourceID: "source-a", LocatorHash: strings.Repeat("a", 64)}})
	evidenceJSON, _ := json.Marshal([]RetrievalEvidenceTrace{{Rank: 1, EvidenceID: "ev-1", SourceID: "source-a", LocatorHash: strings.Repeat("b", 64)}})
	planJSON, _ := json.Marshal(RetrievalQueryPlanTrace{Level: RetrievalLevelDeep, Complexity: "comparison", OriginalHash: strings.Repeat("c", 64), QueryHistory: []string{strings.Repeat("d", 64)}, QueryCount: 1})
	now := time.Now().UTC()
	insert := func(id, tenant string, created time.Time) {
		if err := db.Exec(`INSERT INTO retrieval_traces
			(id, tenant_id, request_id, query_hash, candidate_count, selected_count, sufficient, duration_ms, error_code, temporal_mode,
			 degraded, ambiguous, warnings_json, candidate_trace_json, evidence_trace_json, query_plan_json, config_hash, slo_target_ms, slo_exceeded,
			 policy_mode, route_reason, shadow_level, shadow_reason, created_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			id, tenant, "request-"+id, strings.Repeat("e", 64), 12, 2, 1, 88, "", "current", 0, 0, `[]`, string(candidateJSON), string(evidenceJSON), string(planJSON), "cfg", 200, 0,
			"legacy_shadow", "legacy_non_trivial", RetrievalLevelDeep, "explicit_document_read", created).Error; err != nil {
			t.Fatalf("insert trace: %v", err)
		}
	}
	insert("trace-a1", "tenant-a", now.Add(-time.Minute))
	insert("trace-a2", "tenant-a", now)
	insert("trace-b", "tenant-b", now.Add(time.Minute))

	repo := NewRepository(db)
	items, err := repo.ListRetrievalTraces("tenant-a", 1)
	if err != nil {
		t.Fatalf("list traces: %v", err)
	}
	if len(items) != 1 || items[0].ID != "trace-a2" {
		t.Fatalf("unexpected tenant scoped/limited traces: %#v", items)
	}
	if items[0].QueryPlan.Level != RetrievalLevelDeep || len(items[0].Candidates) != 1 || len(items[0].Evidence) != 1 {
		t.Fatalf("sanitized diagnostics were not decoded: %#v", items[0])
	}
	if items[0].PolicyMode != "legacy_shadow" || items[0].RouteReason != "legacy_non_trivial" || items[0].ShadowLevel != RetrievalLevelDeep || items[0].ShadowReason != "explicit_document_read" {
		t.Fatalf("policy diagnostics were not decoded: %#v", items[0])
	}
	encoded, _ := json.Marshal(items[0])
	text := string(encoded)
	for _, forbidden := range []string{"document body", `\"page\"`, "private/location"} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("trace developer view leaked raw data %q: %s", forbidden, text)
		}
	}
}
