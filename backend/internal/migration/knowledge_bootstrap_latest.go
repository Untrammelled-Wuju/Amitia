package migration

// KnowledgeBootstrapLatestMigration is additive on the modern source. Unlike
// the old RAGWorldBookKnowledgeMigration it never rebuilds world_book: the
// newest WorldBook and Canonical Memory schema remain authoritative.
func KnowledgeBootstrapLatestMigration() Migration {
	return Migration{
		Version: "20260926001",
		Name:    "knowledge_domain_additive_latest_compatible",
		Up: func(s *Step) error {
			s.CreateTable(`CREATE TABLE IF NOT EXISTS knowledge_sources (
                id TEXT PRIMARY KEY,
                tenant_id TEXT NOT NULL,
                logical_name TEXT NOT NULL DEFAULT '',
                filename TEXT NOT NULL DEFAULT '',
                mime_type TEXT NOT NULL DEFAULT '',
                source_type TEXT NOT NULL DEFAULT 'file',
                storage_key TEXT NOT NULL DEFAULT '',
                size_bytes INTEGER NOT NULL DEFAULT 0,
                sha256 TEXT NOT NULL DEFAULT '',
                revision INTEGER NOT NULL DEFAULT 1,
                parser_version TEXT NOT NULL DEFAULT 'v1',
                chunk_schema_version TEXT NOT NULL DEFAULT 'v1',
                embedding_version TEXT NOT NULL DEFAULT '',
                index_version TEXT NOT NULL DEFAULT '',
                status TEXT NOT NULL DEFAULT 'pending',
                expected_units INTEGER NOT NULL DEFAULT 0,
                processed_units INTEGER NOT NULL DEFAULT 0,
                failed_units INTEGER NOT NULL DEFAULT 0,
                indexed_chunks INTEGER NOT NULL DEFAULT 0,
                last_error TEXT NOT NULL DEFAULT '',
                created_at DATETIME NOT NULL,
                updated_at DATETIME NOT NULL,
                deleted_at DATETIME
            )`)
			s.CreateIndex("idx_knowledge_sources_tenant_status", "knowledge_sources", []string{"tenant_id", "status"}, false)
			s.CreateIndex("idx_knowledge_sources_tenant_sha", "knowledge_sources", []string{"tenant_id", "sha256"}, false)

			s.CreateTable(`CREATE TABLE IF NOT EXISTS knowledge_units (
                id TEXT PRIMARY KEY,
                tenant_id TEXT NOT NULL,
                source_id TEXT NOT NULL,
                source_revision INTEGER NOT NULL,
                ordinal INTEGER NOT NULL,
                unit_type TEXT NOT NULL DEFAULT 'text',
                title TEXT NOT NULL DEFAULT '',
                content TEXT NOT NULL DEFAULT '',
                locator_json TEXT NOT NULL DEFAULT '{}',
                metadata_json TEXT NOT NULL DEFAULT '{}',
                checksum TEXT NOT NULL DEFAULT '',
                created_at DATETIME NOT NULL,
                UNIQUE(source_id, source_revision, ordinal)
            )`)
			s.CreateIndex("idx_knowledge_units_tenant_source", "knowledge_units", []string{"tenant_id", "source_id", "source_revision"}, false)

			s.CreateTable(`CREATE TABLE IF NOT EXISTS knowledge_chunks (
                id TEXT PRIMARY KEY,
                tenant_id TEXT NOT NULL,
                source_id TEXT NOT NULL,
                source_revision INTEGER NOT NULL,
                unit_id TEXT NOT NULL,
                chunk_index INTEGER NOT NULL,
                parent_chunk_id TEXT NOT NULL DEFAULT '',
                title TEXT NOT NULL DEFAULT '',
                content TEXT NOT NULL DEFAULT '',
                embedding_text TEXT NOT NULL DEFAULT '',
                token_count INTEGER NOT NULL DEFAULT 0,
                locator_json TEXT NOT NULL DEFAULT '{}',
                metadata_json TEXT NOT NULL DEFAULT '{}',
                checksum TEXT NOT NULL DEFAULT '',
                embedding_version TEXT NOT NULL DEFAULT '',
                index_version TEXT NOT NULL DEFAULT '',
                status TEXT NOT NULL DEFAULT 'pending',
                created_at DATETIME NOT NULL,
                updated_at DATETIME NOT NULL,
                UNIQUE(unit_id, chunk_index)
            )`)
			s.CreateIndex("idx_knowledge_chunks_tenant_source", "knowledge_chunks", []string{"tenant_id", "source_id", "source_revision"}, false)
			s.CreateIndex("idx_knowledge_chunks_index_status", "knowledge_chunks", []string{"tenant_id", "index_version", "status"}, false)

			s.CreateTable(`CREATE TABLE IF NOT EXISTS knowledge_ingest_jobs (
                id TEXT PRIMARY KEY,
                tenant_id TEXT NOT NULL,
                source_id TEXT NOT NULL,
                source_revision INTEGER NOT NULL,
                stage TEXT NOT NULL DEFAULT 'queued',
                status TEXT NOT NULL DEFAULT 'queued',
                cursor_json TEXT NOT NULL DEFAULT '{}',
                last_committed_ordinal INTEGER NOT NULL DEFAULT -1,
                expected_units INTEGER NOT NULL DEFAULT 0,
                processed_units INTEGER NOT NULL DEFAULT 0,
                failed_units INTEGER NOT NULL DEFAULT 0,
                indexed_chunks INTEGER NOT NULL DEFAULT 0,
                retry_count INTEGER NOT NULL DEFAULT 0,
                heartbeat_at DATETIME,
                last_error TEXT NOT NULL DEFAULT '',
                created_at DATETIME NOT NULL,
                updated_at DATETIME NOT NULL,
                finished_at DATETIME
            )`)
			s.CreateIndex("idx_knowledge_jobs_tenant_status", "knowledge_ingest_jobs", []string{"tenant_id", "status", "updated_at"}, false)
			s.CreateIndex("idx_knowledge_jobs_source_revision", "knowledge_ingest_jobs", []string{"source_id", "source_revision"}, false)

			s.CreateTable(`CREATE TABLE IF NOT EXISTS knowledge_index_outbox (
                id TEXT PRIMARY KEY,
                tenant_id TEXT NOT NULL,
                aggregate_type TEXT NOT NULL,
                aggregate_id TEXT NOT NULL,
                operation TEXT NOT NULL,
                payload_json TEXT NOT NULL DEFAULT '{}',
                status TEXT NOT NULL DEFAULT 'pending',
                retry_count INTEGER NOT NULL DEFAULT 0,
                last_error TEXT NOT NULL DEFAULT '',
                next_attempt_at DATETIME,
                created_at DATETIME NOT NULL,
                updated_at DATETIME NOT NULL
            )`)
			s.CreateIndex("idx_knowledge_outbox_status", "knowledge_index_outbox", []string{"status", "next_attempt_at"}, false)
			s.CreateIndex("idx_knowledge_outbox_aggregate", "knowledge_index_outbox", []string{"tenant_id", "aggregate_type", "aggregate_id"}, false)

			s.CreateTable(`CREATE TABLE IF NOT EXISTS knowledge_index_aliases (
                tenant_id TEXT PRIMARY KEY,
                active_version TEXT NOT NULL DEFAULT 'v1',
                previous_version TEXT NOT NULL DEFAULT '',
                updated_at DATETIME NOT NULL
            )`)

			s.CreateTable(`CREATE TABLE IF NOT EXISTS retrieval_traces (
                id TEXT PRIMARY KEY,
                tenant_id TEXT NOT NULL,
                request_id TEXT NOT NULL DEFAULT '',
                query_hash TEXT NOT NULL DEFAULT '',
                sources_json TEXT NOT NULL DEFAULT '[]',
                candidate_count INTEGER NOT NULL DEFAULT 0,
                selected_count INTEGER NOT NULL DEFAULT 0,
                sufficient INTEGER NOT NULL DEFAULT 0,
                duration_ms INTEGER NOT NULL DEFAULT 0,
                error_code TEXT NOT NULL DEFAULT '',
                created_at DATETIME NOT NULL
            )`)
			s.CreateIndex("idx_retrieval_traces_tenant_created", "retrieval_traces", []string{"tenant_id", "created_at"}, false)
			return nil
		},
	}
}
