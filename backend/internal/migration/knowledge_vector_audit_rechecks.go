package migration

func KnowledgeVectorAuditRechecksMigration() Migration {
	return Migration{
		Version: "20261016005",
		Name:    "knowledge_vector_audit_rechecks",
		Up: func(s *Step) error {
			s.CreateTable(`CREATE TABLE IF NOT EXISTS knowledge_vector_audit_recheck_leases (
    audit_id TEXT PRIMARY KEY,
    tenant_id TEXT NOT NULL,
    lease_token TEXT NOT NULL,
    lease_expires_at DATETIME NOT NULL,
    next_allowed_at DATETIME NOT NULL
   )`)
			s.CreateIndex("idx_knowledge_vector_audit_recheck_lease_tenant", "knowledge_vector_audit_recheck_leases", []string{"tenant_id", "next_allowed_at"}, false)
			s.CreateTable(`CREATE TABLE IF NOT EXISTS knowledge_vector_audit_recheck_results (
    id TEXT PRIMARY KEY,
    tenant_id TEXT NOT NULL,
    audit_id TEXT NOT NULL,
    page INTEGER NOT NULL,
    status TEXT NOT NULL,
    result_hash TEXT NOT NULL,
    receipt_hash TEXT NOT NULL,
    observed_at DATETIME NOT NULL,
    created_at DATETIME NOT NULL
   )`)
			s.CreateIndex("idx_knowledge_vector_audit_recheck_result_history", "knowledge_vector_audit_recheck_results", []string{"tenant_id", "audit_id", "created_at"}, false)
			return nil
		},
	}
}
