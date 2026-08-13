package migration

func MemorySummaryConsolidationMigration() Migration {
	return Migration{
		Version: "20260814001",
		Name:    "memory_summary_consolidation",
		Up: func(s *Step) error {
			s.CreateTable(`CREATE TABLE IF NOT EXISTS memory_derivations (
				id TEXT PRIMARY KEY,
				output_memory_id TEXT NOT NULL,
				input_memory_id TEXT NOT NULL,
				input_version INTEGER NOT NULL,
				input_snapshot_hash TEXT NOT NULL DEFAULT '',
				derivation_kind TEXT NOT NULL,
				ordinal INTEGER NOT NULL DEFAULT 0,
				operation_id TEXT NOT NULL DEFAULT '',
				created_at TEXT NOT NULL,
				UNIQUE(output_memory_id, input_memory_id, input_version, derivation_kind)
			)`)

			s.AddColumn("memories", "version", "INTEGER NOT NULL DEFAULT 1")
			s.AddColumn("memories", "derivation_key", "TEXT NOT NULL DEFAULT ''")

			s.AddColumn("memory_events", "version", "INTEGER NOT NULL DEFAULT 1")
			s.AddColumn("memory_events", "operation_id", "TEXT NOT NULL DEFAULT ''")
			s.AddColumn("memory_events", "snapshot_hash", "TEXT NOT NULL DEFAULT ''")
			s.AddColumn("memory_events", "event_reason", "TEXT NOT NULL DEFAULT ''")

			s.AddColumn("memory_candidates", "candidate_kind", "TEXT NOT NULL DEFAULT 'extracted'")
			s.AddColumn("memory_candidates", "confidence", "REAL NOT NULL DEFAULT 0")
			s.AddColumn("memory_candidates", "target_memory_id", "TEXT NOT NULL DEFAULT ''")
			s.AddColumn("memory_candidates", "proposed_action", "TEXT NOT NULL DEFAULT ''")
			s.AddColumn("memory_candidates", "source_memory_ids_json", "TEXT NOT NULL DEFAULT ''")
			s.AddColumn("memory_candidates", "source_versions_json", "TEXT NOT NULL DEFAULT ''")
			s.AddColumn("memory_candidates", "derivation_key", "TEXT NOT NULL DEFAULT ''")
			s.AddColumn("memory_candidates", "reason", "TEXT NOT NULL DEFAULT ''")

			s.CreateIndex("idx_memory_derivations_output", "memory_derivations", []string{"output_memory_id"}, false)
			s.CreateIndex("idx_memory_derivations_input", "memory_derivations", []string{"input_memory_id"}, false)
			s.CreateIndex("idx_memories_derivation_key", "memories", []string{"derivation_key"}, false)
			s.CreateIndex("idx_memory_events_operation", "memory_events", []string{"operation_id"}, false)

			return nil
		},
	}
}
