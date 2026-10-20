package migration

import (
	"fmt"
	"sort"
	"strings"

	"gorm.io/gorm"
)

// ValidateRAGWorldBookKnowledgeSchema is the production schema contract for the
// final WorldBook/Knowledge architecture. It intentionally runs after either
// the fresh-baseline path or the incremental-migration path so a partial/manual
// schema can never be marked healthy and then fail later inside chat retrieval.
func ValidateRAGWorldBookKnowledgeSchema(db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("rag schema contract: db is required")
	}

	requiredColumns := map[string][]string{
		"knowledge_sources": {
			"id", "tenant_id", "logical_name", "filename", "mime_type", "source_type", "storage_key",
			"size_bytes", "sha256", "revision", "parser_version", "chunk_schema_version", "config_hash",
			"embedding_version", "index_version", "status", "expected_units", "processed_units", "failed_units",
			"indexed_chunks", "last_error", "created_at", "updated_at", "deleted_at",
		},
		"knowledge_source_revisions": {
			"id", "tenant_id", "source_id", "revision", "filename", "mime_type", "source_type", "storage_key",
			"size_bytes", "sha256", "parser_version", "chunk_schema_version", "config_hash", "embedding_version",
			"index_version", "status", "expected_units", "processed_units", "failed_units", "indexed_chunks",
			"last_error", "created_at", "updated_at", "activated_at", "retain_until", "purged_at",
		},
		"knowledge_units": {
			"id", "tenant_id", "source_id", "source_revision", "ordinal", "unit_type", "title", "content",
			"locator_json", "metadata_json", "checksum", "valid_from", "valid_to", "supersedes_json", "conflict_group", "created_at",
		},
		"knowledge_chunks": {
			"id", "tenant_id", "source_id", "source_revision", "unit_id", "chunk_index", "parent_chunk_id",
			"title", "content", "embedding_text", "token_count", "locator_json", "metadata_json", "checksum",
			"embedding_version", "index_version", "status", "valid_from", "valid_to", "supersedes_json",
			"conflict_group", "created_at", "updated_at",
		},
		"knowledge_ingest_jobs": {
			"id", "tenant_id", "source_id", "source_revision", "kind", "stage", "status", "cursor_json",
			"last_committed_ordinal", "expected_units", "processed_units", "failed_units", "indexed_chunks",
			"retry_count", "heartbeat_at", "last_error", "created_at", "updated_at", "finished_at",
		},
		"knowledge_index_outbox": {
			"id", "tenant_id", "aggregate_type", "aggregate_id", "source_revision", "operation", "payload_json",
			"status", "retry_count", "last_error", "next_attempt_at", "lease_token", "lease_until", "created_at", "updated_at",
		},
		"knowledge_index_aliases": {"tenant_id", "active_version", "previous_version", "updated_at"},
		"knowledge_benchmark_runs": {
			"id", "tenant_id", "config_hash", "k", "cases", "successful_cases", "error_cases", "failed_cases",
			"positive_cases", "no_answer_cases", "citation_observed_cases", "scenario_counts_json", "recall_at_k",
			"mrr", "ndcg_at_k", "context_precision", "context_recall", "no_answer_accuracy", "citation_precision",
			"citation_recall", "groundedness", "error_rate", "p50_latency_ms", "p95_latency_ms", "passed",
			"violations_json", "created_at",
		},
		"retrieval_traces": {
			"id", "tenant_id", "request_id", "query_hash", "sources_json", "candidate_count", "selected_count",
			"sufficient", "duration_ms", "error_code", "temporal_mode", "as_of", "degraded", "ambiguous",
			"warnings_json", "candidate_trace_json", "evidence_trace_json", "query_plan_json", "config_hash",
			"slo_target_ms", "slo_exceeded", "policy_mode", "route_reason", "shadow_level", "shadow_reason", "created_at",
		},
	}

	for table, columns := range requiredColumns {
		if err := requireSchemaColumns(db, table, columns); err != nil {
			return err
		}
	}

	// Knowledge is global within the authenticated tenant/space boundary. Scan
	// every knowledge_* table, including future additions, so character scoping
	// cannot silently creep back in through a later migration.
	var knowledgeTables []struct {
		Name string `gorm:"column:name"`
	}
	if err := db.Raw(`SELECT name FROM sqlite_master WHERE type='table' AND name LIKE 'knowledge_%' ORDER BY name`).Scan(&knowledgeTables).Error; err != nil {
		return fmt.Errorf("rag schema contract: list knowledge tables: %w", err)
	}
	for _, row := range knowledgeTables {
		columns, err := schemaTableColumns(db, row.Name)
		if err != nil {
			return err
		}
		for name := range columns {
			if strings.EqualFold(name, "character_id") || strings.EqualFold(name, "characterid") {
				return fmt.Errorf("rag schema contract: forbidden role scope column %s.%s", row.Name, name)
			}
		}
	}

	requiredIndexes := []string{
		"idx_knowledge_sources_tenant_status",
		"idx_knowledge_sources_tenant_sha",
		"idx_knowledge_revisions_source",
		"idx_knowledge_revisions_status",
		"idx_knowledge_revisions_retention",
		"idx_knowledge_units_tenant_source",
		"idx_knowledge_units_temporal",
		"idx_knowledge_units_conflict",
		"idx_knowledge_chunks_tenant_source",
		"idx_knowledge_chunks_index_status",
		"idx_knowledge_chunks_temporal",
		"idx_knowledge_chunks_conflict",
		"idx_knowledge_jobs_tenant_status",
		"idx_knowledge_jobs_source_revision",
		"idx_knowledge_outbox_aggregate",
		"idx_knowledge_outbox_lease",
		"idx_knowledge_benchmark_runs_tenant_created",
		"idx_knowledge_benchmark_runs_tenant_config_created",
		"idx_retrieval_traces_tenant_created",
		"idx_retrieval_traces_tenant_mode_created",
		"idx_retrieval_traces_tenant_config_created",
		"idx_retrieval_traces_tenant_slo_created",
		"idx_retrieval_traces_tenant_policy_created",
	}
	for _, index := range requiredIndexes {
		var count int64
		if err := db.Raw(`SELECT COUNT(*) FROM sqlite_master WHERE type='index' AND name=?`, index).Scan(&count).Error; err != nil {
			return fmt.Errorf("rag schema contract: inspect index %s: %w", index, err)
		}
		if count != 1 {
			return fmt.Errorf("rag schema contract: missing required index %s", index)
		}
	}

	criticalIndexes := []struct {
		name    string
		columns []string
		unique  bool
	}{
		{name: "idx_knowledge_revisions_source", columns: []string{"tenant_id", "source_id", "revision"}, unique: true},
		{name: "idx_knowledge_outbox_aggregate", columns: []string{"tenant_id", "aggregate_type", "aggregate_id", "source_revision"}},
		{name: "idx_knowledge_outbox_lease", columns: []string{"tenant_id", "status", "next_attempt_at", "lease_until"}},
		{name: "idx_knowledge_benchmark_runs_tenant_config_created", columns: []string{"tenant_id", "config_hash", "created_at"}},
		{name: "idx_retrieval_traces_tenant_config_created", columns: []string{"tenant_id", "config_hash", "created_at"}},
		{name: "idx_retrieval_traces_tenant_policy_created", columns: []string{"tenant_id", "policy_mode", "created_at"}},
	}
	for _, index := range criticalIndexes {
		if err := requireSchemaIndexShape(db, index.name, index.columns, index.unique); err != nil {
			return err
		}
	}

	return nil
}

type schemaColumnInfo struct {
	Name    string `gorm:"column:name"`
	NotNull int    `gorm:"column:notnull"`
}

func schemaTableColumns(db *gorm.DB, table string) (map[string]schemaColumnInfo, error) {
	if !safeIdentifier(table) {
		return nil, fmt.Errorf("rag schema contract: unsafe table name %q", table)
	}
	var rows []schemaColumnInfo
	if err := db.Raw("PRAGMA table_info(" + table + ")").Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("rag schema contract: inspect table %s: %w", table, err)
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("rag schema contract: missing required table %s", table)
	}
	out := make(map[string]schemaColumnInfo, len(rows))
	for _, row := range rows {
		out[row.Name] = row
	}
	return out, nil
}

func requireSchemaColumns(db *gorm.DB, table string, required []string) error {
	columns, err := schemaTableColumns(db, table)
	if err != nil {
		return err
	}
	missing := make([]string, 0)
	for _, column := range required {
		if _, ok := columns[column]; !ok {
			missing = append(missing, column)
		}
	}
	if len(missing) == 0 {
		return nil
	}
	sort.Strings(missing)
	return fmt.Errorf("rag schema contract: table %s missing columns: %s", table, strings.Join(missing, ", "))
}

func requireSchemaIndexShape(db *gorm.DB, index string, expectedColumns []string, unique bool) error {
	if !safeIdentifier(index) {
		return fmt.Errorf("rag schema contract: unsafe index name %q", index)
	}
	var rows []struct {
		SeqNo int    `gorm:"column:seqno"`
		Name  string `gorm:"column:name"`
	}
	if err := db.Raw("PRAGMA index_info(" + index + ")").Scan(&rows).Error; err != nil {
		return fmt.Errorf("rag schema contract: inspect index shape %s: %w", index, err)
	}
	if len(rows) != len(expectedColumns) {
		return fmt.Errorf("rag schema contract: index %s has %d columns, expected %d", index, len(rows), len(expectedColumns))
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].SeqNo < rows[j].SeqNo })
	for i, expected := range expectedColumns {
		if rows[i].Name != expected {
			return fmt.Errorf("rag schema contract: index %s column %d is %s, expected %s", index, i, rows[i].Name, expected)
		}
	}
	if unique {
		var sql string
		if err := db.Raw(`SELECT COALESCE(sql,'') FROM sqlite_master WHERE type='index' AND name=?`, index).Scan(&sql).Error; err != nil {
			return fmt.Errorf("rag schema contract: inspect unique index %s: %w", index, err)
		}
		if !strings.HasPrefix(strings.ToUpper(strings.TrimSpace(sql)), "CREATE UNIQUE INDEX") {
			return fmt.Errorf("rag schema contract: index %s must be UNIQUE", index)
		}
	}
	return nil
}
