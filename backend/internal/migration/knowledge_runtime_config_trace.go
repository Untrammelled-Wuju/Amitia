package migration

// KnowledgeRuntimeConfigTraceMigration binds retrieval observations to the
// exact typed runtime configuration snapshot without persisting user content.
// SLO state is derived from the same configured retrieval level budget.
func KnowledgeRuntimeConfigTraceMigration() Migration {
	return Migration{
		Version: "20260926008",
		Name:    "knowledge_runtime_config_trace",
		Up: func(s *Step) error {
			for _, table := range []string{"knowledge_sources", "knowledge_source_revisions"} {
				if err := s.AddColumn(table, "config_hash", "TEXT NOT NULL DEFAULT ''"); err != nil {
					return err
				}
			}
			columns := []struct {
				name     string
				typeDecl string
			}{
				{"config_hash", "TEXT NOT NULL DEFAULT ''"},
				{"slo_target_ms", "INTEGER NOT NULL DEFAULT 0"},
				{"slo_exceeded", "INTEGER NOT NULL DEFAULT 0"},
			}
			for _, column := range columns {
				if err := s.AddColumn("retrieval_traces", column.name, column.typeDecl); err != nil {
					return err
				}
			}
			s.CreateIndex("idx_retrieval_traces_tenant_config_created", "retrieval_traces", []string{"tenant_id", "config_hash", "created_at"}, false)
			s.CreateIndex("idx_retrieval_traces_tenant_slo_created", "retrieval_traces", []string{"tenant_id", "slo_exceeded", "created_at"}, false)
			return nil
		},
	}
}
