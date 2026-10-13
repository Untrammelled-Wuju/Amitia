package migration

func RetireLegacyMemoryDataMigration() Migration {
	return Migration{Version: "20261013001", Name: "discard_retired_memory_data", Up: func(s *Step) error {
		for _, table := range []string{"memory_scope_transfers", "memory_derivations", "memory_temporal_metadata", "memory_candidates", "memory_events", "memory_embeddings", "memory_summaries", "retrieval_logs", "user_profiles", "episodic_memories", "memories"} {
			exists, err := s.TableExists(table)
			if err != nil {
				return err
			}
			if exists {
				s.Execute("DELETE FROM " + table)
			}
		}
		for _, item := range []struct{ table, column string }{{"data_lifecycle_outbox_cleanup_items", "target_kind"}, {"deletion_tombstones", "target_type"}} {
			exists, err := s.TableExists(item.table)
			if err != nil {
				return err
			}
			if exists {
				s.Execute("DELETE FROM " + item.table + " WHERE " + item.column + "='memory'")
			}
		}
		return nil
	}}
}
