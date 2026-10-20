package migration

func KnowledgeVectorAuditPagesMigration() Migration {
	return Migration{
		Version: "20261016004",
		Name:    "knowledge_vector_audit_pages",
		Up: func(s *Step) error {
			s.CreateTable(`CREATE TABLE IF NOT EXISTS knowledge_vector_audit_pages (
    audit_id TEXT NOT NULL,
    tenant_id TEXT NOT NULL,
    page INTEGER NOT NULL,
    request_hash TEXT NOT NULL,
    result_json TEXT NOT NULL,
    result_hash TEXT NOT NULL,
    previous_hash TEXT NOT NULL,
    receipt_hash TEXT NOT NULL,
    created_at DATETIME NOT NULL,
    PRIMARY KEY (audit_id, page)
   )`)
			s.CreateIndex("idx_knowledge_vector_audit_pages_tenant", "knowledge_vector_audit_pages", []string{"tenant_id", "audit_id", "page"}, false)
			return nil
		},
	}
}
