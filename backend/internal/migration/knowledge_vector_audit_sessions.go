package migration

func KnowledgeVectorAuditSessionsMigration() Migration {
	return Migration{
		Version: "20261016003",
		Name:    "knowledge_vector_audit_sessions",
		Up: func(s *Step) error {
			s.CreateTable(`CREATE TABLE IF NOT EXISTS knowledge_vector_audit_sessions (
                id TEXT PRIMARY KEY,
                tenant_id TEXT NOT NULL,
                collections_hash TEXT NOT NULL,
                page_limit INTEGER NOT NULL,
                page INTEGER NOT NULL DEFAULT 0,
                next_cursor TEXT NOT NULL DEFAULT '',
                last_cursor TEXT NOT NULL DEFAULT '',
                last_result_json TEXT NOT NULL DEFAULT '{}',
                sampled INTEGER NOT NULL DEFAULT 0,
                suspected_orphans INTEGER NOT NULL DEFAULT 0,
                metadata_mismatches INTEGER NOT NULL DEFAULT 0,
                in_flight INTEGER NOT NULL DEFAULT 0,
                status TEXT NOT NULL DEFAULT 'running',
                created_at DATETIME NOT NULL,
                updated_at DATETIME NOT NULL,
                expires_at DATETIME NOT NULL
            )`)
			s.CreateIndex("idx_knowledge_vector_audit_tenant_updated", "knowledge_vector_audit_sessions", []string{"tenant_id", "updated_at"}, false)
			return nil
		},
	}
}
