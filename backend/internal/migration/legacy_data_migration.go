package migration

func LegacyDataMigration() Migration {
 	return Migration{
 		Version: "202607010003",
		Name:    "create_migration_checkpoints_table",
 		Up: func(step *Step) error {
			step.CreateTable("CREATE TABLE IF NOT EXISTS migration_checkpoints (id TEXT PRIMARY KEY, check_type TEXT NOT NULL DEFAULT '', scope TEXT NOT NULL DEFAULT '', status TEXT NOT NULL DEFAULT 'pending', total_count INTEGER DEFAULT 0, migrated_count INTEGER DEFAULT 0, error_count INTEGER DEFAULT 0, started_at TEXT DEFAULT '', finished_at TEXT DEFAULT '', description TEXT DEFAULT '')")
			return nil
 		},
 	}
 }
