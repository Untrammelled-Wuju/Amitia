package migration

// KnowledgeBenchmarkRunsMigration persists aggregate-only benchmark history.
// It intentionally excludes queries, evidence content and locators so benchmark
// trend storage cannot become a second copy of user knowledge.
func KnowledgeBenchmarkRunsMigration() Migration {
	return Migration{
		Version: "20260927002",
		Name:    "knowledge_benchmark_runs",
		Up: func(s *Step) error {
			s.CreateTable(`CREATE TABLE IF NOT EXISTS knowledge_benchmark_runs (
                id TEXT PRIMARY KEY,
                tenant_id TEXT NOT NULL,
                config_hash TEXT NOT NULL DEFAULT '',
                k INTEGER NOT NULL DEFAULT 0,
                cases INTEGER NOT NULL DEFAULT 0,
                successful_cases INTEGER NOT NULL DEFAULT 0,
                error_cases INTEGER NOT NULL DEFAULT 0,
                failed_cases INTEGER NOT NULL DEFAULT 0,
                positive_cases INTEGER NOT NULL DEFAULT 0,
                no_answer_cases INTEGER NOT NULL DEFAULT 0,
                citation_observed_cases INTEGER NOT NULL DEFAULT 0,
                scenario_counts_json TEXT NOT NULL DEFAULT '{}',
                recall_at_k REAL NOT NULL DEFAULT 0,
                mrr REAL NOT NULL DEFAULT 0,
                ndcg_at_k REAL NOT NULL DEFAULT 0,
                context_precision REAL NOT NULL DEFAULT 0,
                context_recall REAL NOT NULL DEFAULT 0,
                no_answer_accuracy REAL NOT NULL DEFAULT 0,
                citation_precision REAL NOT NULL DEFAULT 0,
                citation_recall REAL NOT NULL DEFAULT 0,
                groundedness REAL NOT NULL DEFAULT 0,
                error_rate REAL NOT NULL DEFAULT 0,
                p50_latency_ms INTEGER NOT NULL DEFAULT 0,
                p95_latency_ms INTEGER NOT NULL DEFAULT 0,
                passed INTEGER NOT NULL DEFAULT 0,
                violations_json TEXT NOT NULL DEFAULT '[]',
                created_at DATETIME NOT NULL
            )`)
			s.CreateIndex("idx_knowledge_benchmark_runs_tenant_created", "knowledge_benchmark_runs", []string{"tenant_id", "created_at"}, false)
			s.CreateIndex("idx_knowledge_benchmark_runs_tenant_config_created", "knowledge_benchmark_runs", []string{"tenant_id", "config_hash", "created_at"}, false)
			return nil
		},
	}
}
