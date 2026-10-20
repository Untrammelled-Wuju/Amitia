package migration

// KnowledgeSchemaContractRepairMigration normalizes index definitions that
// changed after the first RAG cutover. SQLite's CREATE INDEX IF NOT EXISTS does
// not replace an existing index with the same name, so old installations can
// otherwise keep the pre-revision three-column outbox index forever while a
// fresh baseline gets the four-column final definition.
func KnowledgeSchemaContractRepairMigration() Migration {
	return Migration{
		Version: "20260926009",
		Name:    "knowledge_schema_contract_index_repair",
		Up: func(s *Step) error {
			s.Execute(`DROP INDEX IF EXISTS idx_knowledge_outbox_revision`)
			s.Execute(`DROP INDEX IF EXISTS idx_knowledge_outbox_aggregate`)
			s.Execute(`CREATE INDEX IF NOT EXISTS idx_knowledge_outbox_aggregate
				ON knowledge_index_outbox(tenant_id, aggregate_type, aggregate_id, source_revision)`)
			return nil
		},
	}
}
