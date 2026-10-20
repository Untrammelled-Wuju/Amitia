package migration

func KnowledgeVectorRecoveryReviewsMigration() Migration {
	return Migration{
		Version: "20261016006",
		Name:    "knowledge_vector_recovery_reviews",
		Up: func(s *Step) error {
			s.CreateTable(`CREATE TABLE IF NOT EXISTS knowledge_vector_recovery_reviews (
    id TEXT PRIMARY KEY,
    tenant_id TEXT NOT NULL,
    audit_id TEXT NOT NULL,
    recheck_id TEXT NOT NULL,
    receipt_hash TEXT NOT NULL,
    request_key TEXT NOT NULL,
    requested_by_hash TEXT NOT NULL,
    reviewed_by_hash TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL CHECK (status IN ('pending', 'approved', 'revoked')),
    action TEXT NOT NULL CHECK (action = 'manual_investigation'),
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL,
    expires_at DATETIME NOT NULL,
    UNIQUE (tenant_id, request_key),
    UNIQUE (tenant_id, audit_id, recheck_id)
   )`)
			s.CreateIndex("idx_knowledge_vector_recovery_reviews_scope", "knowledge_vector_recovery_reviews", []string{"tenant_id", "audit_id", "created_at"}, false)
			s.CreateTable(`CREATE TABLE IF NOT EXISTS knowledge_vector_recovery_review_events (
                id TEXT PRIMARY KEY,
                tenant_id TEXT NOT NULL,
                review_id TEXT NOT NULL,
                action TEXT NOT NULL CHECK (action IN ('requested', 'approved', 'revoked')),
                actor_hash TEXT NOT NULL,
                receipt_hash TEXT NOT NULL,
                created_at DATETIME NOT NULL
            )`)
			s.CreateIndex("idx_knowledge_vector_recovery_review_events_scope", "knowledge_vector_recovery_review_events", []string{"tenant_id", "review_id", "created_at"}, false)
			return nil
		},
	}
}
