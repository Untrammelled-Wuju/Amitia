package migration

// KnowledgeOutboxLeaseMigration adds a durable lease to secondary-index work.
// The outbox stays canonical even when multiple backend processes reconcile the
// same tenant: only the worker holding a non-expired lease may complete/retry a
// row. This keeps Qdrant work idempotent without relying on process-local locks.
func KnowledgeOutboxLeaseMigration() Migration {
	return Migration{
		Version: "20260926004",
		Name:    "knowledge_index_outbox_lease",
		Up: func(s *Step) error {
			if err := s.AddColumn("knowledge_index_outbox", "lease_token", "TEXT NOT NULL DEFAULT ''"); err != nil {
				return err
			}
			if err := s.AddColumn("knowledge_index_outbox", "lease_until", "DATETIME"); err != nil {
				return err
			}
			s.CreateIndex("idx_knowledge_outbox_lease", "knowledge_index_outbox", []string{"tenant_id", "status", "next_attempt_at", "lease_until"}, false)
			return nil
		},
	}
}
