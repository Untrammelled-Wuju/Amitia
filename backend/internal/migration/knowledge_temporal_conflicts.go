package migration

// KnowledgeTemporalConflictsMigration adds bitemporal-ish validity metadata and
// explicit supersession/conflict fields to canonical Knowledge units/chunks.
// created_at remains ingestion time; valid_from/valid_to describe when a fact is
// true in the source domain. The fields live in SQLite canonical data and are
// never inferred from Qdrant payloads.
func KnowledgeTemporalConflictsMigration() Migration {
	return Migration{
		Version: "20260926005",
		Name:    "knowledge_temporal_and_conflict_metadata",
		Up: func(s *Step) error {
			for _, table := range []string{"knowledge_units", "knowledge_chunks"} {
				if err := s.AddColumn(table, "valid_from", "DATETIME"); err != nil {
					return err
				}
				if err := s.AddColumn(table, "valid_to", "DATETIME"); err != nil {
					return err
				}
				if err := s.AddColumn(table, "supersedes_json", "TEXT NOT NULL DEFAULT '[]'"); err != nil {
					return err
				}
				if err := s.AddColumn(table, "conflict_group", "TEXT NOT NULL DEFAULT ''"); err != nil {
					return err
				}
			}
			s.CreateIndex("idx_knowledge_units_temporal", "knowledge_units", []string{"tenant_id", "valid_from", "valid_to"}, false)
			s.CreateIndex("idx_knowledge_units_conflict", "knowledge_units", []string{"tenant_id", "conflict_group"}, false)
			s.CreateIndex("idx_knowledge_chunks_temporal", "knowledge_chunks", []string{"tenant_id", "valid_from", "valid_to"}, false)
			s.CreateIndex("idx_knowledge_chunks_conflict", "knowledge_chunks", []string{"tenant_id", "conflict_group"}, false)
			return nil
		},
	}
}
