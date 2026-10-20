package migration

// KnowledgeQueryPlanTraceMigration persists only the safe retrieval plan shape
// (level/complexity/query hashes), never raw original/contextual/subquery text.
func KnowledgeQueryPlanTraceMigration() Migration {
	return Migration{
		Version: "20260926007",
		Name:    "knowledge_query_plan_trace",
		Up: func(s *Step) error {
			return s.AddColumn("retrieval_traces", "query_plan_json", "TEXT NOT NULL DEFAULT '{}'")
		},
	}
}
