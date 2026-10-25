package migration

func RemoveGlobalMemoryStorageMigration() Migration {
	return Migration{Version: "20261025001", Name: "remove_global_memory_storage", Up: func(s *Step) error {
		exists, err := s.TableExists("memory_scope_transfers")
		if err != nil {
			return err
		}
		if exists {
			s.Execute("DROP TABLE memory_scope_transfers")
		}
		exists, err = s.ColumnExists("characters", "global_memory_enabled")
		if err != nil {
			return err
		}
		if exists {
			s.Execute("ALTER TABLE characters DROP COLUMN global_memory_enabled")
		}
		return nil
	}}
}
