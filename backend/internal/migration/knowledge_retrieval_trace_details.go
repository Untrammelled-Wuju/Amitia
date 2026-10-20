package migration

// KnowledgeRetrievalTraceDetailsMigration makes retrieval observability useful
// for production debugging without persisting user query/document contents.
// Candidate/evidence JSON contains IDs, ranks, score breakdowns and locator
// hashes only; the raw query remains represented by query_hash.
func KnowledgeRetrievalTraceDetailsMigration() Migration {
	return Migration{
		Version: "20260926006",
		Name:    "knowledge_retrieval_trace_details",
		Up: func(s *Step) error {
			columns := []struct {
				name     string
				typeDecl string
			}{
				{"temporal_mode", "TEXT NOT NULL DEFAULT 'current'"},
				{"as_of", "DATETIME"},
				{"degraded", "INTEGER NOT NULL DEFAULT 0"},
				{"ambiguous", "INTEGER NOT NULL DEFAULT 0"},
				{"warnings_json", "TEXT NOT NULL DEFAULT '[]'"},
				{"candidate_trace_json", "TEXT NOT NULL DEFAULT '[]'"},
				{"evidence_trace_json", "TEXT NOT NULL DEFAULT '[]'"},
			}
			for _, column := range columns {
				if err := s.AddColumn("retrieval_traces", column.name, column.typeDecl); err != nil {
					return err
				}
			}
			s.CreateIndex("idx_retrieval_traces_tenant_mode_created", "retrieval_traces", []string{"tenant_id", "temporal_mode", "created_at"}, false)
			return nil
		},
	}
}
