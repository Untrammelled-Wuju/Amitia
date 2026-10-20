package migration

func KnowledgePolicyTraceMigration() Migration {
	return Migration{
		Version: "20261016007",
		Name:    "knowledge_policy_trace",
		Up: func(s *Step) error {
			columns := []struct {
				name     string
				typeDecl string
			}{
				{"policy_mode", "TEXT NOT NULL DEFAULT ''"},
				{"route_reason", "TEXT NOT NULL DEFAULT ''"},
				{"shadow_level", "TEXT NOT NULL DEFAULT ''"},
				{"shadow_reason", "TEXT NOT NULL DEFAULT ''"},
			}
			for _, column := range columns {
				if err := s.AddColumn("retrieval_traces", column.name, column.typeDecl); err != nil {
					return err
				}
			}
			s.CreateIndex("idx_retrieval_traces_tenant_policy_created", "retrieval_traces", []string{"tenant_id", "policy_mode", "created_at"}, false)
			return nil
		},
	}
}
