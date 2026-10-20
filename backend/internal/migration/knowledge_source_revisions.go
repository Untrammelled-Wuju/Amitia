package migration

// KnowledgeSourceRevisionsMigration turns knowledge_sources into the logical
// source pointer and preserves immutable file/parser/index metadata for every
// revision. The active revision remains on knowledge_sources so existing read
// paths can switch revisions atomically after a new revision is fully ready.
func KnowledgeSourceRevisionsMigration() Migration {
	return Migration{
		Version: "20260926002",
		Name:    "knowledge_source_revisions_and_reindex_jobs",
		Up: func(s *Step) error {
			s.CreateTable(`CREATE TABLE IF NOT EXISTS knowledge_source_revisions (
                id TEXT PRIMARY KEY,
                tenant_id TEXT NOT NULL,
                source_id TEXT NOT NULL,
                revision INTEGER NOT NULL,
                filename TEXT NOT NULL DEFAULT '',
                mime_type TEXT NOT NULL DEFAULT '',
                source_type TEXT NOT NULL DEFAULT 'file',
                storage_key TEXT NOT NULL DEFAULT '',
                size_bytes INTEGER NOT NULL DEFAULT 0,
                sha256 TEXT NOT NULL DEFAULT '',
                parser_version TEXT NOT NULL DEFAULT 'knowledge-parser-v1',
                chunk_schema_version TEXT NOT NULL DEFAULT 'knowledge-chunk-v1',
                embedding_version TEXT NOT NULL DEFAULT '',
                index_version TEXT NOT NULL DEFAULT 'v1',
                status TEXT NOT NULL DEFAULT 'pending',
                expected_units INTEGER NOT NULL DEFAULT 0,
                processed_units INTEGER NOT NULL DEFAULT 0,
                failed_units INTEGER NOT NULL DEFAULT 0,
                indexed_chunks INTEGER NOT NULL DEFAULT 0,
                last_error TEXT NOT NULL DEFAULT '',
                created_at DATETIME NOT NULL,
                updated_at DATETIME NOT NULL,
                activated_at DATETIME,
                UNIQUE(tenant_id, source_id, revision)
            )`)
			s.CreateIndex("idx_knowledge_revisions_source", "knowledge_source_revisions", []string{"tenant_id", "source_id", "revision"}, true)
			s.CreateIndex("idx_knowledge_revisions_sha", "knowledge_source_revisions", []string{"tenant_id", "sha256"}, false)
			s.CreateIndex("idx_knowledge_revisions_status", "knowledge_source_revisions", []string{"tenant_id", "status", "updated_at"}, false)

			// Existing installations only had one mutable source row. Preserve that
			// current snapshot as the first immutable revision before new writes use
			// the revision table.
			s.Execute(`INSERT OR IGNORE INTO knowledge_source_revisions
                (id, tenant_id, source_id, revision, filename, mime_type, source_type, storage_key, size_bytes, sha256,
                 parser_version, chunk_schema_version, embedding_version, index_version, status,
                 expected_units, processed_units, failed_units, indexed_chunks, last_error,
                 created_at, updated_at, activated_at)
                SELECT id || ':r' || revision, tenant_id, id, revision, filename, mime_type, source_type, storage_key,
                       size_bytes, sha256, parser_version, chunk_schema_version, embedding_version, index_version,
                       CASE status
                         WHEN 'ready' THEN 'ready'
                         WHEN 'ready_degraded' THEN 'ready_degraded'
                         WHEN 'failed' THEN 'failed'
                         WHEN 'cancelled' THEN 'cancelled'
                         WHEN 'processing' THEN 'processing'
                         ELSE 'pending'
                       END,
                       expected_units, processed_units, failed_units, indexed_chunks, last_error,
                       created_at, updated_at,
                       CASE WHEN status IN ('ready','ready_degraded') THEN updated_at ELSE NULL END
                FROM knowledge_sources`)

			if err := s.AddColumn("knowledge_ingest_jobs", "kind", "TEXT NOT NULL DEFAULT 'ingest'"); err != nil {
				return err
			}
			s.Execute(`UPDATE knowledge_ingest_jobs SET kind = 'ingest' WHERE TRIM(COALESCE(kind,'')) = ''`)
			if err := s.AddColumn("knowledge_index_outbox", "source_revision", "INTEGER NOT NULL DEFAULT 0"); err != nil {
				return err
			}
			s.CreateIndex("idx_knowledge_outbox_revision", "knowledge_index_outbox", []string{"tenant_id", "aggregate_type", "aggregate_id", "source_revision"}, false)
			return nil
		},
	}
}
